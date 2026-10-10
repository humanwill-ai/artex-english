package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/intercept"
	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/llm"
	"github.com/Autumn-27/norma/permission"
	actool "github.com/Autumn-27/norma/tool"
	"github.com/Autumn-27/norma/transcript"
)

// Planner is the event-driven LLM planner (docs §4.3): each time the asset or
// exploration graph changes (debounced), it reads the exploration route, queries
// assets, judges whether the task goal is met, and emits 0..N exploration intents
// into the frontier. It is the sole intent generator.
type Planner struct {
	findingRecorder   FindingRecorder
	prov              llm.Provider
	model             string
	tx                *transcript.Store                      // raw LLM conversation persistence (nil = off)
	window            int                                    // context window in tokens (for compaction)
	windowFn          func() int                             // optional dynamic task-chain minimum
	maxTurns          int                                    // max agent turns per run (0 = unlimited)
	killWork          func(intentID int64) error             // engine callback to terminate a running work (nil = off)
	steerWork         func(intentID int64, msg string) error // engine callback to steer a running work mid-run (nil = off)
	proxyAddr         string                                 // recording proxy for WebFetch (empty = direct)
	proxyCACert       string                                 // recording proxy's CA cert path (HTTPS verify)
	webSearch         WebSearchOpts                          // web_search tool backend selection (off by default)
	workDir           string                                 // shared work dir (surfaced in prompt as artifact-output target)
	injectConstraints func() bool                            // resolver: inject task operation constraints into system prompt? (nil = yes)
	nonStreamingFn    func() bool                            // resolver: use non-streaming (Complete) path? (nil = streaming)
	noaEnabledFn      func() bool                            // resolver: use experimental noa compaction? (nil = off)
	maxTokensFn       func() int                             // resolver: per-reply output cap (nil/0 = send no cap)
	compactor         *Compactor                             // cold-node compaction (§7); nil = disabled

	// todos keeps ONE plan-scratchpad per task (keyed by exploration id) so the
	// planner's multi-step plan survives across wake-ups — each Plan() is a fresh
	// session, but the shared store lets it record a serial exploit chain once and
	// dispatch it step-by-step over rounds instead of front-loading it in parallel.
	todoMu sync.Mutex
	todos  map[int64]*actool.TodoStore
}

var ErrAwaitCondition = errors.New("task awaiting external condition")

func NewPlanner(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int) *Planner {
	return &Planner{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns, todos: map[int64]*actool.TodoStore{}}
}

func (p *Planner) SetCompactionWindowResolver(fn func() int) { p.windowFn = fn }

// SetCompactor wires the cold-node compactor (cold-digest §7). Called each
// planner wake-up to advance the round counter, maintain cold stamps, and
// (off the hot path) fold cold nodes into digests. nil = feature disabled.
func (p *Planner) SetCompactor(c *Compactor) { p.compactor = c }

// SetNonStreaming wires a resolver deciding whether runs use the non-streaming
// model path (true = non-streaming). nil/unset = streaming (default).
func (p *Planner) SetNonStreaming(fn func() bool) { p.nonStreamingFn = fn }

func (p *Planner) nonStreaming() bool { return p.nonStreamingFn != nil && p.nonStreamingFn() }

// SetNoaEnabled wires a resolver deciding whether runs use the experimental noa
// context-compression mechanism. nil/unset = off (built-in compaction). Read per
// run so the settings toggle takes effect without rebuilding the agent.
func (p *Planner) SetNoaEnabled(fn func() bool) { p.noaEnabledFn = fn }

// SetMaxTokens wires a resolver for the per-reply output cap. nil/unset or 0 =
// send no cap and let the endpoint decide. Read per run, like nonStreaming.
func (p *Planner) SetMaxTokens(fn func() int) { p.maxTokensFn = fn }

func (p *Planner) maxTokens() int {
	if p.maxTokensFn == nil {
		return 0
	}
	return p.maxTokensFn()
}

func (p *Planner) compactionWindow() int {
	if p.windowFn != nil {
		return p.windowFn()
	}
	return p.window
}

// SetProxy points the planner's WebFetch at the recording proxy plus the CA cert
// it trusts to verify HTTPS through it (empty addr = direct).
func (p *Planner) SetProxy(addr, caCert string) { p.proxyAddr, p.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the planner (off by default).
func (p *Planner) SetWebSearch(o WebSearchOpts) { p.webSearch = o }

// SetConstraintInject wires a resolver deciding whether this task's operation
// constraints get injected into the planner system prompt. Read per round so the
// settings toggle takes effect without rebuilding the agent. nil = inject (default).
func (p *Planner) SetConstraintInject(fn func() bool) { p.injectConstraints = fn }

// wantConstraints reports whether constraint injection is enabled (default yes).
func (p *Planner) wantConstraints() bool { return p.injectConstraints == nil || p.injectConstraints() }

// todoFor returns the task's persistent planning todo store, creating it on first
// use. Shared across all of this task's planner wake-ups.
func (p *Planner) todoFor(expID int64) *actool.TodoStore {
	p.todoMu.Lock()
	defer p.todoMu.Unlock()
	s := p.todos[expID]
	if s == nil {
		s = actool.NewTodoStore()
		p.todos[expID] = s
	}
	return s
}

// SetKillWork wires the engine's per-work terminate callback so the planner's
// kill_work tool can stop a single running worker.
func (p *Planner) SetKillWork(fn func(intentID int64) error) { p.killWork = fn }

// SetSteerWork wires the engine's per-work steering callback so the planner's
// steer_work tool can inject a mid-run course-correction into a running worker.
func (p *Planner) SetSteerWork(fn func(intentID int64, msg string) error) { p.steerWork = fn }

// renderPlannerTodos formats the persistent planning todo for injection into the
// wake-up prompt (empty when there are no todos yet — first wake-up).
func renderPlannerTodos(items []actool.Todo) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Your planning to-do list (retained between wake-ups; written in the previous round)]:\n")
	for _, it := range items {
		mark := map[actool.TodoStatus]string{actool.TodoPending: "☐", actool.TodoInProgress: "▶", actool.TodoCompleted: "✔"}[it.Status]
		if mark == "" {
			mark = "☐"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", mark, it.Content))
	}
	b.WriteString("Proceed from this list: dispatch an intent only for a next step whose prerequisites are complete or whose required facts already exist. Update the list with TodoWrite, marking steps satisfied by facts as completed. Do not redispatch steps already marked pending/in_progress.")
	return b.String()
}

// TriggerEvent describes what concretely caused this planning round to fire, so
// the planner looks first at the actual change instead of re-scanning the whole
// overview. Kind:
//
//	"done"    — a worker finished intent IntentID (its output conclusion is fetched).
//	"finding" — a worker reported a finding on intent IntentID (Detail = 摘要).
//	"goal"    — the human (via 主 agent 的 set_goals) added one OR MORE goals in a
//	            single call (Goals = 本次新增的目标文本，1+ 条；set_goals 支持批量).
//	"goal_deleted" — the human deleted a goal from 总览的目标管理 (Detail = 被删目标文本).
//	"goal_edited"  — the human edited a goal from 总览的目标管理 (OldGoal→NewGoal 文本).
//	"cancelled" — the human deleted intent IntentID (Detail = 删除原因). The intent is
//	            stopped (not deleted) and the reason is attached to it as a fact.
type TriggerEvent struct {
	Kind     string
	IntentID int64
	Detail   string
	Summary  string   // Kind=="cancelled" 专用：删除前捕获的意图摘要（真删除后节点已不存在，无法再查）
	Goals    []string // Kind=="goal" 专用：本次 set_goals 新增的目标文本（1 条或多条）
	OldGoal  string   // Kind=="goal_edited" 专用：修改前的目标文本
	NewGoal  string   // Kind=="goal_edited" 专用：修改后的目标文本
	Hints    []string // Kind=="hint" 专用：本次 add_hint 新增的提示文本（1 条或多条）
}

// renderTriggers spells out the change(s) that fired this round: for a finished
// worker — which intent + its output conclusion; for a finding — which intent +
// what was found. Empty for time/heartbeat wakes. Reads the store (best-effort;
// a blank field never blocks the round).
func renderTriggers(ts *db.ExplorationStore, evs []TriggerEvent) string {
	if len(evs) == 0 || ts == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Actual changes that triggered this round (read these first, then decide whether to add directions)]:")
	for _, ev := range evs {
		switch ev.Kind {
		case "goal":
			if len(ev.Goals) == 1 {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added a goal: %s — a new objective to achieve. Add exploration directions for it if no corresponding intent exists.", ev.Goals[0]))
			} else {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added %d goals: %s — these are new objectives. Add exploration directions for each goal without a corresponding intent.", len(ev.Goals), strings.Join(ev.Goals, "；")))
			}
		case "hint":
			if len(ev.Hints) == 1 {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added a strategic hint: %s — it is attached to the exploration graph. Adjust or add exploration directions accordingly if no corresponding intent exists.", ev.Hints[0]))
			} else {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added %d strategic hints: %s — all are attached to the exploration graph. Adjust or add exploration directions based on each.", len(ev.Hints), strings.Join(ev.Hints, "；")))
			}
		case "goal_deleted":
			b.WriteString(fmt.Sprintf("\n- The human deleted this goal: %s — it has been removed. Reassess the remaining goals/directions; do not dispatch further intents for the deleted goal.", ev.Detail))
		case "goal_edited":
			b.WriteString(fmt.Sprintf("\n- The human changed the goal from \"%s\" to \"%s\" — adjust exploration directions to the new goal; stop dispatching the original direction if it no longer applies.", ev.OldGoal, ev.NewGoal))
		case "finding":
			b.WriteString(fmt.Sprintf("\n- The worker for intent #%d (%s) reported a finding: %s", ev.IntentID, intentSummary(ts, ev.IntentID), ev.Detail))
		case "cancelled":
			// 意图内容优先用删除时捕获的 Summary（真删除后节点已不存在，intentSummary 查不到）。
			sm := ev.Summary
			if sm == "" {
				sm = intentSummary(ts, ev.IntentID)
			}
			b.WriteString(fmt.Sprintf("\n- The user deleted intent #%d. Intent: %s; deletion reason: %s. It has been deleted and will no longer execute. Replan accordingly.", ev.IntentID, sm, ev.Detail))
		default: // "done"
			b.WriteString(fmt.Sprintf("\n- The worker for intent #%d (%s) finished with this conclusion: %s", ev.IntentID, intentSummary(ts, ev.IntentID), workerOutput(ts, ev.IntentID)))
			if fids := factIDsYielded(ts, ev.IntentID); fids != "" {
				b.WriteString(fmt.Sprintf("; IDs of new facts produced by this intent: %s ", fids))
			}
		}
	}
	b.WriteString("\n(Use node_detail / get_worker_output / list_findings for complete details.)")
	return b.String()
}

// factIDsYielded lists the fact ids an intent produced this run as "#12、#15", so the
// planner can jump straight to the round's incremental facts. Empty (best-effort) when
// the intent yielded no facts or the lookup fails.
func factIDsYielded(ts *db.ExplorationStore, id int64) string {
	ids, err := ts.FactsYielded(id)
	if err != nil || len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, fid := range ids {
		parts[i] = fmt.Sprintf("#%d", fid)
	}
	return strings.Join(parts, "、")
}

// intentSummary reads an intent node's one-line summary (best-effort, "?" on miss).
func intentSummary(ts *db.ExplorationStore, id int64) string {
	n, err := ts.GetNode(id)
	if err != nil || n == nil {
		return "?"
	}
	var p map[string]any
	if json.Unmarshal(n.Payload, &p) == nil {
		if s, ok := p["summary"].(string); ok && s != "" {
			return s
		}
	}
	return "?"
}

// workerOutput returns the finished worker's conclusion for an intent — the last
// 'result' (else 'text') activity's full detail, truncated. Same source get_worker_output uses.
func workerOutput(ts *db.ExplorationStore, id int64) string {
	acts, _, err := ts.ActivityList(&id, 0, 1000)
	if err != nil {
		return "(Failed to retrieve output)"
	}
	var pick *db.Activity
	for i := range acts {
		if acts[i].Kind == "result" {
			pick = &acts[i]
		} else if acts[i].Kind == "text" && pick == nil {
			pick = &acts[i]
		}
	}
	if pick == nil {
		return "(No output has been recorded for this work yet)"
	}
	out, _ := ts.ActivityDetail(pick.ID)
	if out == "" {
		out = pick.Summary
	}
	return truncOutput(out, 800)
}

// truncOutput caps a worker-output blob so the trigger context doesn't bloat the
// system prompt every round; full text is one get_worker_output call away.
func truncOutput(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + " … (truncated; see get_worker_output for the complete output)"
}

// renderGraphOverview folds the pre-computed graph_overview snapshot into the
// wake-up prompt so the planner starts each round with the full situation in
// hand — saving the round-trip it would otherwise spend calling the tool. It is
// the exact same JSON graph_overview would return; deeper detail is still one
// tool call away (node_detail / list_facts / …).
func renderGraphOverview(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return "" // fall back to the model calling graph_overview itself
	}
	return "\n\n[Situation for this round (prefetched graph_overview, equivalent to calling that tool; request node_detail/list_facts, etc. only when more detail is needed)]:\n" + string(b)
}

// plannerDefaultTmpl is the built-in EDITABLE body (段 [A]) of the planner prompt,
// seeded into agent_prompts. Goal is a {{.Goal}} template var; the 中间产物输出规约
// tail is code-owned (artifactSpec) and appended by plannerSystem after rendering.
const plannerDefaultTmpl = `You are the "planner" in a cybersecurity platform's authorized penetration-testing system, awakened frequently whenever the graph changes. Your job is to read the situation → assess goals → add exploration intents **only when there are genuinely new, uncovered directions**. You are the planner, not the executor: this round may produce only **generated/clarified intents** or **goal assessments**. Do not perform the actual work during planning.

Task objective: {{.Goal}}

**How many intents to produce this round (consider this first)**:
- **Hard minimum**: whenever the objective is unmet and there are no open or running intents, produce at least one genuinely new intent advancing it. If two distinct confirmed observations show the same external condition remains unavailable after a bounded recheck, and no useful in-scope direction remains, call await_condition with both fact IDs and the concrete resume condition instead. Keep the goal open. Do not repeat the same request indefinitely or use a negative proof for unavailable evidence.
- Apart from that hard minimum, **zero intents is a valid result only with a good reason**, not a default belief that fewer is safer: ① **Already covered** — all directions you considered are being handled by open/running intents; generating reworded duplicates is a serious error. ② **Waiting for a dependency** — the next step depends on output from currently running work that is not yet available; forced dispatch would leave the downstream work without its prerequisite. Wait for the next wake-up after the graph updates.
- Conversely, dispatch when there is a genuinely **uncovered direction that does not depend on running work**, or when the objective is unmet and an in-scope surface remains untested. Do not use zero intents as an excuse for inaction.

**Decision process on each wake-up**:

1. **The complete situation is attached below** (the graph_overview result, so do not call it again): task (original title plus objective/root node), asset counts, goals and their states, open/running/recent_done intents, sites_without_endpoints (sites without endpoints, suggesting potentially unexplored directions), facts (exploration-fact count, distinct from vulnerabilities), and recent_facts ({id,summary,confidence?}).
   - **Scope**: exploration nodes (goals/intents/facts/findings) belong only to this task. **The asset graph is shared globally** across tasks; asset counts are global within scope, not unique to this task. Ignore assets unrelated to this task.
   - **Lineage**: each intent has parents (upstream facts/intents it derives from) and yields (downstream facts/findings it produced); each recent_facts entry has from_intent. Use these to understand which direction produced a fact and whether facts together suggest a new direction.
   - **Negative/uncertain observations** (such as "port closed/not injectable" in recent_facts) are worker observations, not final judgments. Before accepting them, inspect evidence with node_detail(id). Treat a direction as provisionally blocked only when the evidence is solid, confidence=observed, and the approaches have been exhausted. If evidence is absent, based on a single attempt or appearance, or confidence=inferred, treat the direction as **unresolved**. If it is in scope and not covered by another intent, normally dispatch a verification intent to confirm or refute it. **Recheck the same negative direction at most once**; if the recheck remains negative with reasonable evidence, respect that conclusion and stop dispatching it.
   - **Retrieve details only when needed**: list_facts (paginated, newest first, default 20, q filtering, before pagination, total/has_more), list_findings (all findings), node_detail(id) (full evidence/details; lists/recent_facts contain summaries only), list_assets (pull: q search, type/company_id/task_id filters, pagination, or direct id/ids lookup), asset_neighbors. Assets are globally shared; do not fetch the entire asset set by default.

2. **Assess goals (your core responsibility)**: goals already contains each goal and state. For an unmet goal proved by a finding/fact, call prove_goal(goal_id, evidence_id, reason) to mark it met. **Marking the last unmet goal automatically completes the entire task.** Completion is driven only by individual prove_goal calls; there is no other one-click completion method.
   - ⚠️ **Check quantitative acceptance criteria; never sign off early.** If a goal includes a measurable condition (X% coverage, N flags, a particular privilege), you **must** check actual graph_overview values (coverage.pct, findings_total, etc.) before prove_goal. If the threshold is unmet, **do not** call prove_goal; dispatch intents to close the gap. Do not mark met because the goal is "mostly achieved" or the "core part is done." Example: required coverage 100%, measured coverage.pct=40% → unmet; dispatch further testing intents.

3. **Optional, opening phase only, minimal probing for understanding**: only when the graph has almost no facts (recent_facts essentially empty and the task just started), and the situation alone cannot support a concrete initial intent, use Bash or similar tools for a very small amount of read-only probing (for example, 1–2 curl requests for the homepage/fingerprint). **The only valid output is a more precise intent description**, never vulnerability discovery/verification/exploitation or endpoint/directory/parameter enumeration results; those belong to a worker and must be dispatched as an intent. Three hard boundaries:
   - If worker-produced facts already exist (facts>0 / recent_facts nonempty), **do not probe yourself**. Base every judgment on existing facts. Produce only new intents or end the round. Dispatch a worker to investigate a lead; do not curl it yourself.
   - Even at the start, stop after at most ≤3 probes, solely to clarify initial intents. If you find yourself investigating deeply rather than quickly choosing a direction (enumerating endpoints/directories, trying IDs one by one, decoding chains, repeatedly probing the same interface, or testing any injection/access-control bypass/vulnerability), stop immediately and formulate an intent; these are worker responsibilities.
   - If existing facts/the situation suffice, do not probe at all.

4. **Choose new directions**: **restraint here means avoiding duplicate intents, not minimizing dispatches**. While the objective is unmet, ask what deeper, more forceful, uncovered approach could advance it, not whether to wrap up. Intents are **open exploration directions**, not fixed types or a menu. Derive them from known facts, assets, and goals; compare each with open + running + recent_done:
   - Covered by open/running → do not generate it again; work is in progress.
   - Appears in recent_done → **first inspect its state to understand why it stopped**:
     · **done (completed normally)**: already covered; do not redispatch unchanged. Whether it is a dead end depends on facts in its yields, not its state. Redispatch only for a **materially new mechanism** (new fact/asset/parameter/substantially different approach), and explain the difference in summary. Rewording or hoping another try will work does not qualify; do not retry.
     · **exhausted (budget ran out mid-exploration, only partial results recorded) / blocked (model or network failure, little exploration completed)**: both indicate incomplete work and information. First inspect get_worker_trace / get_worker_output to learn actual progress and the obstacle. If near a breakthrough when the budget ran out, dispatch a continuation from that progress; if only an external failure prevented execution (often blocked), redispatch the same direction; if it repeatedly stalls at the same point, change the approach/direction. Base the decision on real trace progress, not state alone.
   - Entirely new direction not covered by any intent → generate it.
   - All known directions are covered by open/running intents → generate none and end the round; wait for the running/queued work. If only recent_done covers them and no open/running intent remains while the objective is unmet, the hard minimum above requires a new or resumed direction.
   - **Depth before coverage**: coverage is a minimum/acceptance criterion, not the exploration objective itself. After identifying a valuable entry point (potential RCE/privilege escalation/data disclosure), prioritize intents that pursue it **in depth**, instead of shallow tests across assets merely to equalize coverage.
   - **Maintain diverse routes; do not converge too early**: while the objective is unmet, if current intents all concentrate on one route/entry point and a **fundamentally different** uncovered direction exists (another entry surface/asset class/exploitation chain), prioritize that alternative over synonymous intents on the same route. Compare substance, not wording; still do not duplicate an already covered alternative. Ideally maintain 2–3 routes with different mechanisms (such as an upload chain and authentication bypass), concentrating resources only after a route demonstrates progress toward the objective. **Diversity always remains subordinate to the Operational constraints above**: never generate an intent for an excluded entry surface/port/host/operation, however different it may be.

   **Serial exploitation chains: dispatch step by step, not in parallel.** For a strongly dependent chain (①→②→③, each step requiring the previous step's actual output), do not dispatch all steps at once; downstream work would lack its prerequisites and duplicate effort or stall. Record the full chain with TodoWrite (one item per step). This round, dispatch only the step whose prerequisites are satisfied (usually the first). After it produces a fact, dispatch the next step on the next wake-up (the to-do list will be included), marking satisfied steps completed. Do not split the same action into two intents ("confirm the trigger point" and "trigger the trigger point" are one step). Use parallel intents only for independent dimensions, such as enumerating unrelated endpoints.

5. **Submit**: submit the selected directions in **one** add_intent batch (intents array, at most the 4 highest-value directions, not repeated individual calls):
   - **summary**: one natural-language sentence describing the direction (full target address + what to do + why), without a fixed classification. Deduplication mainly compares this with existing intent summaries.
   - **asset_ids**: IDs of target assets to test/attack (supply whenever possible: zero/one/multiple, from list_assets). Always include them when the direction concerns a specific site/interface/parameter/host, to support coverage deduplication and asset linkage. Include every involved asset for cross-asset work; leave empty only for purely global reconnaissance without a specific asset.
   - **parent_ids**: upstream nodes from which the direction was derived (optional: zero/one/multiple). Include all facts combined into the intent, and IDs of upstream intents/findings it derives from. Leave empty for a completely new top-level direction.

Do not duplicate or force unnecessary work; do dispatch when the objective is unmet and a deeper, uncovered approach exists. Be concise, focused, and efficient.`

func plannerSystem(goal, dataDir, workDir string) string {
	body := renderSystem("planner", plannerDefaultTmpl, PlannerVars{Goal: goal, DataDir: dataDir, Now: nowStr()})
	return body + artifactSpec(workDir) + langDirective()
}

// Plan runs one planning round. emit, if non-nil, receives the planner's execution
// steps (so users can see how it reads the situation and judges goals — the
// planner is the intent generator and was previously a black box). Returns whether
// the planner judged the goal met.
// triggers carries the concrete change(s) that fired this round — worker(s) done
// and/or finding(s) reported (may be several — the engine debounces a burst; empty
// for time/heartbeat wakes). They are spelled out at the top of the prompt so the
// planner looks first at the actual change (which intent, its output/finding).
func (p *Planner) Plan(ctx context.Context, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, goal string, triggers []TriggerEvent, emit func(db.Activity)) (met bool, reason string, err error) {
	// cold-digest §2.3/§7: advance this task's planner-round counter, maintain the
	// cold_since_round stamps, and (if a threshold is hit) kick off background
	// compaction. Synchronous part is cheap (a few queries); the LLM compaction
	// runs in a detached goroutine so it never adds latency to this round.
	p.compactor.OnPlannerRound(ctx, ts)
	tsx := NewToolSet(ts, "planner")
	tsx.SetFindingRecorder(p.findingRecorder)
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetTaskID(taskID)
	tsx.SetCoverageEnabled(as == nil || as.CoverageEnabled(taskID))
	tsx.killWork = p.killWork   // enable kill_work tool (nil = unavailable)
	tsx.steerWork = p.steerWork // enable steer_work tool (nil = unavailable)
	if origin, _ := ts.OriginFactID(); origin > 0 {
		tsx.SetOwnerNode(origin) // planner-side anchors default to the task root (origin fact)
	}
	// 领域工具 + 基础默认工具集（Read/Write/Edit/MultiEdit/LS/Glob/Grep/Bash）
	// 资产覆盖度功能关闭时剔除 add_task_scope/list_untested_assets（不入 prompt）。
	base := append(tsx.DropCoverageTools(tsx.PlannerTools()), actool.DefaultTools()...)
	ctx = WithRunInfo(ctx, RunInfo{TaskID: taskID, ExplorationID: explorationID(ts)})
	tools, def, cleanup := AugmentTools(ctx, "planner", base)
	defer cleanup()
	// 关键态势（刚完成的意图 + 预取的完整图）改放【本轮 user 输入】(见下方 input)，system
	// 只留静态规划正文。move-out 让 system 每轮稳定、更利于缓存；代价是若单轮变长，态势可能
	// 被 compaction 压缩（planner 单轮通常短，风险低）。situational 会拼进下方 input。
	situational := renderTriggers(ts, triggers) + renderGraphOverview(tsx.graphOverviewData())
	// 任务级 deadline / 终局模式(经 ctx 注入,见 taskclock.go)。终局那一轮把任务超时
	// planner 收尾词作为【本轮操作指令】拼进本轮 user 输入(随 situational),让它只做最后
	// 目标判定、不产新意图。
	tc := taskClockFrom(ctx)
	if tc.Final {
		situational += "\n\n[Final task settlement (special instructions for this round, overriding the regular planning process above)]:\n" + resolveTaskTimeoutWrapup("planner")
	}
	// 本任务的工作目录 <workDir>/tasks/<taskID>，先建好。
	taskDir := ensureRunDir(p.workDir, taskID, 0)
	ctx = intercept.WithReviewContext(ctx, taskDir, intercept.ReviewBackground{})
	sysBody := plannerSystem(goal, p.workDir, taskDir)
	if p.wantConstraints() {
		sysBody += constraintBlock(ts) // 操作约束(若有)注入系统提示,框定探索边界
	}
	system, boundary := deferredSystem(sysBody, def)
	// planner 无自身墙钟预算;有 deadline 时把 MaxDuration 夹逼到剩余,让在跑的规划轮在
	// 任务到点时进收尾(因超时→任务超时词,因步数→per-run 词)。
	maxDur, clamped := clampMaxDuration(tc.DeadlineUnix, 0)
	settle := wrapupSettlement("planner", nil)
	if tc.DeadlineUnix > 0 {
		settle = wrapupSettlementForTask("planner", nil, clamped)
	}
	opts := agentcore.Options{
		Provider:        p.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // 走记录代理留痕；载入代理 CA 验证 MITM 重签的 HTTPS 证书
		WebFetchProxy:   p.proxyAddr,
		WebFetchCACert:  p.proxyCACert,
		// 联网搜索(可选)。ddgs 无需 key；brave-free 需 BraveKey；tavily 需 TavilyKey。
		// WebSearchProxy 是独立出口代理(http/https/socks5)，与记录流量的 MITM 代理无关；空则直连。
		EnableWebSearch:       p.webSearch.Enabled,
		WebSearchBackend:      p.webSearch.Backend,
		BraveSearchAPIKey:     p.webSearch.BraveKey,
		TavilySearchAPIKey:    p.webSearch.TavilyKey,
		DeepSeekSearchBaseURL: p.webSearch.DeepSeekBaseURL,
		DeepSeekSearchAPIKey:  p.webSearch.DeepSeekAPIKey,
		DeepSeekSearchModel:   p.webSearch.DeepSeekModel,
		WebSearchProxy:        p.webSearch.Proxy,
		BashEnv:               proxyEnv(p.proxyAddr, p.proxyCACert), // Bash 子命令默认走代理+信任 CA
		WorkingDir:            taskDir,                              // 本任务工作目录 <workDir>/tasks/<taskID>
		ToolOutputDir:         cmdOutDir(taskDir),
		MaxTurns:              p.maxTurns, // 0 = unlimited (configurable in agent management)
		MaxDuration:           maxDur,     // 0=不限;有 deadline 时=距 deadline 剩余
		Compaction:            compactionConfig(p.compactionWindow()),
		// 跨唤醒共享的规划待办：让串行链在多轮之间保留（session 是新的，store 不是）。
		Todos: p.todoFor(ts.ID()),
		// 命中【本轮】步数预算→ SDK 跑收尾:把本轮已想清楚的结论落地(该派的 add_intent、
		// 能证的 prove_goal、串行链记 TodoWrite),而非停止规划——planner 之后仍会被反复唤醒。
		// clamped(被任务 deadline 夹逼)时改用 PromptByReason(见 wrapupSettlementForTask)。
		Settlement:   settle,
		NonStreaming: p.nonStreaming(), // 该 profile 选非流式时走 Provider.Complete
		MaxTokens:    p.maxTokens(),    // 0 = 不发上限,由服务端默认值决定
	}
	if p.tx != nil { // persist raw LLM conversation; one accumulating file per task's planner
		opts.Transcript = p.tx
		opts.SessionID = fmt.Sprintf("exp%d-planner", ts.ID())
	}
	// 实验功能:开启后由 noa 接管上下文压缩(归档集中在 <workDir>/noa/<SessionID> 下,持久)。
	noaSession := fmt.Sprintf("exp%d-planner", ts.ID())
	enableNoa(&opts, p.noaEnabledFn, p.workDir, noaSession, noaWarn(noaSession))
	// 态势（刚完成的意图 + 完整图）现在拼进本轮 user 输入（见下方 input）。user 里还有
	// 指令 + 跨唤醒待办（todo 是模型自己的规划便签，可再生，放 user 即可）。
	// 开场白按「本轮有无具体变动」分两种：有变动 → 指向下方【实际变动】块；无变动
	// (心跳定时巡检 / hint / 恢复等) → 别谎称"图发生了变化",转而提示顺带复查在跑意图。
	lead := "A specific change just occurred (see [Actual changes that triggered this round] below). Plan the next step accordingly:"
	if len(triggers) == 0 {
		lead = "This wake-up is a **scheduled inspection (heartbeat due) / no specific change signal**; the graph may not have changed. Also review running intents: use steer_work to correct prolonged lack of progress or drift, and kill_work to stop an entirely wrong direction. Then assess goals and decide whether to add directions:"
		// 心跳/无变动唤醒时,若全图已无任何 open 或 running 意图 → 探索已停摆(没 worker 在跑、
		// 也没排队方向)。明确告知 planner 并强制其本轮补出新方向,别只复查在跑意图后空转一轮。
		if active, err := ts.HasActiveIntent(); err == nil && !active {
			lead = "This wake-up is a scheduled inspection with no open or running intents. Assess whether the objective is achieved. If it is unmet, add a genuinely new in-scope direction. If two confirmed facts show repeated external unavailability after a bounded recheck and no useful direction remains, call await_condition with both IDs and the concrete resume condition:"
		}
	}
	input := lead + situational + "\n\nAssess goals using the situation above. When a goal is truly achieved, mark it individually with prove_goal and the appropriate outcome. For a confirmed finding_workflow goal, wait for the saved finding, bound traffic and current detailed report before proving it; a reporter save will trigger another planning round. A verified negative result may use outcome=negative with its supporting fact. Unavailable evidence is inconclusive and must leave the goal open. With no active intents, add a genuinely new direction if one exists; after one documented recheck of the same unavailable condition with no useful direction left, call await_condition. Never redispatch an unchanged failed request indefinitely.**" +
		renderPlannerTodos(opts.Todos.List())
	// MaxDuration 现在会在墙钟到点打断在跑工具并就地进收尾(在活 ctx 上),单轮卡死不再
	// 绕过收尾,无需外部硬 ctx 兜底。ctx 只承载 pause / kill / shutdown。
	_, _, err = captureRun(ctx, opts, input,
		func(r db.Activity) {
			if emit != nil {
				r.Worker = "planner" // planner activity has no intent_id (it generates them)
				emit(r)
			}
		})
	if err == nil && !tsx.GoalMet && tsx.AwaitReason != "" {
		return false, tsx.AwaitReason, ErrAwaitCondition
	}
	return tsx.GoalMet, tsx.Reason, err
}
