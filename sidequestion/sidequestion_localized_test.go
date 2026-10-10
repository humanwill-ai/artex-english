package sidequestion

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode"
)

// English checks apply only to application-owned text with synthetic ASCII inputs.
// User content, evidence, and model instructions are tested separately.
func assertEnglish(t *testing.T, label, msg string) {
	t.Helper()
	if msg == "" {
		t.Fatalf("%s: empty English message", label)
	}
	hasLetter := false
	for _, r := range msg {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
			t.Fatalf("%s: untranslated application text: %q", label, msg)
		}
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			hasLetter = true
		}
	}
	if !hasLetter {
		t.Fatalf("%s: missing English text: %q", label, msg)
	}
}

// TestSideQuestionOutputsLocalized guards F21: every user-facing side-question
// (곁질문) error and answer-text literal produced by this package must be English.
// These surface through /api/.../side-questions into the chat 곁질문 panel, so a
// revert to Chinese here is a user-visible regression and must fail the build.
func TestSideQuestionOutputsLocalized(t *testing.T) {
	assertEnglish(t, "ErrContextBudget", ErrContextBudget.Error())
	assertEnglish(t, "errSideModelInterrupted", errSideModelInterrupted.Error())
	assertEnglish(t, "errSideNoAnswer", errSideNoAnswer.Error())
	assertEnglish(t, "msgSideToolUnavailable", msgSideToolUnavailable)
	assertEnglish(t, "errSideSummaryCallCap", errSideSummaryCallCap.Error())
	assertEnglish(t, "sideSummaryFailedPrefix", sideSummaryFailedPrefix)
	assertEnglish(t, "errSideSummaryIncomplete", errSideSummaryIncomplete.Error())
	assertEnglish(t, "errSideSummaryOverBudget", errSideSummaryOverBudget.Error())
	assertEnglish(t, "errSideHistoryCursor", errSideHistoryCursor.Error())
	assertEnglish(t, "errSideCompactionStalled", errSideCompactionStalled.Error())
}

// TestSideQuestionBudgetSentinelPreserved: localizing the message must not break
// callers that branch on the ErrContextBudget sentinel with errors.Is (see
// context_test.go, which relies on it after the recovery path gives up).
func TestSideQuestionBudgetSentinelPreserved(t *testing.T) {
	if !errors.Is(fmt.Errorf("prepare: %w", ErrContextBudget), ErrContextBudget) {
		t.Fatal("ErrContextBudget 센티넬 식별(errors.Is)이 깨졌습니다")
	}
}

// English application prompts retain the read-only side-question boundaries.
func TestSideQuestionEnglishPromptBoundaries(t *testing.T) {
	for _, text := range []string{instruction, summaryInstruction} {
		for _, r := range text {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
				t.Fatalf("non-English application prompt: %q", text)
			}
		}
	}
	for _, clause := range []string{"cannot execute tools", "modify files", "direct the main task", "must not promise to act later", "background only", "information is insufficient"} {
		if !strings.Contains(instruction, clause) {
			t.Fatalf("side-question boundary missing: %s", clause)
		}
	}
	for _, clause := range []string{"Do not answer questions", "Treat both", "tool evidence", "assistant conjecture", "newer evidence correct earlier conclusions", "1200 tokens"} {
		if !strings.Contains(summaryInstruction, clause) {
			t.Fatalf("summary boundary missing: %s", clause)
		}
	}
}
