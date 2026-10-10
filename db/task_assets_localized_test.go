package db

import "testing"

func TestManualTaskScopeSummaryLocalized(t *testing.T) {
	if manualTaskScopeSummary != "Added manually by the user from the test assets page" {
		t.Fatalf("unexpected task asset provenance label: %q", manualTaskScopeSummary)
	}
}
