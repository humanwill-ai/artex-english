package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	actool "github.com/Autumn-27/norma/tool"
)

// TestBuiltinToolSeeds ensures the catalog builds from a nil-store ToolSet without
// panicking, has stable keys, unions agent bindings, and carries real descriptions.
func TestBuiltinToolSeeds(t *testing.T) {
	seeds := BuiltinToolSeeds()
	if len(seeds) == 0 {
		t.Fatal("no seeds")
	}
	byKey := map[string]ToolSeed{}
	for _, s := range seeds {
		if s.Key == "" || s.Desc == "" {
			t.Errorf("seed %q missing key/desc", s.Key)
		}
		byKey[s.Key] = s
	}
	// record_fact is bound to worker (also mainagent, which can log confirmed facts).
	rf, ok := byKey["record_fact"]
	hasWorker := false
	for _, a := range rf.Agents {
		if a == "worker" {
			hasWorker = true
		}
	}
	if !ok || !hasWorker {
		t.Errorf("record_fact agents = %v, want to include worker", rf.Agents)
	}
	// add_task_scope is bound to the planner (deliberate scope widening).
	if ts, ok := byKey["add_task_scope"]; !ok || len(ts.Agents) == 0 {
		t.Errorf("add_task_scope not seeded / has no agent binding: %v", ts.Agents)
	}
	// SDK generic tools (incl. sleep, now part of DefaultTools) are deliberately NOT
	// seeded — every agent owns them; they flow through ToolResolve untouched.
	for _, k := range []string{"Bash", "Read", "Write", "Edit", "Grep", "sleep"} {
		if _, ok := byKey[k]; ok {
			t.Errorf("SDK tool %q should not be seeded", k)
		}
	}
}

// TestDecorateToolInjectsDefaults verifies a schema "default" fills a missing param
// before the underlying handler runs, and an explicitly-provided value is kept.
func TestDecorateToolInjectsDefaults(t *testing.T) {
	var seen map[string]any
	base := actool.Build(actool.Spec{
		Name: "probe", Description: "orig",
		Run: func(_ context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			_ = json.Unmarshal(in, &seen)
			return actool.Text("ok"), nil
		},
	})
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"limit": map[string]any{"type": "integer", "description": "n", "default": float64(3)},
		"q":     map[string]any{"type": "string", "description": "query"},
	}}
	dec := DecorateTool(base, "new desc", schema)
	if dec.Description() != "new desc" {
		t.Errorf("description = %q", dec.Description())
	}

	// limit omitted → default 3 injected; q kept.
	if _, err := dec.Call(context.Background(), json.RawMessage(`{"q":"x"}`), nil); err != nil {
		t.Fatal(err)
	}
	if seen["limit"] != float64(3) || seen["q"] != "x" {
		t.Errorf("injected = %v, want limit=3 q=x", seen)
	}

	// limit provided → default does NOT override.
	if _, err := dec.Call(context.Background(), json.RawMessage(`{"limit":9}`), nil); err != nil {
		t.Fatal(err)
	}
	if seen["limit"] != float64(9) {
		t.Errorf("limit = %v, want 9 (no override)", seen["limit"])
	}
}

func TestDecoratedFindingExplicitNullPreservesNoHint(t *testing.T) {
	oldPolicy := FindingTrafficBindingEnabled
	FindingTrafficBindingEnabled = func() bool { return true }
	t.Cleanup(func() { FindingTrafficBindingEnabled = oldPolicy })
	for _, defaultID := range []int{0, 37} {
		for _, name := range []string{"report_finding", "unrelated_tool"} {
			t.Run(fmt.Sprintf("%s/default_%d", name, defaultID), func(t *testing.T) {
				var seen map[string]json.RawMessage
				base := actool.Build(actool.Spec{Name: name, Run: func(_ context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
					seen = nil
					if err := json.Unmarshal(in, &seen); err != nil {
						return actool.Result{}, err
					}
					return actool.Text("ok"), nil
				}})
				// Model the stored pre-fix schema inside the runtime nullable wrapper.
				stored := map[string]any{"type": "object", "properties": map[string]any{
					"evidence_hint_id": map[string]any{"type": "integer", "default": defaultID},
					"limit":            map[string]any{"type": "integer", "default": 5},
				}}
				inner := DecorateTool(base, "CUSTOM", stored)
				assembled, _ := findingWorkflowTools("worker", []actool.CoreTool{inner})
				for _, tc := range []struct{ input, want string }{
					{`{"evidence_hint_id":null,"limit":null}`, "null"},
					{`{"evidence_hint_id": null ,"limit":null}`, "null"},
					{`{"limit":null}`, fmt.Sprint(defaultID)},
					{`{"evidence_hint_id":23,"limit":null}`, "23"},
				} {
					if _, err := assembled[0].Call(t.Context(), json.RawMessage(tc.input), nil); err != nil {
						t.Fatal(err)
					}
					want := tc.want
					if name != "report_finding" && want == "null" {
						want = fmt.Sprint(defaultID)
					}
					if string(seen["evidence_hint_id"]) != want {
						t.Fatalf("input %s: hint = %s, want %s", tc.input, seen["evidence_hint_id"], want)
					}
					if string(seen["limit"]) != "5" {
						t.Fatalf("unrelated null/default changed: %s", seen["limit"])
					}
				}
			})
		}
	}
}
