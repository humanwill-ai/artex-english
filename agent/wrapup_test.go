package agent

import (
	"strings"
	"testing"
	"unicode"
)

// Empty persisted wrap-up fields resolve to these built-in instructions. Check
// English prose, tool identifiers, and the distinct per-run/task-timeout rules.
func TestWrapupInstructionBodiesPreserved(t *testing.T) {
	all := map[string]string{
		"settleWrapUpPrompt":        settleWrapUpPrompt,
		"plannerWrapUpDefault":      plannerWrapUpDefault,
		"mainAgentWrapUpDefault":    mainAgentWrapUpDefault,
		"genericWrapUpDefault":      genericWrapUpDefault,
		"workerTaskTimeoutDefault":  workerTaskTimeoutDefault,
		"plannerTaskTimeoutDefault": plannerTaskTimeoutDefault,
	}
	for name, prompt := range all {
		for _, r := range prompt {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
				t.Errorf("%s: untranslated instruction character %q", name, r)
				break
			}
		}
	}
	mustContain := func(name, prompt string, required ...string) {
		t.Helper()
		for _, part := range required {
			if !strings.Contains(prompt, part) {
				t.Errorf("%s: missing required instruction %q", name, part)
			}
		}
	}
	mustContain("worker", settleWrapUpPrompt, "insert_assets", "record_fact", "report_finding", "exactly one sentence of plain text", "Do not execute any further commands or probes")
	mustContain("worker task timeout", workerTaskTimeoutDefault, "insert_assets", "record_fact", "report_finding", "exactly one sentence of plain text", "entire task", "Do not start any new commands or probes")
	mustContain("generic", genericWrapUpDefault, "exactly one sentence of plain text", "record completed results")
	mustContain("main agent", mainAgentWrapUpDefault, "exactly one sentence of plain text", "Do not start any new exploration or operations")
	mustContain("planner", plannerWrapUpDefault, "add_intent", "prove_goal", "TodoWrite", "Only **this round** is ending", "do not invent intents merely to wrap up", "without summary text")
	mustContain("planner task timeout", plannerTaskTimeoutDefault, "prove_goal", "entire task", "Do not generate any new intents", "without summary text")
}

func TestWrapupDefaultsRouting(t *testing.T) {
	for role, expected := range map[string]string{
		"worker": settleWrapUpPrompt, "planner": plannerWrapUpDefault,
		"mainagent": mainAgentWrapUpDefault, "unknown-agent": genericWrapUpDefault,
	} {
		if got := WrapupDefault(role); got != expected {
			t.Errorf("%s: incorrect per-run wrap-up default", role)
		}
	}
	if TaskTimeoutWrapupDefault("worker") != workerTaskTimeoutDefault {
		t.Error("worker task-timeout default changed")
	}
	if TaskTimeoutWrapupDefault("planner") != plannerTaskTimeoutDefault {
		t.Error("planner task-timeout default changed")
	}
	if TaskTimeoutWrapupDefault("mainagent") != "" {
		t.Error("mainagent must have no task-timeout prompt")
	}
	if workerTaskTimeoutDefault == settleWrapUpPrompt || plannerTaskTimeoutDefault == plannerWrapUpDefault {
		t.Error("per-run and task-timeout instructions must remain distinct")
	}
}

func TestWrapupSummariesRequestEnglish(t *testing.T) {
	for name, prompt := range map[string]string{
		"worker": settleWrapUpPrompt, "mainagent": mainAgentWrapUpDefault,
		"generic": genericWrapUpDefault, "worker_timeout": workerTaskTimeoutDefault,
	} {
		if !strings.Contains(prompt, "in English") {
			t.Errorf("%s must request an English summary", name)
		}
	}
}
