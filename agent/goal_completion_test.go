package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

func callGoalTool(t *testing.T, tool actool.CoreTool, input any) actool.Result {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := tool.Call(context.Background(), raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestStructuredGoalCompletionKeepsNegativeAndObservationPaths(t *testing.T) {
	d := testDB(t)
	defer d.Close()
	task, err := d.CreateTask("structured completion", "local synthetic observation", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(task.ID)
	store := d.Exploration(task.ExplorationID)
	tools := NewToolSet(store, "planner")
	tools.SetTaskID(task.ID)
	tools.SetAssetStore(d.Assets(), d.Companies())
	if r := callGoalTool(t, tools.goalMet(), map[string]any{"reason": "no goals"}); !r.IsError {
		t.Fatal("goal_met must reject a task before goals are persisted")
	}

	goal, err := tools.addOneGoal(goalItem{Text: "A confirmed result requires saved deliverables", Completion: "finding_workflow"})
	if err != nil {
		t.Fatal(err)
	}
	fact, err := store.AddNode(db.KindFact, map[string]any{"summary": "verified negative response"}, 0, "confirmed", "worker", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r := callGoalTool(t, tools.proveGoal(), map[string]any{"goal_id": goal, "evidence_id": fact, "outcome": "confirmed"}); !r.IsError {
		t.Fatal("fact must not bypass confirmed finding workflow", r.Flatten())
	}
	if r := callGoalTool(t, tools.proveGoal(), map[string]any{"goal_id": goal, "evidence_id": fact, "outcome": "negative"}); r.IsError {
		t.Fatal("verified negative result should remain provable", r.Flatten())
	}
	observationGoal, err := tools.addOneGoal(goalItem{Text: "Record an observation", Completion: "observation"})
	if err != nil {
		t.Fatal(err)
	}
	if r := callGoalTool(t, tools.proveGoal(), map[string]any{"goal_id": observationGoal, "evidence_id": fact}); r.IsError {
		t.Fatal("observation-only fact proof should remain valid", r.Flatten())
	}
	openGoal, err := tools.addOneGoal(goalItem{Text: "Open finding workflow", Completion: "finding_workflow"})
	if err != nil {
		t.Fatal(err)
	}
	if r := callGoalTool(t, tools.goalMet(), map[string]any{"reason": "bypass"}); !r.IsError || !strings.Contains(r.Flatten(), "cannot bypass") {
		t.Fatal("goal_met bypass must reject open structured workflow", r.Flatten())
	}
	if node, _ := store.GetNode(openGoal); node == nil || node.State != "open" {
		t.Fatal("rejected bypass changed goal state")
	}
}

func TestAwaitConditionRequiresRepeatedFactsAndNoActiveWork(t *testing.T) {
	d := testDB(t)
	defer d.Close()
	task, err := d.CreateTask("await condition", "local unavailable proof", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(task.ID)
	store := d.Exploration(task.ExplorationID)
	tools := NewToolSet(store, "planner")
	tools.SetTaskID(task.ID)
	tools.SetAssetStore(d.Assets(), d.Companies())
	if _, err := tools.addOneGoal(goalItem{Text: "Verify proof", Completion: "finding_workflow"}); err != nil {
		t.Fatal(err)
	}
	one, err := store.AddNode(db.KindFact, map[string]any{"summary": "first unavailable observation"}, 0, "confirmed", "worker", nil)
	if err != nil {
		t.Fatal(err)
	}
	two, err := store.AddNode(db.KindFact, map[string]any{"summary": "recheck remains unavailable"}, 0, "confirmed", "worker", nil)
	if err != nil {
		t.Fatal(err)
	}
	await := tools.awaitCondition()
	if r := callGoalTool(t, await, map[string]any{"evidence_ids": []int64{one, one}, "condition": "service recovers"}); !r.IsError {
		t.Fatal("one observation must not pause")
	}
	intent, err := store.AddNode(db.KindIntent, map[string]any{"summary": "active recheck"}, 0, "open", "planner", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r := callGoalTool(t, await, map[string]any{"evidence_ids": []int64{one, two}, "condition": "service recovers"}); !r.IsError {
		t.Fatal("active worker intent must block pause")
	}
	if err := store.SetNodeState(intent, "done"); err != nil {
		t.Fatal(err)
	}
	if r := callGoalTool(t, await, map[string]any{"evidence_ids": []int64{one, two}, "condition": "service returns a usable response"}); r.IsError || tools.AwaitReason == "" {
		t.Fatal("bounded unavailable condition should pause with goals open", r.Flatten())
	}
}
