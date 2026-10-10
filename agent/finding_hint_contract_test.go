package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

// A successful recorder spy lets this test isolate hint validation and reference
// handoff from the separately tested evidence-file persistence implementation.
type hintContractRecorder struct {
	calls int
	refs  []db.TrafficRef
}

func (r *hintContractRecorder) Record(_ context.Context, _ db.RecordFindingInput, refs []db.TrafficRef) (*db.RecordedFinding, error) {
	r.calls++
	r.refs = append([]db.TrafficRef(nil), refs...)
	return &db.RecordedFinding{Traffic: &db.FindingTraffic{}}, nil
}

func TestReportFindingOptionalHintContract(t *testing.T) {
	old := FindingTrafficBindingEnabled
	FindingTrafficBindingEnabled = func() bool { return true }
	t.Cleanup(func() { FindingTrafficBindingEnabled = old })
	d := testDB(t)
	defer d.Close()
	task, err := d.CreateTask("optional hint contract", "synthetic", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(task.ID)
	other, err := d.CreateTask("other task", "synthetic", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(other.ID)
	ts := NewToolSet(nil, "fixture")
	ts.ts, ts.taskID = d.Exploration(task.ExplorationID), task.ID
	explicit := db.TrafficRef{TrafficID: "direct-proof", Role: "proof"}
	handoff := db.TrafficRef{TrafficID: "hint-baseline", Role: "baseline"}
	hint, err := ts.ts.AddNode(db.KindHint, map[string]any{"summary": "verified handoff", "traffic_refs": []db.TrafficRef{handoff}}, 0, "active", "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := d.Exploration(other.ExplorationID).AddNode(db.KindHint, map[string]any{"summary": "other task hint", "traffic_refs": []db.TrafficRef{handoff}}, 0, "active", "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := ts.ts.AddNode(db.KindFact, map[string]any{"summary": "not a hint"}, 0, "confirmed", "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, value        string
		valid, carriesHint bool
	}{
		{"omitted", "", true, false}, {"null", "null", true, false},
		{"valid_hint", fmt.Sprint(hint), true, true},
		{"zero", "0", false, false}, {"negative", "-1", false, false},
		{"string_id", fmt.Sprintf("%q", fmt.Sprint(hint)), false, false},
		{"fraction", "1.5", false, false}, {"overflow", "9223372036854775808", false, false},
		{"boolean", "true", false, false}, {"object", "{}", false, false},
		{"array", "[]", false, false}, {"missing_hint", "9223372036854775807", false, false},
		{"foreign_task", fmt.Sprint(foreign), false, false}, {"wrong_kind", fmt.Sprint(fact), false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			recorder := &hintContractRecorder{}
			ts.SetFindingRecorder(recorder)
			notices := 0
			ts.notifyFinding = func(int64, string) { notices++ }
			before := ts.writes.Findings
			body := `{"vulnclass":"TEST","severity":"low","summary":"synthetic proof","traffic_refs":[{"traffic_id":"direct-proof","role":"proof"}]`
			if c.value != "" {
				body += `,"evidence_hint_id":` + c.value
			}
			body += `}`
			tools, _ := findingWorkflowTools("worker", []actool.CoreTool{ts.addFinding()})
			var called bool
			for _, tool := range tools {
				if tool.Name() != "report_finding" {
					continue
				}
				called = true
				if err := actool.ValidateInput(tool.InputSchema(), json.RawMessage(body)); c.valid && err != nil {
					t.Fatalf("valid input rejected by assembled schema: %v", err)
				}
				result, err := tool.Call(t.Context(), json.RawMessage(body), nil)
				if err != nil || result.IsError == c.valid {
					t.Fatalf("valid=%v err=%v result=%+v", c.valid, err, result)
				}
			}
			if !called {
				t.Fatal("report_finding missing")
			}
			if !c.valid {
				if recorder.calls != 0 || notices != 0 || ts.writes.Findings != before {
					t.Fatal("invalid hint reached persistence or notification")
				}
				return
			}
			want := []db.TrafficRef{explicit}
			if c.carriesHint {
				want = append(want, handoff)
			}
			if recorder.calls != 1 || notices != 1 || ts.writes.Findings != before+1 || !reflect.DeepEqual(recorder.refs, want) {
				t.Fatalf("lost/changed references or notification: %+v notices=%d writes=%+v", recorder, notices, ts.writes)
			}
		})
	}
}
