package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	actool "github.com/Autumn-27/norma/tool"
)

func TestFindingWorkflowSharedAssemblyAndSwitch(t *testing.T) {
	oldPolicy, oldResolve, oldAugment := FindingTrafficBindingEnabled, ToolResolve, ToolAugment
	t.Cleanup(func() { FindingTrafficBindingEnabled, ToolResolve, ToolAugment = oldPolicy, oldResolve, oldAugment })
	ToolAugment = nil
	on := false
	FindingTrafficBindingEnabled = func() bool { return on }
	ts := NewToolSet(nil, "fixture")
	ToolResolve = func(_ context.Context, _ string, base []actool.CoreTool) []actool.CoreTool {
		out := make([]actool.CoreTool, len(base))
		for i, tool := range base {
			out[i] = DecorateTool(tool, "CUSTOM DESCRIPTION", tool.InputSchema())
		}
		return out
	}
	for _, role := range []string{"worker", "planner", "mainagent", "auto", "pentest", "custom-agent"} {
		for _, enabled := range []bool{false, true, false} {
			on = enabled
			out, def, cleanup := AugmentTools(t.Context(), role, []actool.CoreTool{ts.addFinding(), ts.addHint()})
			cleanup()
			system, _ := deferredSystem("USER CUSTOM PROMPT", def)
			if !strings.HasPrefix(system[0], "USER CUSTOM PROMPT") {
				t.Fatal("custom prompt replaced")
			}
			if strings.Contains(system[0], "traffic_refs") != on {
				t.Fatalf("%s: guidance ignored switch: %v", role, on)
			}
			if on && (!strings.Contains(system[0], "TCP") || !strings.Contains(system[0], "evidence_hint_id")) {
				t.Fatal("missing optional/handoff contract")
			}
			for _, tool := range out {
				if !strings.HasPrefix(tool.Description(), "CUSTOM DESCRIPTION") {
					t.Fatal("custom description replaced")
				}
				raw, _ := json.Marshal(tool.InputSchema())
				var schema map[string]any
				json.Unmarshal(raw, &schema)
				props := schema["properties"].(map[string]any)
				if (props["traffic_refs"] != nil) != on {
					t.Fatalf("%s: binding schema ignored switch", role)
				}
				if tool.Name() == "add_hint" {
					nested := props["hints"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
					if (nested["traffic_refs"] != nil) != on {
						t.Fatal("nested hint schema ignored switch")
					}
				}
			}
		}
	}
	on = true
	ToolResolve = func(context.Context, string, []actool.CoreTool) []actool.CoreTool { return nil }
	out, def, cleanup := AugmentTools(t.Context(), "planner", []actool.CoreTool{ts.addFinding()})
	defer cleanup()
	if len(out) != 0 || def.FindingGuidance != "" {
		t.Fatal("reintroduced disabled tool or its guidance")
	}
}

// A seeded database may still override the tool with the historical integer-only
// schema. Exercise the actual assembly order, including custom stored prose.
func TestFindingHintRuntimeContractAfterStoredOverride(t *testing.T) {
	oldPolicy, oldResolve, oldAugment := FindingTrafficBindingEnabled, ToolResolve, ToolAugment
	t.Cleanup(func() { FindingTrafficBindingEnabled, ToolResolve, ToolAugment = oldPolicy, oldResolve, oldAugment })
	ToolAugment = nil
	enabled := true
	FindingTrafficBindingEnabled = func() bool { return enabled }
	ts := NewToolSet(nil, "fixture")
	stored := ts.addFinding().InputSchema()
	props := stored["properties"].(map[string]any)
	props["evidence_hint_id"].(map[string]any)["description"] = "CUSTOM HINT DESCRIPTION"
	props["evidence_hint_id"].(map[string]any)["title"] = "CUSTOM TITLE"
	before, _ := json.Marshal(stored)
	ToolResolve = func(_ context.Context, _ string, base []actool.CoreTool) []actool.CoreTool {
		return []actool.CoreTool{DecorateTool(base[0], "CUSTOM TOOL DESCRIPTION", stored)}
	}
	for _, role := range []string{"worker", "planner", "mainagent", "auto", "pentest", "reporter", "custom-agent"} {
		for _, on := range []bool{true, false, true} {
			enabled = on
			out, _, cleanup := AugmentTools(t.Context(), role, []actool.CoreTool{ts.addFinding()})
			cleanup()
			if len(out) != 1 {
				t.Fatalf("%s: tool missing", role)
			}
			got := out[0]
			if !strings.HasPrefix(got.Description(), "CUSTOM TOOL DESCRIPTION") {
				t.Fatal("custom tool prose replaced")
			}
			raw, err := json.Marshal(got.InputSchema())
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatal(err)
			}
			gp := schema["properties"].(map[string]any)
			if !on {
				if gp["evidence_hint_id"] != nil || gp["traffic_refs"] != nil {
					t.Fatal("disabled binding retained parameters")
				}
			} else {
				hint := gp["evidence_hint_id"].(map[string]any)
				kinds, _ := hint["type"].([]any)
				if len(kinds) != 2 || kinds[0] != "integer" || kinds[1] != "null" || hint["minimum"] != float64(1) {
					t.Fatalf("runtime contract missing: %s", raw)
				}
				if !strings.HasPrefix(hint["description"].(string), "CUSTOM HINT DESCRIPTION\n") || hint["title"] != "CUSTOM TITLE" {
					t.Fatal("custom property prose replaced")
				}
				if gp["intent_id"].(map[string]any)["type"] != "integer" {
					t.Fatal("unrelated ID schema changed")
				}
				for _, required := range schema["required"].([]any) {
					if required == "evidence_hint_id" {
						t.Fatal("optional hint became required")
					}
				}
				// The SDK's limited validator must accept the newly advertised null. The
				// production handler independently checks all present non-null values.
				if err := actool.ValidateInput(got.InputSchema(), json.RawMessage(`{"vulnclass":"TEST","severity":"low","summary":"synthetic","evidence_hint_id":null}`)); err != nil {
					t.Fatalf("SDK rejects null: %v", err)
				}
				twice := findingHintRuntimeSchema(got.InputSchema())
				twiceRaw, _ := json.Marshal(twice)
				if string(raw) != string(twiceRaw) {
					t.Fatal("runtime schema correction is not idempotent")
				}
			}
			after, _ := json.Marshal(stored)
			if string(before) != string(after) {
				t.Fatal("assembly mutated persisted schema")
			}
		}
	}
}

func TestFindingHintRuntimeContractPreservesRemovedProperty(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"summary": str("CUSTOM")}}
	before, _ := json.Marshal(schema)
	after, _ := json.Marshal(findingHintRuntimeSchema(schema))
	if string(before) != string(after) {
		t.Fatal("reintroduced deliberately removed hint parameter")
	}
	if findingHintRuntimeSchema(nil) != nil {
		t.Fatal("nil schema changed")
	}
}
