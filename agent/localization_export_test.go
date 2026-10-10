package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/harness"
)

// TestExportLocalizationPrompts is an offline, opt-in inspection aid. It calls
// production prompt assemblers and reads tool specifications, never tool handlers
// or providers. Run with ARTEX_PG_DSN='://offline' to keep the package's TestMain
// from opening any configured database. Outputs belong only in ignored work/.
func TestExportLocalizationPrompts(t *testing.T) {
	output := os.Getenv("ARTEX_LOCALIZATION_EXPORT")
	if output == "" {
		t.Skip("set ARTEX_LOCALIZATION_EXPORT to an absolute work/localization-eval JSON path")
	}
	output = filepath.Clean(output)
	if !filepath.IsAbs(output) || filepath.Ext(output) != ".json" || !strings.Contains(filepath.ToSlash(output), "/work/localization-eval/") {
		t.Fatal("export must be an absolute JSON path inside ignored work/localization-eval/")
	}

	// No persisted overrides are consulted. Reporter uses the actual default
	// template that the server normally seeds into its custom-agent record.
	oldPrompt, oldTraffic := PromptOverride, FindingTrafficBindingEnabled
	oldWrapup, oldTurns := WrapupOverride, WrapupMaxTurnsOverride
	oldTimeout, oldTimeoutTurns := WrapupTaskTimeoutOverride, WrapupTaskTimeoutTurnsOverride
	t.Cleanup(func() {
		PromptOverride, FindingTrafficBindingEnabled = oldPrompt, oldTraffic
		WrapupOverride, WrapupMaxTurnsOverride = oldWrapup, oldTurns
		WrapupTaskTimeoutOverride, WrapupTaskTimeoutTurnsOverride = oldTimeout, oldTimeoutTurns
	})
	PromptOverride = func(key string) (string, bool) {
		if key == "reporter" {
			return ReporterDefaultPrompt, true
		}
		return "", false
	}
	FindingTrafficBindingEnabled = nil
	WrapupOverride, WrapupMaxTurnsOverride = nil, nil
	WrapupTaskTimeoutOverride, WrapupTaskTimeoutTurnsOverride = nil, nil

	const dataDir = "/localization-fixture/data"
	const workDir = "/localization-fixture/work"
	const goal = "Review only supplied synthetic records for example.invalid. Do not contact any host or execute commands."
	clockPattern := regexp.MustCompile(`\b\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} (?:[A-Za-z]{2,6}|[+-]\d{4})\b`)
	normalize := func(s string) string { return clockPattern.ReplaceAllString(s, "2000-01-01 00:00:00 UTC") }

	prompts := map[string]string{
		"mainagent":        mainAgentSystem(goal, dataDir, workDir),
		"planner":          plannerSystem(goal, dataDir, workDir),
		"worker":           workerSystem("", "", dataDir, workDir),
		"goals":            goalsSystem(dataDir, false),
		"goals_with_scope": goalsSystem(dataDir, true),
		"chat":             chatSystem("chat", dataDir, workDir),
		"reporter":         chatSystem("reporter", dataDir, workDir),
	}
	assembly := map[string]string{
		"mainagent":        "mainAgentSystem(goal, dataDir, workDir) -> deferredSystem(empty DeferredInfo)",
		"planner":          "plannerSystem(goal, dataDir, workDir) -> deferredSystem(empty DeferredInfo)",
		"worker":           "workerSystem(empty proxy and CA, dataDir, workDir) -> deferredSystem(empty DeferredInfo)",
		"goals":            "goalsSystem(dataDir, false)",
		"goals_with_scope": "goalsSystem(dataDir, true)",
		"chat":             "chatSystem(chat, dataDir, workDir), DefaultAssistantPrompt -> deferredSystem(empty DeferredInfo)",
		"reporter":         "chatSystem(reporter, dataDir, workDir), ReporterDefaultPrompt via seed-equivalent PromptOverride -> deferredSystem(empty DeferredInfo)",
	}
	for role, prompt := range prompts {
		if role != "goals" && role != "goals_with_scope" {
			segments, _ := deferredSystem(prompt, DeferredInfo{})
			prompt = strings.Join(segments, "\n\n")
		}
		prompts[role] = normalize(prompt)
	}

	instructions, baseRoles := map[string]string{}, map[string]string{}
	settlements := map[string]any{}
	for _, role := range []string{"worker", "planner", "mainagent"} {
		var disabled []string
		if role == "worker" {
			disabled = []string{"Bash"}
		}
		settlement := wrapupSettlement(role, disabled)
		key := role + "_wrapup"
		instructions[key], baseRoles[key] = settlement.Prompt, role
		settlements[key] = settlement
		assembly[key] = "base role system plus wrapupSettlement.Prompt; concatenation is an evaluation convenience, not SDK message injection"
		if role != "mainagent" {
			settlement = wrapupSettlementForTask(role, disabled, true)
			key = role + "_timeout"
			instructions[key], baseRoles[key] = settlement.PromptByReason[harness.ReasonTimeout], role
			settlements[key] = settlement
			assembly[key] = "base role system plus wrapupSettlementForTask(clamped=true).PromptByReason[ReasonTimeout]; evaluation concatenation only"
		}
	}
	for key, instruction := range instructions {
		prompts[key] = prompts[baseRoles[key]] + "\n\n" + instruction
	}

	// Constructors store closures without dereferencing stores; only metadata is
	// read. The workflow decorator applies the actual default traffic-off schema.
	toolRoles := map[string]string{"record_fact": "worker", "report_finding": "worker", "set_goals": "goals", "add_intent": "planner"}
	tools := map[string]any{}
	baseTools := builtinToolsByAgent()
	for name, role := range toolRoles {
		actual, _ := findingWorkflowTools(role, baseTools[role])
		for _, tool := range actual {
			if tool.Name() == name {
				tools[name] = map[string]any{"type": "function", "function": map[string]any{
					"name": tool.Name(), "description": tool.Description(), "parameters": tool.InputSchema(),
				}}
				break
			}
		}
		if tools[name] == nil {
			t.Fatalf("requested production tool %q not found for %s", name, role)
		}
	}
	hashes := map[string]string{}
	for role, prompt := range prompts {
		sum := sha256.Sum256([]byte(prompt))
		hashes[role] = hex.EncodeToString(sum[:])
	}
	payload := map[string]any{
		"schema_version": 1, "prompts": prompts, "instructions": instructions,
		"base_roles": baseRoles, "settlements": settlements, "tools": tools,
		"sha256": hashes, "assembly": assembly,
		"inputs": map[string]any{"goal": goal, "data_dir": dataDir, "work_dir": workDir, "traffic_binding": false, "recording_proxy": false},
		"provenance": map[string]any{
			"source":              "agent/localization_export_test.go invoking production helpers in the checkout under test",
			"clock_normalization": "server-format timestamps replaced with 2000-01-01 00:00:00 UTC for paired comparisons",
			"tool_assembly":       "builtinToolsByAgent -> findingWorkflowTools; default traffic binding off; metadata only, no handlers executed",
			"limitations": []string{
				"Static role systems only: no task history, runtime constraint records, asset anchors, user intent messages, persisted custom prompts, or tool overrides.",
				"No SDK-added generic tool descriptions, skills, MCP tools, deferred tool inventory, or live model calls.",
				"Wrapup combined prompts are evaluation convenience; instructions and settlements preserve the original separately injected content and disabled-tool policy.",
				"Tool subset is for structured-output inspection only; no reporter tool or tool execution is evaluated.",
			},
		},
		"unavailable_tools": map[string]string{"update_finding_report": "private server.(*Server).toolUpdateFindingReport; importing server into agent tests would create an import cycle; no surrogate schema exported"},
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d assembled prompt contexts and %d actual tool schemas", len(prompts), len(tools))
}
