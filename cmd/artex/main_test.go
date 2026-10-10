package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/Autumn-27/artex/agent"
)

func TestShutdownContextPreservesNamedCause(t *testing.T) {
	signalCtx, signalCancel := context.WithCancel(context.Background())
	ctx, shutdown := shutdownContext(signalCtx)
	defer shutdown(nil)
	signalCancel()
	<-ctx.Done()
	if code, _, _, ok := agent.AbortReason(ctx); !ok || code != "shutdown" {
		t.Fatalf("code=%q ok=%v, want shutdown", code, ok)
	}
}

// TestPrintBannerLocalized checks English display text and unchanged runtime values.
// Operational log messages are outside the banner localization scope.
func TestPrintBannerLocalized(t *testing.T) {
	out := captureStdout(t, func() { printBanner(":8787") })
	for _, want := range []string{"AI autonomous penetration testing system", "Version " + version, "Listening on :8787"} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q: %q", want, out)
		}
	}
	for _, r := range out {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
			t.Errorf("untranslated banner text %q: %q", r, out)
		}
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns whatever
// fn wrote. printBanner uses fmt.Print*, which resolves os.Stdout at call time,
// so swapping it here captures the banner.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}
