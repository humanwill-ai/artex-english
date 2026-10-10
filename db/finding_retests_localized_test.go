package db

import "testing"

func TestFindingRetestReasonsLocalized(t *testing.T) {
	for name, item := range map[string]struct{ got, want string }{
		"missing conclusion": {retestNoConclusionReason, "The agent did not save a retest conclusion. Review the conversation and run the retest again"},
		"service restart":    {retestServiceRestartReason, "The service restarted and interrupted the retest. Start it again"},
	} {
		if item.got != item.want {
			t.Errorf("%s: got %q, want %q", name, item.got, item.want)
		}
	}
}
