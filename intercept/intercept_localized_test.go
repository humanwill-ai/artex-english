package intercept

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
)

// hasHangul reports whether s contains any Hangul syllable.
func hasHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

// hasHan reports whether s contains any CJK Han ideograph.
func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// Default messages are English; user-supplied reasons remain unchanged.
func assertEnglish(t *testing.T, label, s string) {
	t.Helper()
	if s == "" || hasHangul(s) || hasHan(s) {
		t.Errorf("%s: expected English application text: %q", label, s)
	}
}

// TestInterceptMessagesLocalized checks English messages shared with the agent
// and stored in approval records. English display projections are tested in
// server/intercept_display_test.go; the HTTP 409 error is tested separately below.
func TestInterceptMessagesLocalized(t *testing.T) {
	for _, c := range []struct{ name, s string }{
		{"msgReviewContextIncomplete", msgReviewContextIncomplete},
		{"msgModelApprovalFailed", msgModelApprovalFailed},
		{"msgModelOutputUnparsable", msgModelOutputUnparsable},
		{"reasonWorkCanceled", reasonWorkCanceled},
		{"reasonWorkCanceledPreExec", reasonWorkCanceledPreExec},
		{"reasonApprovalTimeout", reasonApprovalTimeout},
		{"reasonManualDeny", reasonManualDeny},
		{"reasonManualAllow", reasonManualAllow},
	} {
		assertEnglish(t, c.name, c.s)
	}
}

// TestJudgeActionLabelLocalized checks English verdict labels used in new agent context. Unknown actions still
// pass through untranslated; English display labels are tested separately.
func TestJudgeActionLabelLocalized(t *testing.T) {
	for _, action := range []string{"allow", "deny", "ask"} {
		assertEnglish(t, "judgeActionLabel("+action+")", judgeActionLabel(action))
	}
	if got := judgeActionLabel("weird"); got != "weird" {
		t.Errorf("judgeActionLabel(weird) = %q, want passthrough", got)
	}
}

// TestDefaultMessageLocalized checks English rule-derived deny/ask messages
// shared with the agent, including the rule name; allow stays empty.
func TestDefaultMessageLocalized(t *testing.T) {
	for _, action := range []string{"deny", "ask"} {
		msg := defaultMessage(action, "R1")
		assertEnglish(t, "defaultMessage("+action+")", msg)
		if !strings.Contains(msg, "R1") {
			t.Errorf("defaultMessage(%s) dropped rule name: %q", action, msg)
		}
	}
	if got := defaultMessage("allow", "R1"); got != "" {
		t.Errorf("defaultMessage(allow) = %q, want empty", got)
	}
}

// TestToolApprovalSummaryLocalized checks that the English activity summary keeps
// its tool name and pending ID. The frontend approval-summary helper also parses
// historical Korean and Chinese summaries without rewriting stored records.
func TestToolApprovalSummaryLocalized(t *testing.T) {
	s := fmt.Sprintf(msgToolApprovalRequestFmt, "Bash", 42)
	if s != "Approval requested for tool Bash (#42)" {
		t.Errorf("Unexpected English summary: %q", s)
	}
	if !strings.Contains(s, "(#42)") {
		t.Errorf("summary lost the (#N) marker (transcript.tsx pending_id regex): %q", s)
	}
	if !strings.Contains(s, "tool Bash") {
		t.Errorf("summary lost the '도구 X 승인' shape (transcript.tsx toolName regex): %q", s)
	}
}

func TestAlreadyDecidedEnglish(t *testing.T) {
	if ErrAlreadyDecided.Error() != "This approval has already been handled or does not exist. Refresh the records" {
		t.Fatal(ErrAlreadyDecided)
	}
}
