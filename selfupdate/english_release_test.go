package selfupdate

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEnglishEditionNeverChecksUpstreamReleases(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != "https://api.github.com/repos/humanwill-ai/artex-en/releases/latest" {
			t.Fatalf("unexpected release source: %s", req.URL)
		}
		if req.Header.Get("User-Agent") != "artex-selfupdate" {
			t.Fatal("upstream detection indicator changed")
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	release, err := FetchLatest(context.Background(), client)
	if release != nil || err == nil || !strings.Contains(err.Error(), "No release is available") || !strings.Contains(err.Error(), "private") {
		t.Fatalf("missing/private release should be explained: release=%v err=%v", release, err)
	}
	if calls != 1 {
		t.Fatalf("must not fall back to another source, calls=%d", calls)
	}
}
