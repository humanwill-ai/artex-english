package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

const findingIDGuidance = "\n\n**Finding ID conventions**: finding_id is the independent finding record ID; finding_node_id is the exploration node ID. The id from list_findings / list_task_findings / node_detail / get_task_node_detail remains the exploration node ID; read the independent ID from finding_id in the same response. get_finding_traffic / bind_finding_traffic use the independent finding_id. The legacy finding_id parameter of update_finding_report still takes finding_node_id. Do not use the number on the first line of report_finding for evidence tools, and do not guess other numbers after an ID error."

// The server supplies the persisted setting. A missing setting/host is off.
// Consulted at assembly and again on writes so an already-running session
// cannot keep binding after the user switches the feature off.
var FindingTrafficBindingEnabled func() bool

func findingTrafficBindingEnabled() bool {
	return FindingTrafficBindingEnabled != nil && FindingTrafficBindingEnabled()
}

// Applied after ToolResolve: user descriptions and prompts remain intact, while
// all actual reporters (including Planner and custom chat agents) see the same
// API contract. Disabled/unbound tools are never reintroduced here.
func findingWorkflowTools(agentKey string, tools []actool.CoreTool) ([]actool.CoreTool, string) {
	if !findingTrafficBindingEnabled() {
		out := make([]actool.CoreTool, 0, len(tools))
		for _, tool := range tools {
			if tool.Name() == "bind_finding_traffic" {
				continue
			}
			if agentKey == "reporter" && (tool.Name() == "traffic_search" || tool.Name() == "traffic_get" || tool.Name() == "traffic_blob") {
				continue
			}
			switch tool.Name() {
			case "report_finding", "add_hint", "add_task_hint":
				// Work on a copy: toggling back on must restore the original schema.
				raw, _ := json.Marshal(tool.InputSchema())
				var schema map[string]any
				if json.Unmarshal(raw, &schema) == nil {
					stripTrafficParameters(schema)
					tool = DecorateTool(tool, tool.Description(), schema)
				}
			}
			out = append(out, tool)
		}
		return out, ""
	}
	out := append([]actool.CoreTool(nil), tools...)
	has := map[string]bool{}
	for i, tool := range out {
		has[tool.Name()] = true
		note := ""
		switch tool.Name() {
		case "report_finding":
			tool = DecorateTool(tool, tool.Description(), findingHintRuntimeSchema(tool.InputSchema()))
			note = "\nBy default, the report agent verifies and binds traffic before writing the report. The reporter of a finding retains verification commands, key output, existing real traffic IDs and their purpose in evidence, so the report agent can check them against execution records; no extra traffic lookup is required merely for binding. Explicit immediate binding remains supported: traffic_refs or evidence_hint_id may submit verified references; the latter reads structured references from the specified hint in this task. Any invalid reference fails the entire submission. TCP/no-traffic cases do not need these optional parameters. Returned finding_id and finding_node_id identify the independent record and exploration node respectively."
		case "add_hint", "add_task_hint":
			note = "\nWhen handing off a confirmed finding, preserve verified traffic IDs, roles, notes, and order in traffic_refs of the corresponding hint (top level for a single hint, or in the corresponding hints element for a batch), and explain in text which specific finding they prove. Do not hand off only prose while discarding existing traffic references. Unverified candidates must not be passed as evidence."
		case "get_finding_traffic", "bind_finding_traffic", "list_findings", "list_task_findings", "node_detail", "get_task_node_detail", "update_finding_report":
			note = findingIDGuidance
		}
		if note != "" {
			out[i] = DecorateTool(tool, tool.Description()+note, tool.InputSchema())
		}
	}
	guidance := ""
	if has["report_finding"] || has["add_task_hint"] || has["add_hint"] {
		guidance = "\n\n**Traffic evidence handoff (optional)**: By default, the report agent performs automatic binding after the finding is stored and before writing the report. The finding reporter should retain verification commands, key output, existing real traffic IDs and their purpose in evidence, including intent_id within a task for traceability. Extra traffic lookup merely for binding is unnecessary. When Auto / Planner reports on behalf of an executor, do not discard the executor's existing references. add_hint / add_task_hint can hand off traffic_refs; explicit immediate binding remains compatible with traffic_refs / evidence_hint_id on report_finding. Register TCP or no-traffic findings normally; do not guess IDs or repeat probes solely to obtain traffic."
		if has["add_task_hint"] && !has["add_hint"] {
			guidance += "\nWhen platform chat has no task context, do not call report_finding directly. Hand off to the corresponding existing task via add_task_hint for its task agent to register the finding, then check the result with list_task_findings."
			guidance += "\nFor HTTP evidence, search traffic after the request that produced the observation. A search made before that request cannot establish that no capture exists afterward. Preserve the request's real traffic ID and relevant response details in the finding evidence or hint for the report agent."
		}
		if has["prove_goal"] || has["goal_met"] {
			guidance += "\nBefore declaring the goal complete, finish reporting/handing off the evidence already available from this run. Do not end the task or cancel workers merely because a textual finding has been registered while evidence handoff is incomplete. No-traffic cases do not require waiting or forcing traffic capture."
		}
	}
	if has["update_finding_report"] && has["bind_finding_traffic"] && has["get_finding_traffic"] {
			guidance += "\n\n**Automatic traffic association before reporting (enabled)**: You are responsible for verifying and binding traffic for the finding that triggered this run before writing its report. First obtain explicit finding_id and finding_node_id from the report_finding JSON response or get_task_node_detail / list_task_findings. Read the finding details, the corresponding intent's execution records, and existing evidence list; prefer real IDs handed off by the finding reporter. If this verification used HTTP and traffic tools are available, run traffic_search after the request that produced the observation, then inspect each candidate with traffic_get to verify that its request/response actually supports this finding. A search from before the request cannot prove there is no later capture. Domain and time only filter candidates and do not prove association. Bind confirmed evidence in reproduction order using bind_finding_traffic(finding_id, traffic_refs), selecting baseline / proof / verification / supporting and explaining each role. Work only on this finding; do not create duplicate findings or probe the target again. After successful binding, call get_finding_traffic again for the latest version, read the required bodies, and pass the version actually read as evidence_version to update_finding_report (whose finding_id parameter still takes finding_node_id). Existing bindings need not be appended again. For TCP, uncaptured traffic, unavailable tools, or no exact match, skip automatic binding and write the report normally from text/command evidence, explaining why; do not guess merely to fill the traffic list. Do not claim binding succeeded if it failed; preserve existing evidence and explain the failure to bind in the report."
	}
	if guidance != "" || has["get_finding_traffic"] || has["update_finding_report"] {
		guidance += findingIDGuidance
	}
	return out, guidance
}

// findingHintRuntimeSchema updates only the native handler's optional hint contract,
// after stored tool overrides. Keep persisted defaults and custom prose intact; the
// returned shallow copies isolate every map we modify from the stored/base schema.
// Deliberately removed properties stay removed. Other IDs retain their contracts.
func findingHintRuntimeSchema(schema map[string]any) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	hint, ok := props["evidence_hint_id"].(map[string]any)
	if !ok {
		return schema
	}
	out := make(map[string]any, len(schema))
	for key, value := range schema {
		out[key] = value
	}
	nextProps := make(map[string]any, len(props))
	for key, value := range props {
		nextProps[key] = value
	}
	nextHint := make(map[string]any, len(hint))
	for key, value := range hint {
		nextHint[key] = value
	}
	nextHint["type"] = []string{"integer", "null"}
	nextHint["minimum"] = 1
	const guidance = "Optional: omit evidence_hint_id or pass null when no handoff hint exists. Otherwise use a positive integer ID of an existing hint in this task; never pass 0 or invent an ID."
	description, _ := nextHint["description"].(string)
	if !strings.HasSuffix(description, guidance) {
		if description != "" {
			description += "\n"
		}
		nextHint["description"] = description + guidance
	}
	nextProps["evidence_hint_id"] = nextHint
	out["properties"] = nextProps
	return out
}

func stripTrafficParameters(schema map[string]any) {
	props, _ := schema["properties"].(map[string]any)
	delete(props, "traffic_refs")
	delete(props, "evidence_hint_id")
	if required, ok := schema["required"].([]any); ok {
		kept := required[:0]
		for _, key := range required {
			if key != "traffic_refs" && key != "evidence_hint_id" {
				kept = append(kept, key)
			}
		}
		schema["required"] = kept
	}
	if hints, ok := props["hints"].(map[string]any); ok {
		if items, ok := hints["items"].(map[string]any); ok {
			stripTrafficParameters(items)
		}
	}
}

// HintTrafficSchema is shared by the task-local and cross-task hint tools.
func HintTrafficSchema() map[string]any {
	return map[string]any{"type": "array", "description": "Optional: verified traffic references for the specific finding in this hint, preserving order. After handoff, report_finding can carry these references via evidence_hint_id.", "items": obj(map[string]any{"traffic_id": str("Real traffic ID"), "role": str("baseline / proof / verification / supporting"), "note": str("What conclusion this traffic supports")}, "traffic_id")}
}

func (t *ToolSet) findingRefsFromHint(hintID int64, explicit []db.TrafficRef) ([]db.TrafficRef, error) {
	if hintID <= 0 {
		return db.NormalizeTrafficRefs(explicit)
	}
	n, err := t.ts.GetNode(hintID) // local store only: inherited hints cannot supply evidence
	if err != nil {
		return nil, err
	}
	if n == nil || n.Kind != db.KindHint {
		return nil, fmt.Errorf("evidence_hint_id=%d must be a hint node in this task (inherited hints cannot be used directly for binding)", hintID)
	}
	var payload struct {
		Refs []db.TrafficRef `json:"traffic_refs"`
	}
	if err := json.Unmarshal(n.Payload, &payload); err != nil {
		return nil, err
	}
	return db.NormalizeTrafficRefs(append(append([]db.TrafficRef{}, explicit...), payload.Refs...))
}
