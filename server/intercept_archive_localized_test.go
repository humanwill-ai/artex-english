package server

import (
	"net/url"
	"testing"
	"unicode"
)

// English checks apply only to application-owned text with synthetic ASCII inputs.
// User content, evidence, and model instructions are tested separately.
func assertEnglishError(t *testing.T, label, msg string) {
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

// TestInterceptArchiveErrorsLocalized guards the F3 first bundle: every
// user-facing error response produced by the pure validators in intercept.go and
// task_archives.go must be English (no Han or Hangul). If anyone
// reverts one of these literals to Chinese, this test fails.
func TestInterceptArchiveErrorsLocalized(t *testing.T) {
	// intercept.go: interceptFilterParams — query-string validation.
	if _, err := interceptFilterParams(url.Values{"status": {"bogus"}}); err == nil {
		t.Fatal("status 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "filter.status", err.Error())
	}
	if _, err := interceptFilterParams(url.Values{"decision_source": {"bogus"}}); err == nil {
		t.Fatal("decision_source 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "filter.decision_source", err.Error())
	}

	// intercept.go: validateInterceptRuleReq — each branch isolated by making the
	// earlier fields valid.
	ruleCases := []struct {
		label string
		req   interceptRuleReq
	}{
		{"rule.name", interceptRuleReq{Name: ""}},
		{"rule.match_target", interceptRuleReq{Name: "r", MatchTarget: "bad"}},
		{"rule.match_type", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "bad"}},
		{"rule.pattern_empty", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "string", Pattern: ""}},
		{"rule.action", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "string", Pattern: "p", Action: "bad"}},
		{"rule.regex", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "regex", Pattern: "(", Action: "allow"}},
	}
	for _, tc := range ruleCases {
		err := validateInterceptRuleReq(tc.req)
		if err == nil {
			t.Fatalf("%s: 검증이 통과해서는 안 됩니다", tc.label)
		}
		assertEnglishError(t, tc.label, err.Error())
	}

	// task_archives.go: normalizeArchiveIDs — batch id validation.
	if _, err := normalizeArchiveIDs(nil); err == nil {
		t.Fatal("빈 archive_ids 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "archive_ids.empty", err.Error())
	}
	tooMany := make([]int64, 101)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	if _, err := normalizeArchiveIDs(tooMany); err == nil {
		t.Fatal("101개 archive_ids 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "archive_ids.too_many", err.Error())
	}
	if _, err := normalizeArchiveIDs([]int64{0}); err == nil {
		t.Fatal("0 archive id 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "archive_ids.nonpositive", err.Error())
	}

	// task_archives.go: validateArchivePath — package path validation.
	if err := validateArchivePath("/tmp/artex-data", ""); err == nil {
		t.Fatal("빈 보관 경로 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "archive_path.empty", err.Error())
	}
	if err := validateArchivePath("/tmp/artex-data", "/etc/passwd"); err == nil {
		t.Fatal("관리 디렉터리 밖 경로 검증이 통과해서는 안 됩니다")
	} else {
		assertEnglishError(t, "archive_path.outside", err.Error())
	}
}
