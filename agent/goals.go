package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/llm"
	acperm "github.com/Autumn-27/norma/permission"
	actool "github.com/Autumn-27/norma/tool"
	"github.com/Autumn-27/norma/transcript"
)

// goalsDefaultTmpl is the built-in EDITABLE body (段 [A]) of the goals-decomposer
// prompt, seeded into agent_prompts. No template vars are used today.
const goalsDefaultTmpl = `You decompose penetration-testing goals. Identify the **final outcomes the user wants to achieve**, rather than planning attack steps.

**First step, before decomposing goals: extract operational constraints**
From the task objective/description, identify explicit operator rules about **which operations are allowed or prohibited** and register each with set_constraints. If neither contains operational constraints, no extraction is needed:
- type=deny: prohibited operations (for example, no port scanning, no writes/deletions in production, no brute force, or no access to a particular subdomain).
- type=allow: explicitly permitted or limited scope of operations (for example, passive reconnaissance only or a specific domain only).
- A constraint is neither a goal nor an attack step; it defines operational boundaries.
- **Constraints must be self-contained and name concrete targets**. Replace references such as "current target/current port/current IP/current domain/this site" with the **specific values** in the task objective/description. Constraints are injected separately into execution prompts, so references without context cannot identify their targets.
  Example: for https://abc.example.net, write "Only test abc.example.net," not "Only test the current target"; write "Only test target port 443; do not scan other ports," not "Only test the current port." If the original says "current target" but the address is explicit, insert that address.
- **Register only constraints explicitly stated or emphasized in the objective/description. Never invent them.** When uncertain about type, use deny as the more conservative choice.
- If the objective/description contains no operational constraints, **do not** call set_constraints.
After registering any constraints, decompose the goals below.

**A goal is a final deliverable/verifiable outcome.**

**Not goals (must not be listed as subgoals)**:
- Information gathering, reconnaissance, endpoint scanning
- Vulnerability analysis and verification processes
- Attack steps or exploitation methods
- Result-verification steps

**Decomposition rules**:
- One final objective in the user's description → produce one goal.
- Multiple **independent** final deliverables → list them separately.
- Set vulnclass when a specific vulnerability class applies; leave it empty for information-gathering/business-logic goals.
- Never invent goals the user did not mention.
- For each goal, set structured completion=finding_workflow when the requested successful result includes a saved finding, bound traffic evidence, and a current detailed report. Set completion=observation for observation-only deliverables. This field describes the requested deliverable, not whether testing has succeeded. A verified negative result may still be proved later without a finding.

Submit the results with set_goals.`

// goalsScopeTail is the code-owned tail appended after the editable goals body
// WHEN an asset store + task context are available. It teaches the decomposer to
// also lift the explicit asset scope out of the goal/description and register it
// via add_task_scope. Kept in code (not the DB-editable body) so it always applies
// on released DBs and can't be edited away — same pattern as the trafficTool tail.
const goalsScopeTail = `

**Additional responsibility: register the test asset scope**
In addition to decomposing goals, identify the **explicitly supplied test asset scope** in the task objective/description and register it with add_task_scope. This is the task's authorization boundary and the denominator for asset-testing coverage. **Minimum-scope principle: register only the exact target explicitly named by the user; never expand it yourself.**
- A URL or hostname address (such as https://xxx.example.com/path or app.example.com) → use the **full hostname**, kind=subdomain, value=full hostname.
  Example: https://a1b2c3.lab.example.net/path → kind=subdomain, value=a1b2c3.lab.example.net (**not** example.net).
  **Never** shorten a hostname containing subdomains to the root domain. Registering all of example.com when the user supplied xxx.example.com expands beyond the user's target and violates the minimum-scope principle.
- Use kind=root_domain, value=example.com only when the user supplies the **bare root domain with no subdomain** (such as example.com), or explicitly says "the entire site / all subdomains / the entire domain."
- A plain IP address or network range → kind=ip / cidr, value=IP or CIDR.
- **Do not** register company scope (company). A newly created task usually has no corresponding company in the asset system, so registration would fail. Leave company-level scope to a later planning stage.
Other rules:
- Register only scope **explicitly stated in the objective/description**. Never invent or infer unmentioned domains/IPs.
- Briefly identify the source statement in reason for auditability.
- If no explicit asset scope exists, **do not** call add_task_scope.
Register any scope with add_task_scope first, then submit goals with set_goals.`

// goalsSystem assembles the goals-decomposer system prompt: the rendered body
// [A] (DB-overridable), the code-owned scope-extraction tail when add_task_scope
// is wired (withScope), and the code-owned English output-language tail [C] last —
// mirroring chatSystem/plannerSystem so a DB-edited body can never drop the tail.
// DecomposeGoalsWithProvider and the localization test share this one assembly, so
// the langDirective tail can't drift between runtime and test. EngagementDescription
// is intentionally left empty: the task description rides in the user message, not
// the {{.EngagementDescription}} var (see DecomposeGoalsWithProvider).
func goalsSystem(dataDir string, withScope bool) string {
	sys := renderSystem("goals", goalsDefaultTmpl, GoalsVars{DataDir: dataDir, Now: nowStr()})
	if withScope {
		sys += goalsScopeTail
	}
	return sys + langDirective()
}

// GoalSpec is one decomposed objective.
type GoalSpec struct {
	Text      string `json:"text"`
	VulnClass string `json:"vulnclass,omitempty"`
}

// DecomposeGoals asks the LLM to break a pentest task goal into discrete,
// independently-verifiable objectives (each becomes a goal node). Returns nil if
// no provider is configured or the call yields nothing — the caller then falls
// back to a rule-based split so goal nodes always exist.
//
// prov is supplied by the caller (rather than built here from a Config) so goal
// decomposition rides the SAME provider instance as the rest of the engine — it
// shares the rate limiter, gets recorded by llmrec, and participates in LLM
// failover instead of quietly bypassing all three.
//
// desc is the task's free-text description (背景：靶标范围/flag 数量/交战说明等).
// It is fed alongside the goal so the decomposer no longer splits blind — the
// prompt still forbids inventing anything the two texts don't state.
//
// emit, when non-nil, receives every LLM step (thinking/tool_use/result) with
// Worker="planner" so the round-0 goal-decomposition activity is visible in the UI.
//
// as + taskID, when non-nil/positive, wire the add_task_scope tool so the
// decomposer can register the explicit asset scope it extracts from the goal.
//
// ts is the task's exploration store: set_goals writes the decomposed goal nodes
// straight into it (the same managed tool the main agent uses to add goals at
// runtime). The returned specs are read back from the store so callers can emit
// per-goal activity and detect the "LLM produced nothing" case for their fallback.
func DecomposeGoals(ctx context.Context, prov llm.Provider, dataDir, goalText, desc string, as *db.AssetStore, ts *db.ExplorationStore, taskID int64, emit func(db.Activity)) []GoalSpec {
	if prov == nil {
		return nil
	}
	return DecomposeGoalsWithProvider(ctx, prov, dataDir, goalText, desc, as, ts, taskID, false, 0, emit)
}

// DecomposeGoalsWithProvider is the task-runtime variant used when a task has an
// ordered provider chain. It preserves the same tools and write behavior while
// letting the caller own provider selection/failover. maxTokens is the profile's
// per-reply output cap (0 = send none).
func DecomposeGoalsWithProvider(ctx context.Context, prov llm.Provider, dataDir, goalText, desc string, as *db.AssetStore, ts *db.ExplorationStore, taskID int64, nonStreaming bool, maxTokens int, emit func(db.Activity)) []GoalSpec {
	if prov == nil {
		return nil
	}
	// 目标拆解是一次性调用：不挂 transcript store，所以 agentcore 不会往 ctx 上挂
	// session id（它只在有 writer 时才挂，见 agentcore.Prompt）。而按 session-id 头
	// 做提示缓存/粘性路由的网关（opencode zen 缺 x-opencode-session 直接 400
	// MissingSessionID）读的就是 ctx 上这个值——不补就是「对话正常、拆解 400」。
	// 显式挂一个稳定 id：同一探索的拆解请求共享它（利于命中缓存），且命名与
	// planner/worker 不冲突，能被 llmrec.parseSession 正确归因。
	if ts != nil {
		ctx = transcript.WithSessionID(ctx, fmt.Sprintf("exp%d-goals", ts.ID()))
	}
	// worker="goals" tags the goal nodes' provenance; ts/taskID let set_goals link
	// each goal under the task root. This is the catalog's real set_goals tool, so a
	// web-edited description/schema on it applies here too.
	tsx := &ToolSet{as: as, ts: ts, taskID: taskID, worker: "goals"}
	// Wire add_task_scope only when we have a real asset store + task to write to.
	// goalsSystem appends the scope-extraction tail in lockstep (withScope) so the
	// prompt never asks for a tool that isn't present, and it owns the output-language
	// tail last so a DB-edited body can't drop it. Description rides in the user
	// message, NOT the {{.EngagementDescription}} var, so a prompt can't inject it twice.
	withScope := as != nil && taskID > 0
	sys := goalsSystem(dataDir, withScope)
	// set_constraints 始终可用(不依赖 asset store):正文已含「先抽操作约束再拆目标」这步
	// (可在 agent 编辑页改措辞),这里只需接上工具。
	tools := []actool.CoreTool{tsx.setGoals(), tsx.setConstraints()}
	if withScope {
		tools = append(tools, tsx.addTaskScope())
	}
	userMsg := "Task objective:\n" + goalText
	if d := strings.TrimSpace(desc); d != "" {
		userMsg += "\n\nTask description (background that may include target scope, flag count, or engagement instructions; reference only, do not invent details it does not mention):\n" + d
	}
	// Use captureRun so every LLM step is emitted as an activity record (visible in
	// the plan tab under the round-0 marker). Falls back gracefully when emit is nil.
	captureEmit := func(r db.Activity) {
		if emit != nil {
			r.Worker = "planner"
			emit(r)
		}
	}
	captureRun(ctx, agentcore.Options{
		Provider:               prov,
		SystemPrompt:           []string{sys},
		Tools:                  tools,
		PermissionMode:         acperm.ModeBypass,
		DisableBackgroundTasks: true,
		// 3 步(抽约束 → 登记范围 → 拆目标)各需一次工具调用,给足回合避免收尾前漏调 set_goals。
		MaxTurns:     8,
		NonStreaming: nonStreaming, // 该 profile 选非流式时走 Provider.Complete
		MaxTokens:    maxTokens,    // 0 = 不发上限,由服务端默认值决定
	}, userMsg, captureEmit)
	// set_goals persisted the goals directly; read them back so the caller sees what
	// was written (empty slice ⇒ the LLM produced nothing ⇒ caller falls back).
	if ts == nil {
		return nil
	}
	nodes, _ := ts.ListByKind(db.KindGoal, 10000)
	var out []GoalSpec
	for _, n := range nodes {
		var p struct {
			Text      string `json:"text"`
			VulnClass string `json:"vulnclass"`
		}
		_ = json.Unmarshal(n.Payload, &p)
		if strings.TrimSpace(p.Text) != "" {
			out = append(out, GoalSpec{Text: p.Text, VulnClass: p.VulnClass})
		}
	}
	return out
}
