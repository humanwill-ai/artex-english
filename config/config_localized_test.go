package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// hasHan detects basic CJK Han ideographs in legacy fixtures. The English
// application-message assertion below separately rejects Han and Hangul.
func hasHan(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func hasHangul(s string) bool {
	for _, r := range s {
		if r >= 0xac00 && r <= 0xd7a3 {
			return true
		}
	}
	return false
}

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

// TestPostgresDSNErrorLocalized pins the install-path startup message to English.
// When neither ARTEX_PG_DSN nor a config file supplies a database, PostgresDSN
// returns an error that propagates verbatim (db.DSN → server.NewManager → the
// `log.Fatalf("open stores: %v", err)` in cmd/artex/main.go). An operator who
// boots with missing config sees it first, so it must read as English.
func TestPostgresDSNErrorLocalized(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARTEX_PG_DSN", "")
	t.Setenv("ARTEX_CONFIG", filepath.Join(dir, "nope.json"))

	_, _, err := PostgresDSN()
	if err == nil {
		t.Fatal("missing config should error")
	}
	assertEnglish(t, "startup error", err.Error())

	// The identifiers an operator must act on stay verbatim (not translated).
	for _, want := range []string{"ARTEX_PG_DSN", "database", "dsn", "host/user/dbname"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("startup error should keep identifier %q verbatim: %q", want, err.Error())
		}
	}

	// What the operator actually sees after propagation (no %w wrapping anywhere
	// on the path), recorded so the message can be eyeballed in context.
	t.Logf("open stores: %v", err)
}

// TestPostgresDSNSourceLocalized pins the three source labels (logged at
// server/manager.go:361) to English. They live in the same function as the
// startup error, so they are localized together to avoid a mixed-language file.
func TestPostgresDSNSourceLocalized(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// env source
	t.Setenv("ARTEX_CONFIG", filepath.Join(dir, "nope.json"))
	t.Setenv("ARTEX_PG_DSN", "postgres://envwins/x")
	if _, source, err := PostgresDSN(); err != nil {
		t.Fatalf("env source: %v", err)
	} else {
		assertEnglish(t, "env source", source)
	}

	// config file dsn source
	t.Setenv("ARTEX_PG_DSN", "")
	if err := os.WriteFile(cfgPath, []byte(`{"database":{"dsn":"postgres://full/dsn"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARTEX_CONFIG", cfgPath)
	if _, source, err := PostgresDSN(); err != nil {
		t.Fatalf("dsn source: %v", err)
	} else {
		assertEnglish(t, "dsn source", source)
	}

	// config file fields source
	if err := os.WriteFile(cfgPath, []byte(`{"database":{"host":"10.1.2.3","dbname":"d","user":"u"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, source, err := PostgresDSN(); err != nil {
		t.Fatalf("fields source: %v", err)
	} else {
		assertEnglish(t, "fields source", source)
	}
}
