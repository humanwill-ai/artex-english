package server

import (
	_ "embed"
	"encoding/json"
	"log"

	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/traffic"
)

// Exact source-edition defaults are compatibility data, never newly generated
// model instructions. They establish the narrow migration match.
//
//go:embed legacy_prompt_defaults.json
var legacyPromptDefaultsJSON []byte

var legacyPromptDefaults = func() map[string][]string {
	var defaults map[string][]string
	if err := json.Unmarshal(legacyPromptDefaultsJSON, &defaults); err != nil {
		panic(err)
	}
	return defaults
}()

const legacyRetesterName = "漏洞复测"
const legacyRetesterDescription = "从漏洞详情手动启动，读取原证据并保存独立复测结论。"
const retesterName = "Finding retest"
const retesterDescription = "Start manually from finding details, review the original evidence, and save a separate retest conclusion."

// migrateEnglishDefaultPrompts only upgrades unchanged first system seeds.
// The database helper also checks sole-version history and retains the old text.
// Saved user prompts, reverted/promoted versions, and custom agents are preserved.
func (s *Server) migrateEnglishDefaultPrompts() {
	defaults := agent.BuiltinPromptSeeds()
	defaults["reporter"] = agent.ReporterDefaultPrompt
	defaults[db.FindingRetestAgentKey] = agent.RetesterDefaultPrompt
	for key, english := range defaults {
		a, err := s.m.pg.GetAgentByKey(key)
		if err != nil || a == nil {
			continue
		}
		if _, err := s.migrateEnglishDefaultPrompt(a, english); err != nil {
			log.Printf("[prompts] English default migration for %s failed: %v", key, err)
		}
	}
}

// migrateEnglishDefaultPrompt adds caller-side provenance for source editions
// where reporter/retester are seeded as non-builtin assistant agents.
func (s *Server) migrateEnglishDefaultPrompt(a *db.Agent, english string) (bool, error) {
	key := a.Key
	requireBuiltin := key != "reporter" && key != db.FindingRetestAgentKey
	legacyKey := key
	if !requireBuiltin {
		if a.Builtin || a.Role != "assistant" {
			return false, nil
		}
		if key == "reporter" {
			if !((a.Name == legacyReporterName && a.Description == legacyReporterDescription) || (a.Name == reporterAgentName && a.Description == reporterAgentDescription)) {
				return false, nil
			}
		} else {
			legacyKey = "finding_retester"
			if !((a.Name == legacyRetesterName && a.Description == legacyRetesterDescription) || (a.Name == retesterName && a.Description == retesterDescription)) {
				return false, nil
			}
		}
	}
	changed, err := s.m.pg.MigratePristineEnglishPrompt(a.ID, legacyPromptDefaults[legacyKey], english, requireBuiltin)
	if err != nil {
		return false, err
	}
	if changed {
		log.Printf("[prompts] Migrated pristine %s default to English; original retained in version history", key)
	}
	if changed && key == db.FindingRetestAgentKey {
		if _, err := s.m.pg.Exec(`UPDATE agents SET name=$2,description=$3 WHERE id=$1 AND builtin=false AND role='assistant' AND name=$4 AND description=$5`, a.ID, retesterName, retesterDescription, legacyRetesterName, legacyRetesterDescription); err != nil {
			log.Printf("[retester] English default metadata migration failed: %v", err)
		}
	}
	return changed, nil
}

//go:embed legacy_tool_defaults.json
var legacyToolDefaultsJSON []byte

type englishToolDefault struct {
	Key         string          `json:"key"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

var legacyToolDefaults = func() []englishToolDefault {
	var defaults []englishToolDefault
	if err := json.Unmarshal(legacyToolDefaultsJSON, &defaults); err != nil {
		panic(err)
	}
	return defaults
}()

func (s *Server) currentEnglishToolDefaults() map[string]englishToolDefault {
	defaults := map[string]englishToolDefault{}
	for _, seed := range agent.BuiltinToolSeeds() {
		schema, _ := json.Marshal(seed.Schema)
		defaults[seed.Key] = englishToolDefault{seed.Key, seed.Desc, schema}
	}
	tools := append(s.orchestrationTools(), s.platformTools()...)
	tools = append(tools, traffic.SeedToolMetas()...)
	for _, tool := range tools {
		schema, _ := json.Marshal(tool.InputSchema())
		defaults[tool.Name()] = englishToolDefault{tool.Name(), tool.Description(), schema}
	}
	return defaults
}

// Both stored prose and schema must match the frozen source seed. Updating only
// prose keeps tool identifiers, parameters, permissions, bindings and handlers intact.
func (s *Server) migrateEnglishDefaultTools() {
	current := s.currentEnglishToolDefaults()
	for _, old := range legacyToolDefaults {
		// These three contracts have explicit, versioned upgrades below. The
		// prose-only migration must never be asked to accept schema changes.
		switch old.Key {
		case "prove_goal", "set_goals", "update_finding_report":
			continue
		}
		next, ok := current[old.Key]
		if !ok {
			continue
		}
		if _, err := s.m.pg.MigratePristineEnglishTool(old.Key, old.Description, next.Description, old.Schema, next.Schema); err != nil {
			log.Printf("[tools] English default migration for %s failed: %v", old.Key, err)
		}
	}
	s.upgradeWorkflowToolSchemas(current)
}

var priorEnglishWorkflowDescriptions = map[string]string{
	"prove_goal": "Call when you determine that a finding/fact proves a goal is met: connect the evidence node to the goal and mark the goal met.",
	"set_goals": "Add exploration goals to this task. A goal is a final deliverable/verifiable outcome, not an attack step or reconnaissance action.\n" +
		"Prefer batches: submit multiple goals in the goals array; returned ids has the same length and order (failed items have id=0; see errors). For one item, omit goals and provide top-level text.\n" +
		"vulnclass is optional: corresponding vulnerability class (such as SQLi/IDOR); leave empty for business-logic goals. The system determines completion and marks goals met; this tool only adds goals.",
	"update_finding_report": "Write/update the detailed report for an existing finding (full Markdown, replacing previous content). finding_id is the ID returned by report_finding (the number in \"finding recorded: <id>\"). Suggested sections: overview, impact, reproduction steps, evidence/PoC, and remediation.",
}

// Upgrade only known prior built-in contracts. Schema prose supplied by a user,
// bindings and enablement remain unchanged; edited descriptions are left alone.
func (s *Server) upgradeWorkflowToolSchemas(current map[string]englishToolDefault) {
	legacy := map[string]englishToolDefault{}
	for _, item := range legacyToolDefaults {
		legacy[item.Key] = item
	}
	for _, key := range []string{"prove_goal", "set_goals", "update_finding_report"} {
		row, err := s.m.pg.GetTool(key)
		if err != nil || row == nil || !row.System {
			continue
		}
		next := current[key]
		old := legacy[key]
		if row.Description != old.Description && row.Description != priorEnglishWorkflowDescriptions[key] && row.Description != next.Description {
			continue
		}
		var schema, desired map[string]any
		if json.Unmarshal(row.Schema, &schema) != nil || json.Unmarshal(next.Schema, &desired) != nil {
			continue
		}
		candidate := map[string]any{}
		if json.Unmarshal(next.Schema, &candidate) != nil {
			continue
		}
		props := objectProperty(candidate, "properties")
		field := "completion"
		if key == "prove_goal" {
			field = "outcome"
		} else if key == "update_finding_report" {
			field = "evidence_version"
		}
		delete(props, field)
		if key == "update_finding_report" {
			candidate["required"] = []any{"finding_id", "report"}
		}
		before, _ := json.Marshal(candidate)
		compatible := db.SameSchemaContract(row.Schema, before)
		if key == "update_finding_report" && !compatible {
			props[field] = objectProperty(desired, "properties")[field]
			withOptional, _ := json.Marshal(candidate)
			compatible = db.SameSchemaContract(row.Schema, withOptional)
		}
		if !compatible {
			continue
		}
		rowProps := objectProperty(schema, "properties")
		rowProps[field] = objectProperty(desired, "properties")[field]
		if key == "update_finding_report" {
			required, _ := schema["required"].([]any)
			required = append(required, field)
			schema["required"] = required
		}
		updated, _ := json.Marshal(schema)
		description := row.Description
		if row.Description == old.Description || row.Description == priorEnglishWorkflowDescriptions[key] {
			description = next.Description
		}
		if _, err := s.m.pg.Exec(`UPDATE tools SET schema=$3::jsonb,description=$4,updated_at=now()
WHERE key=$1 AND system AND kind='builtin' AND schema=$2::jsonb AND description=$5`, key, string(row.Schema), string(updated), description, row.Description); err != nil {
			log.Printf("[tools] workflow schema upgrade for %s failed: %v", key, err)
		}
	}
}
