package server

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
)

func TestEnglishDefaultToolContractsMatchSourceEdition(t *testing.T) {
	// Compare actual factory schemas against the frozen original seed. Only schema
	// prose may change: property names/types, enum values, defaults and constraints
	// must remain identical. No tool handler is executed.
	strip := func(raw json.RawMessage) any {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		var walk func(any)
		walk = func(v any) {
			switch v := v.(type) {
			case map[string]any:
				for k, c := range v {
					if _, ok := c.(string); ok && (k == "description" || k == "title") {
						delete(v, k)
					} else {
						walk(c)
					}
				}
			case []any:
				for _, c := range v {
					walk(c)
				}
			}
		}
		walk(value)
		return value
	}
	current := (&Server{}).currentEnglishToolDefaults()
	for _, old := range legacyToolDefaults {
		next, ok := current[old.Key]
		if !ok {
			t.Errorf("original tool missing: %s", old.Key)
			continue
		}
		currentContract := strip(next.Schema)
		if schema, ok := currentContract.(map[string]any); ok {
			props, _ := schema["properties"].(map[string]any)
			switch old.Key {
			case "prove_goal":
				if _, ok := props["outcome"]; !ok {
					t.Error("prove_goal outcome contract missing")
				}
				delete(props, "outcome")
			case "set_goals":
				if _, ok := props["completion"]; !ok {
					t.Error("set_goals completion contract missing")
				}
				delete(props, "completion")
			case "update_finding_report":
				if _, ok := props["evidence_version"]; !ok {
					t.Error("report evidence_version contract missing")
				}
				required, _ := schema["required"].([]any)
				if len(required) == 0 || required[len(required)-1] != "evidence_version" {
					t.Error("report evidence_version must be required")
				}
				schema["required"] = []any{"finding_id", "report"}
			}
		}
		if !reflect.DeepEqual(strip(old.Schema), currentContract) {
			t.Errorf("non-prose schema contract changed for %s", old.Key)
		}
	}
}

func TestWorkflowToolSchemaUpgradePreservesEditedRows(t *testing.T) {
	s, _, _ := trafficEvidenceServer(t)
	key := "prove_goal"
	row, err := s.m.pg.GetTool(key)
	if err != nil || row == nil {
		t.Fatal(row, err)
	}
	t.Cleanup(func() {
		bindings, _ := json.Marshal(row.Agents)
		_ = s.m.pg.UpdateTool(key, row.Description, row.Schema, bindings, row.Enabled)
	})
	var old englishToolDefault
	for _, item := range legacyToolDefaults {
		if item.Key == key {
			old = item
		}
	}
	customBindings := json.RawMessage(`["planner","custom-agent"]`)
	if err := s.m.pg.UpdateTool(key, old.Description, old.Schema, customBindings, false); err != nil {
		t.Fatal(err)
	}
	s.upgradeWorkflowToolSchemas(s.currentEnglishToolDefaults())
	upgraded, err := s.m.pg.GetTool(key)
	if err != nil || upgraded == nil {
		t.Fatal(upgraded, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(upgraded.Schema, &schema); err != nil {
		t.Fatal(err)
	}
	props := objectProperty(schema, "properties")
	if _, ok := props["outcome"]; !ok || upgraded.Enabled || !reflect.DeepEqual(upgraded.Agents, []string{"planner", "custom-agent"}) {
		t.Fatalf("pristine contract was not upgraded safely: %+v", upgraded)
	}
	if err := s.m.pg.UpdateTool(key, "USER CUSTOM DESCRIPTION", old.Schema, customBindings, false); err != nil {
		t.Fatal(err)
	}
	s.upgradeWorkflowToolSchemas(s.currentEnglishToolDefaults())
	custom, err := s.m.pg.GetTool(key)
	if err != nil || custom == nil || custom.Description != "USER CUSTOM DESCRIPTION" || !db.SameSchemaContract(custom.Schema, old.Schema) {
		t.Fatalf("edited row changed: %+v err=%v", custom, err)
	}
}

func TestLegacyPromptSnapshotsCoverAllDefaultAgents(t *testing.T) {
	for key := range agent.BuiltinPromptSeeds() {
		if len(legacyPromptDefaults[key]) != 1 || strings.TrimSpace(legacyPromptDefaults[key][0]) == "" {
			t.Errorf("missing source seed for %s", key)
		}
	}
	for _, key := range []string{"reporter", "finding_retester"} {
		if len(legacyPromptDefaults[key]) != 1 {
			t.Errorf("missing source seed for %s", key)
		}
	}
	if reporterToolCallMessage == reporterToolCallMessageV1 || reporterToolCallMessage == reporterToolCallMessageV2 {
		t.Fatal("new trigger still uses legacy prompt")
	}
}

// Use unique temporary rows so a configured developer database never has its
// actual reporter, retester, or built-in prompt history changed by this test.
func TestEnglishDefaultPromptMigrationProvenancePG(t *testing.T) {
	dsn, _, err := db.DSN()
	if err != nil {
		t.Skip("test postgres not configured")
	}
	pg, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	s := &Server{m: &Manager{pg: pg}}
	defaults := agent.BuiltinPromptSeeds()
	defaults["reporter"] = agent.ReporterDefaultPrompt
	defaults[db.FindingRetestAgentKey] = agent.RetesterDefaultPrompt
	for _, kind := range []string{"worker", "reporter", db.FindingRetestAgentKey} {
		for _, variant := range []string{"pristine", "custom_text", "custom_metadata", "user_copy", "later_version", "selected_old_version", "wrong_role"} {
			t.Run(kind+"/"+variant, func(t *testing.T) {
				key := fmt.Sprintf("english_server_migration_%d", time.Now().UnixNano())
				name, description := legacyReporterName, legacyReporterDescription
				legacyKey := kind
				if kind == db.FindingRetestAgentKey {
					name, description = legacyRetesterName, legacyRetesterDescription
					legacyKey = "finding_retester"
				}
				if variant == "custom_metadata" {
					name += " custom"
				}
				a, err := pg.CreateAgent(key, name, description)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _, _ = pg.Exec(`DELETE FROM agents WHERE id=$1`, a.ID) }()
				if kind == "worker" {
					a.Builtin = true
					if _, err := pg.Exec(`UPDATE agents SET builtin=true WHERE id=$1`, a.ID); err != nil {
						t.Fatal(err)
					}
				}
				if variant == "wrong_role" {
					a.Role = "worker"
				}
				original := legacyPromptDefaults[legacyKey][0]
				if variant == "custom_text" {
					original += "\nCustom instruction"
				}
				by, note := "system", "内置默认"
				if variant == "user_copy" {
					by, note = "operator", "Copied prompt"
				}
				if _, err := pg.SavePrompt(a.ID, original, note, by); err != nil {
					t.Fatal(err)
				}
				if variant == "later_version" || variant == "selected_old_version" {
					if _, err := pg.SavePrompt(a.ID, original, "User edit", "operator"); err != nil {
						t.Fatal(err)
					}
				}
				if variant == "selected_old_version" {
					if _, err := pg.Exec(`UPDATE agents SET current_prompt_id=(SELECT id FROM agent_prompts WHERE agent_id=$1 AND version=1) WHERE id=$1`, a.ID); err != nil {
						t.Fatal(err)
					}
				}
				// Candidate retains its actual row/history ID; key selects the same
				// provenance branch used by startup without touching canonical rows.
				a.Key = kind
				changed, err := s.migrateEnglishDefaultPrompt(a, defaults[kind])
				want := variant == "pristine" || kind == "worker" && (variant == "custom_metadata" || variant == "wrong_role")
				if err != nil || changed != want {
					t.Fatalf("changed=%v want=%v err=%v", changed, want, err)
				}
				current, err := pg.CurrentPrompt(a.ID)
				if err != nil {
					t.Fatal(err)
				}
				expect := original
				if want {
					expect = defaults[kind]
				}
				if current != expect {
					t.Fatal("wrong active prompt or custom prompt overwritten")
				}
				var historical string
				if err := pg.QueryRow(`SELECT template_text FROM agent_prompts WHERE agent_id=$1 AND version=1`, a.ID).Scan(&historical); err != nil || historical != original {
					t.Fatal("original prompt history changed", err)
				}
				if changed, err := s.migrateEnglishDefaultPrompt(a, defaults[kind]); err != nil || changed {
					t.Fatalf("repeat migration changed=%v err=%v", changed, err)
				}
				stored, err := pg.GetAgentByKey(key)
				if err != nil {
					t.Fatal(err)
				}
				if want && kind == db.FindingRetestAgentKey {
					if stored.Name != retesterName || stored.Description != retesterDescription {
						t.Fatal("pristine retester metadata not translated")
					}
				} else if stored.Name != name || stored.Description != description {
					t.Fatal("unrelated/custom metadata changed")
				}
			})
		}
	}
}

func TestEnglishLogLevelsPreserveLegacyClassification(t *testing.T) {
	for _, message := range []string{"write failed", "discarded activity record", "refusing target", "request rejected", "host unreachable", "写入失败", "丢弃活动", "拒绝", "不可达"} {
		if got := levelOf(message); got != "error" {
			t.Errorf("%q level=%s", message, got)
		}
	}
	for _, message := range []string{"retrying in 1s", "重试", "skipped event", "worker stopped"} {
		if got := levelOf(message); got != "warn" {
			t.Errorf("%q level=%s", message, got)
		}
	}
	if got := levelOf("planning complete"); got != "info" {
		t.Errorf("normal level=%s", got)
	}
}
