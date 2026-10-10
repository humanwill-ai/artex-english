package report

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
)

// English templates must never translate user-authored content or raw evidence.
func TestEnglishExportsPreserveMultilingualContent(t *testing.T) {
	f := &db.DBFinding{ID: 7, Name: "한국어 标题", VulnClass: "CustomType", Severity: "high", Status: "confirmed",
		Summary: "요약 中文 summary", Evidence: "GET /증거?q=证据 HTTP/1.1\r\nX-Original: 그대로\r\n\r\n",
		Report: "## 사용자 보고서\n保留原文", TaskDescription: "사용자 task", CreatedAt: time.Unix(0, 0)}
	for label, out := range map[string]string{
		"summary": FindingsMarkdown([]*db.DBFinding{f}, time.Unix(0, 0)),
		"single":  SingleFindingMarkdown(f, time.Unix(0, 0)),
	} {
		for _, original := range []string{f.Name, f.Summary, strings.TrimSpace(f.Evidence), f.Report} {
			if !strings.Contains(out, original) {
				t.Errorf("%s export changed original content %q", label, original)
			}
		}
		if !strings.Contains(out, "report") {
			t.Errorf("%s is missing English report labels", label)
		}
	}
	raw := strings.TrimPrefix(string(FindingsCSV([]*db.DBFinding{f})), "\xEF\xBB\xBF")
	rows, err := csv.NewReader(strings.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("CSV row count: %d", len(rows))
	}
	columns := map[string]string{}
	for i, name := range rows[0] {
		columns[name] = rows[1][i]
	}
	for name, want := range map[string]string{"Name": f.Name, "Type": f.VulnClass, "Severity": f.Severity, "Status": f.Status, "Overview": f.Summary, "Task": f.TaskDescription} {
		if columns[name] != want {
			t.Errorf("CSV %s = %q, want original %q", name, columns[name], want)
		}
	}
}
