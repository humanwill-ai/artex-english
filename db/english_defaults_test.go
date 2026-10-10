package db

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestEnglishSchemaContract(t *testing.T) {
	before := json.RawMessage(`{"type":"object","properties":{"description":{"type":"string","description":"Original help"},"status":{"type":"string","enum":["ready","pending"]}},"required":["status"]}`)
	after := json.RawMessage(`{"required":["status"],"properties":{"status":{"enum":["ready","pending"],"type":"string"},"description":{"description":"Translated help","type":"string"}},"type":"object"}`)
	if !sameSchemaContract(before, after) {
		t.Fatal("description translation or JSON key order changed the contract")
	}
	for _, changed := range []json.RawMessage{
		json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["ready","done"]}},"required":["status"]}`),
		json.RawMessage(`{"type":"object","properties":{"description":{"type":"string"},"status":{"type":"string","enum":["ready","pending"]}},"required":[]}`),
		json.RawMessage(`{"type":"object","properties":{"description":{"type":"integer"},"status":{"type":"string","enum":["ready","pending"]}},"required":["status"]}`),
		json.RawMessage(`invalid`),
	} {
		if sameSchemaContract(before, changed) {
			t.Fatalf("accepted schema contract change: %s", changed)
		}
	}
}

func TestEnglishSchemaContractPreservesInstanceData(t *testing.T) {
	for _, keyword := range []string{"default", "const", "enum", "examples"} {
		before := json.RawMessage(fmt.Sprintf(`{"type":"object","%s":{"description":"original","title":"original"}}`, keyword))
		after := json.RawMessage(fmt.Sprintf(`{"type":"object","%s":{"description":"translated","title":"translated"}}`, keyword))
		if sameSchemaContract(before, after) {
			t.Errorf("accepted instance data changes under %s", keyword)
		}
	}
}

func englishDefaultsDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(testDSN(t))
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestEnglishPromptMigrationPreservesHistoryAndCustomization(t *testing.T) {
	d := englishDefaultsDB(t)
	for _, tc := range []struct {
		name, by, note, text          string
		builtin, edit, rollback, want bool
	}{
		{"legacy_seed", "system", "内置默认", "Original default", true, false, false, true},
		{"english_seed_note", "system", "Built-in default", "Original default", true, false, false, true},
		{"custom_text", "system", "内置默认", "Customized text", true, false, false, false},
		{"user_copy", "operator", "Copied default", "Original default", true, false, false, false},
		{"custom_agent", "system", "内置默认", "Original default", false, false, false, false},
		{"later_version", "system", "内置默认", "Original default", true, true, false, false},
		{"selected_old_version", "system", "内置默认", "Original default", true, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := fmt.Sprintf("english_migration_%d", time.Now().UnixNano())
			a, err := d.CreateAgent(key, "Test agent", "Synthetic migration test")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM agents WHERE id=$1`, a.ID) })
			if _, err := d.Exec(`UPDATE agents SET builtin=$2 WHERE id=$1`, a.ID, tc.builtin); err != nil {
				t.Fatal(err)
			}
			if _, err := d.SavePrompt(a.ID, tc.text, tc.note, tc.by); err != nil {
				t.Fatal(err)
			}
			if tc.edit {
				if _, err := d.SavePrompt(a.ID, tc.text, "User edit", "operator"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.rollback {
				if _, err := d.Exec(`UPDATE agents SET current_prompt_id=(SELECT id FROM agent_prompts WHERE agent_id=$1 AND version=1) WHERE id=$1`, a.ID); err != nil {
					t.Fatal(err)
				}
			}
			changed, err := d.MigratePristineEnglishPrompt(a.ID, []string{"Original default"}, "English default", true)
			if err != nil || changed != tc.want {
				t.Fatalf("changed=%v want=%v err=%v", changed, tc.want, err)
			}
			current, err := d.CurrentPrompt(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.text
			if tc.want {
				want = "English default"
			}
			if current != want {
				t.Fatalf("current=%q want=%q", current, want)
			}
			var original string
			if err := d.QueryRow(`SELECT template_text FROM agent_prompts WHERE agent_id=$1 AND version=1`, a.ID).Scan(&original); err != nil || original != tc.text {
				t.Fatalf("original history changed: %q %v", original, err)
			}
			if changed, err := d.MigratePristineEnglishPrompt(a.ID, []string{"Original default"}, "English default", true); err != nil || changed {
				t.Fatalf("repeat migration=%v err=%v", changed, err)
			}
		})
	}
}

func TestEnglishToolMigrationPreservesSettingsAndEdits(t *testing.T) {
	d := englishDefaultsDB(t)
	before := json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer","description":"Original help"}},"required":["id"]}`)
	after := json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer","description":"English help"}},"required":["id"]}`)
	for _, tc := range []struct {
		name                                        string
		customDescription, customSchema, customTool bool
	}{
		{"pristine", false, false, false}, {"edited_description", true, false, false}, {"edited_schema", false, true, false}, {"custom_tool", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := fmt.Sprintf("english-tool-%d", time.Now().UnixNano())
			if err := d.SeedTool(key, "Original tool", before, json.RawMessage(`["chat"]`)); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM tools WHERE key=$1`, key) })
			if _, err := d.Exec(`UPDATE tools SET enabled=false WHERE key=$1`, key); err != nil {
				t.Fatal(err)
			}
			if tc.customDescription {
				_, _ = d.Exec(`UPDATE tools SET description='Custom tool prose' WHERE key=$1`, key)
			}
			if tc.customSchema {
				_, _ = d.Exec(`UPDATE tools SET schema=jsonb_set(schema,'{properties,id,description}','"Custom parameter help"') WHERE key=$1`, key)
			}
			if tc.customTool {
				_, _ = d.Exec(`UPDATE tools SET system=false WHERE key=$1`, key)
			}
			old, err := d.GetTool(key)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := d.MigratePristineEnglishTool(key, "Original tool", "English tool", before, after)
			want := !tc.customDescription && !tc.customSchema && !tc.customTool
			if err != nil || changed != want {
				t.Fatalf("changed=%v want=%v err=%v", changed, want, err)
			}
			got, err := d.GetTool(key)
			if err != nil {
				t.Fatal(err)
			}
			if got.Enabled || len(got.Agents) != 1 || got.Agents[0] != "chat" {
				t.Fatal("bindings or enablement changed")
			}
			if want {
				if got.Description != "English tool" {
					t.Fatal("English description missing")
				}
			} else if got.Description != old.Description || string(got.Schema) != string(old.Schema) {
				t.Fatal("customized data changed")
			}
			if changed, err := d.MigratePristineEnglishTool(key, "Original tool", "English tool", before, after); err != nil || changed {
				t.Fatalf("repeat migration=%v %v", changed, err)
			}
		})
	}
}

func TestEnglishPromptMigrationConcurrentUserSave(t *testing.T) {
	d := englishDefaultsDB(t)
	for i := 0; i < 5; i++ {
		a, err := d.CreateAgent(fmt.Sprintf("english_concurrent_%d", time.Now().UnixNano()), "Test agent", "Synthetic concurrency test")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM agents WHERE id=$1`, a.ID) })
		if _, err := d.Exec(`UPDATE agents SET builtin=true WHERE id=$1`, a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := d.SavePrompt(a.ID, "Original default", "Built-in default", "system"); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		errors := make(chan error, 2)
		go func() {
			<-start
			_, err := d.MigratePristineEnglishPrompt(a.ID, []string{"Original default"}, "English default", true)
			errors <- err
		}()
		go func() {
			<-start
			_, err := d.SavePrompt(a.ID, "My custom instructions", "User edit", "operator")
			errors <- err
		}()
		close(start)
		for j := 0; j < 2; j++ {
			if err := <-errors; err != nil {
				t.Fatal(err)
			}
		}
		if current, err := d.CurrentPrompt(a.ID); err != nil || current != "My custom instructions" {
			t.Fatalf("concurrent migration replaced user edit: %q, %v", current, err)
		}
	}
}
