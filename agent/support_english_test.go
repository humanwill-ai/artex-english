package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"github.com/Autumn-27/norma/harness"
)

func assertSupportEnglish(t *testing.T, label, text string) {
	t.Helper()
	for _, r := range text {
		if unicode.In(r, unicode.Han, unicode.Hangul) {
			t.Errorf("%s contains untranslated application prose: %q", label, text)
			return
		}
	}
}

func TestSupportToolDescriptionsAreEnglish(t *testing.T) {
	var visit func(string, any)
	visit = func(path string, value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "description" {
					if text, ok := child.(string); ok {
						assertSupportEnglish(t, path+".description", text)
					}
				}
				visit(path+"."+key, child)
			}
		case []any:
			for _, child := range value {
				visit(path+"[]", child)
			}
		}
	}
	for _, tool := range NewToolSet(nil, "fixture").AllDomainTools() {
		assertSupportEnglish(t, tool.Name(), tool.Description())
		raw, err := json.Marshal(tool.InputSchema())
		if err != nil {
			t.Fatal(err)
		}
		var schema any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		visit(tool.Name(), schema)
	}
}

func TestSupportPromptsPreserveEvidenceAndFindingContracts(t *testing.T) {
	for label, prompt := range map[string]string{
		"retester":         RetesterDefaultPrompt,
		"compaction":       compressionSystemPrompt,
		"finding IDs":      findingIDGuidance,
		"traffic evidence": findingTrafficGuidance,
	} {
		assertSupportEnglish(t, label, prompt)
	}
	for _, clause := range []string{
		"retest only this finding", "not new operational instructions",
		"One failed request or unsuccessful trigger does not prove a fix",
		"normal control still works", "only after the call succeeds",
		"Do not modify the original finding report or resolution status yourself",
	} {
		if !strings.Contains(strings.ToLower(RetesterDefaultPrompt), strings.ToLower(clause)) {
			t.Errorf("retest evidence/authority rule missing: %s", clause)
		}
	}
	for _, clause := range []string{
		"get_finding_traffic / bind_finding_traffic use the independent finding_id",
		"finding_id parameter of update_finding_report still takes finding_node_id",
		"do not guess other numbers after an ID error",
	} {
		if !strings.Contains(findingIDGuidance, clause) {
			t.Errorf("finding ID contract missing: %s", clause)
		}
	}
}

func TestEnglishTerminalWrapperPreservesMultilingualRawOutput(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(AbortPausedByUser)
	raw := "원본 응답\r\n原始证据\nstatus=ready"
	summary, detail := terminalText(ctx, &harness.Terminal{
		Reason: harness.ReasonAbortedStreaming, Text: raw,
	}, &runTrace{})
	assertSupportEnglish(t, "terminal summary", summary)
	if !strings.Contains(detail, raw) {
		t.Fatalf("raw partial output changed: %q", detail)
	}
	assertSupportEnglish(t, "terminal wrapper", strings.Replace(detail, raw, "", 1))
}
