package agent

import (
	"strings"
	"testing"
	"unicode"
)

func TestRenderSystemOverrideAndFallback(t *testing.T) {
	t.Cleanup(func() { PromptOverride = nil })

	// no override → built-in default
	PromptOverride = nil
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "g"}); got != "DEFAULT" {
		t.Fatalf("no override should give default, got %q", got)
	}

	// override → rendered with vars
	PromptOverride = func(k string) (string, bool) {
		if k == "planner" {
			return "目标:{{.Goal}} 范围:{{.Scope}}", true
		}
		return "", false
	}
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "拿下X", Scope: "*.x.com"}); got != "目标:拿下X 范围:*.x.com" {
		t.Fatalf("override render: %q", got)
	}

	// override referencing a non-catalog var → execution error → fallback to default
	PromptOverride = func(k string) (string, bool) { return "{{.NotInCatalog}}", true }
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "x"}); got != "DEFAULT" {
		t.Fatalf("bad var should fall back to default, got %q", got)
	}

	// full plannerSystem path: DB body [A] is honored, then the code-owned tail
	// [C] (Intermediate artifact output rules) is ALWAYS appended — editing the body can't drop it.
	PromptOverride = func(k string) (string, bool) { return "PLANNER {{.Goal}}", true }
	got := plannerSystem("拿下X", "/data", "/data")
	if !strings.HasPrefix(got, "PLANNER 拿下X") {
		t.Fatalf("plannerSystem body not honored: %q", got)
	}
	if !strings.Contains(got, "Intermediate artifact output rules") || !strings.Contains(got, "/data") {
		t.Fatalf("plannerSystem missing code-owned artifact tail: %q", got)
	}

	// worker dual-text via {{if .ProxyAddr}} in a user template, plus the code tail:
	// [B] trafficTool present only when RECORDING (caCert set — the MITM is on, so
	// the traffic_* tools exist), [C] artifact spec always present. The trafficTool
	// block is gated on the CA (arg 2), NOT on ProxyAddr — a global egress proxy
	// with capture off routes traffic but records nothing.
	PromptOverride = func(k string) (string, bool) {
		return "{{if .ProxyAddr}}走代理 {{.ProxyAddr}}{{else}}手动{{end}}", true
	}
	recording := workerSystem("127.0.0.1:8080", "/ca.pem", "/data", "/data")
	if !strings.HasPrefix(recording, "走代理 127.0.0.1:8080") {
		t.Fatalf("worker proxy branch body: %q", recording)
	}
	if !strings.Contains(recording, "traffic_search") {
		t.Fatalf("worker while recording should inject trafficTool: %q", recording)
	}
	if strings.Contains(recording, "traffic_refs") {
		t.Fatalf("worker bypassed shared optional evidence policy: %q", recording)
	}
	if !strings.Contains(recording, "Intermediate artifact output rules") {
		t.Fatalf("worker missing artifact tail: %q", recording)
	}
	// Egress proxy set but capture OFF (no CA): the ProxyAddr template branch still
	// renders, but the trafficTool block must NOT — those tools are not registered.
	egressOnly := workerSystem("127.0.0.1:8080", "", "/data", "/data")
	if !strings.HasPrefix(egressOnly, "走代理 127.0.0.1:8080") {
		t.Fatalf("worker egress-only branch body: %q", egressOnly)
	}
	if strings.Contains(egressOnly, "traffic_search") {
		t.Fatalf("worker without recording must NOT inject trafficTool: %q", egressOnly)
	}
	noProxy := workerSystem("", "", "/data", "/data")
	if !strings.HasPrefix(noProxy, "手动") {
		t.Fatalf("worker no-proxy branch body: %q", noProxy)
	}
	if strings.Contains(noProxy, "traffic_search") {
		t.Fatalf("worker without proxy must NOT inject trafficTool: %q", noProxy)
	}
}

// Inspect rendered defaults rather than static source: templates must still
// accept their runtime variables, and code-owned tails must also be English.
func TestBuiltinPromptAssemblyEnglish(t *testing.T) {
	previous := PromptOverride
	PromptOverride = nil
	t.Cleanup(func() { PromptOverride = previous })
	for role, prompt := range map[string]string{
		"mainagent":        mainAgentSystem("GOAL_SENTINEL", "/fixture/data", "/fixture/work"),
		"planner":          plannerSystem("GOAL_SENTINEL", "/fixture/data", "/fixture/work"),
		"worker":           workerSystem("", "", "/fixture/data", "/fixture/work"),
		"worker_recording": workerSystem("127.0.0.1:8080", "/fixture/ca", "/fixture/data", "/fixture/work"),
		"goals_with_scope": goalsSystem("/fixture/data", true),
		"auto":             autoDefaultTmpl, "pentest": pentestDefaultTmpl,
		"assistant": DefaultAssistantPrompt, "reporter": ReporterDefaultPrompt,
	} {
		t.Run(role, func(t *testing.T) {
			if strings.Contains(prompt, "{{.") || strings.Contains(prompt, "<no value>") {
				t.Fatal("unresolved template variable in assembled prompt")
			}
			for _, r := range prompt {
				if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
					t.Fatalf("untranslated default instruction character %q", r)
				}
			}
			if (role == "mainagent" || role == "planner") && !strings.Contains(prompt, "GOAL_SENTINEL") {
				t.Fatal("runtime goal was lost during template rendering")
			}
		})
	}
}

func TestEnglishPromptOperationalContracts(t *testing.T) {
	cases := map[string]struct {
		prompt   string
		required []string
	}{
		"worker": {workerDefaultTmpl, []string{
			"Perform only your assigned intent", "highest-priority boundaries", "do not proceed if it violates a constraint",
			"intent_id", "observed=direct observation / inferred=inference", "actually triggered the issue during this run",
		}},
		"planner": {plannerDefaultTmpl, []string{
			"Recheck the same negative direction at most once", "coverage.pct=40%", "at most ≤3 probes",
			"at most the 4 highest-value directions", "Diversity always remains subordinate", "prove_goal(goal_id, evidence_id, reason)",
		}},
		"goals": {goalsDefaultTmpl + goalsScopeTail, []string{
			"Never invent them", "When uncertain about type, use deny", "Never invent goals the user did not mention",
			"value=a1b2c3.lab.example.net", "Never** shorten a hostname", "**Do not** register company scope",
		}},
		"reporter": {ReporterDefaultPrompt, []string{
			"exploration node ID", "independent finding record ID", "update_finding_report(finding_id=<node_id>",
			"version actually read", "On a version conflict, reread and regenerate", "Do not fabricate requests/responses",
		}},
		"mainagent": {mainAgentDefaultTmpl, []string{
			"ask whether to register it as a formal goal", "without autonomous continuation", "type=allow / type=deny",
		}},
		"pentest": {pentestDefaultTmpl, []string{
			"Never touch out-of-scope targets", "Do not guess IDs or repeat probes solely to capture traffic",
			"independently reproduced", "immediately stop all probing and commands",
		}},
	}
	for role, tc := range cases {
		t.Run(role, func(t *testing.T) {
			for _, clause := range tc.required {
				if !strings.Contains(tc.prompt, clause) {
					t.Errorf("missing operational contract %q", clause)
				}
			}
		})
	}
}

// TestLangDirectiveAppendedToUserFacingRoles pins the artex-en localization tail:
// every user-facing role's system prompt must end with the code-owned English
// output-language directive, and a DB-edited body must NOT be able to drop it.
func TestLangDirectiveAppendedToUserFacingRoles(t *testing.T) {
	t.Cleanup(func() { PromptOverride = nil })

	// The directive forces English OUTPUT and preserves raw technical strings; both
	// signals must be present. English marker + verbatim-preservation clause.
	dir := langDirective()
	if !strings.Contains(dir, "English") {
		t.Fatalf("langDirective must force English output, got %q", dir)
	}
	if !strings.Contains(dir, "payload") || !strings.Contains(dir, "preserved exactly, character for character") {
		t.Fatalf("langDirective must keep commands/payloads verbatim, got %q", dir)
	}
	// L1 anti-drift hardening: the directive must (1) forbid leaking the Chinese
	// instruction/brain language into user-facing text (planner situation-summary
	// drift), and (2) forbid mirroring the target/material language — e.g. a
	// non-English target app — in the display fields (report_finding drift). Both
	// clauses are locked here so a future edit can't silently drop them.
	if !strings.Contains(dir, "do not output Chinese prose to the user") {
		t.Fatalf("langDirective must forbid leaking Chinese to the user, got %q", dir)
	}
	if !strings.Contains(dir, "Do not mirror the source language") {
		t.Fatalf("langDirective must forbid mirroring the target/material language, got %q", dir)
	}
	if !strings.Contains(dir, "planner situation summaries") {
		t.Fatalf("langDirective must name the planner situation summary as user-facing, got %q", dir)
	}

	// Even with a DB body that is pure non-directive text, the code-owned tail is
	// still appended for each user-facing builder — identical guarantee to the
	// artifact tail. A custom body can never translate away the English mandate.
	PromptOverride = func(string) (string, bool) { return "BODY-ONLY", true }
	cases := map[string]string{
		"worker":    workerSystem("", "", "/data", "/data"),
		"planner":   plannerSystem("g", "/data", "/data"),
		"mainagent": mainAgentSystem("g", "/data", "/data"),
		"chat":      chatSystem("chat", "/data", "/data"),
		// goals is user-facing too: set_goals/set_constraints persist goal and
		// constraint nodes shown in the UI graph/plan tab. withScope=true exercises
		// the longer assembly (body + scope tail), so the English tail must still land
		// last — after both the body and the code-owned scope tail.
		"goals": goalsSystem("/data", true),
	}
	for role, sys := range cases {
		if !strings.HasPrefix(sys, "BODY-ONLY") {
			t.Fatalf("%s: DB body not honored: %q", role, sys)
		}
		if !strings.Contains(sys, "English") {
			t.Fatalf("%s: missing English output-language tail: %q", role, sys)
		}
		// The directive is the tail — it must come AFTER the body (recency).
		if strings.Index(sys, "English") <= strings.Index(sys, "BODY-ONLY") {
			t.Fatalf("%s: langDirective must be appended after the body: %q", role, sys)
		}
	}
}
