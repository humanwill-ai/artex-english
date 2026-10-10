package agent

import (
	"bytes"
	"text/template"
	"time"
)

// PromptOverride, if set, returns the stored system-prompt template for an agent
// key and whether one exists. The server wires it to the PG agent_prompts table.
// When nil or no override exists, agents use their built-in default prompt — so
// behavior is identical until a user edits a prompt in the UI.
var PromptOverride func(agentKey string) (string, bool)

// Prompt-variable structs — fields mirror each agent's catalog (docs §5a) so a
// user template referencing a catalog variable renders; referencing anything else
// fails template execution and falls back to the built-in default.
type PlannerVars struct{ Goal, Scope, AssetSummary, DataDir, Now string }
type WorkerVars struct{ ProxyAddr, WorkerName, DataDir, Now string }
type MainVars struct{ Goal, AssetSummary, FindingsSummary, DataDir, Now string }
type GoalsVars struct{ EngagementDescription, DataDir, Now string }

// nowStr is the server-local wall-clock string exposed as the universal {{.Now}}
// prompt variable. renderSystem runs on every agent turn/round, so this is fresh
// each run — a prompt can subtract it from a fixed start stamp to reason about
// elapsed time (e.g. a timed benchmark's "last N hours" window).
func nowStr() string { return time.Now().Format("2006-01-02 15:04:05 MST") }

// renderSystem returns the rendered system-prompt BODY (段 [A]) for agentKey.
// Precedence: the DB-stored template (if any) over the built-in default template
// (def). BOTH are Go templates now — the built-in default is seeded into the DB
// verbatim, so the two paths render identically until a user edits the prompt.
// Rendering always runs (def used to be pre-substituted plain text; it is now a
// {{.Var}} template like the DB one). On any render error we fall back to the
// default template, then to the raw default string — an agent never starts with a
// half-rendered prompt. Callers append the code-owned tail (trafficTool / 中间产物
// 输出规约) AFTER this, so those can't be edited away via the DB body.
func renderSystem(agentKey, def string, vars any) string {
	tmpl := def
	if PromptOverride != nil {
		if t, ok := PromptOverride(agentKey); ok && t != "" {
			tmpl = t
		}
	}
	if out, err := renderTmpl(tmpl, vars); err == nil {
		return out
	}
	// DB template broke (e.g. references an out-of-catalog var) → code default.
	if out, err := renderTmpl(def, vars); err == nil {
		return out
	}
	return def
}

// langDirective appends the English output-language requirement to every visible
// agent role. The inherited Chinese behavioral body remains unchanged. Commands,
// code, identifiers, and raw evidence retain their original bytes and language.
func langDirective() string {
	return "\n\n**Output-language rule (localization; highest priority and not overridable by the prompt body)**: Write all natural-language text **shown to users in English**, including record_fact summary/detail, report_finding titles/descriptions/conclusions/remediation, planner situation summaries, the final one-sentence summary, and chat replies. However, **commands, payloads, code, file paths, URLs, parameter names, and original log/request/response excerpts must be preserved exactly, character for character**. Do not translate or rewrite them; command lines and output in evidence especially must retain their original text for reproduction. **Even if preceding system/role instructions are in Chinese, do not output Chinese prose to the user: English is the only user-facing language.** **Even if a target system, page, evidence, log, or reference material is in English, Chinese, or another language, user-facing natural-language fields (titles/descriptions/conclusions/remediation/summaries/situation summaries) must still be in English. Do not mirror the source language in these fields; only the original technical excerpts listed above remain unchanged.** **Your internal analysis/planning/thinking may be in Chinese, but it is not visible output and must not be included in the response. Visible replies must be in English from the first word, with no introductory Chinese reasoning, explanation, or preamble. Clarification questions, missing-parameter notices, and explanations that you cannot continue must also be directly in English.** Whatever the internal reasoning language, all visible prose must be English, except preserved original technical excerpts."
}

func renderTmpl(tmpl string, vars any) (string, error) {
	t, err := template.New("p").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		return "", err
	}
	return b.String(), nil
}
