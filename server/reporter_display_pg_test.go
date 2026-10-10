package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
)

func TestLegacyReporterDisplaySeedProvenancePG(t *testing.T) {
	dsn, _, err := db.DSN()
	if err != nil {
		t.Skip("test postgres not configured")
	}
	pg, err := db.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer pg.Close()
	s := &Server{m: &Manager{pg: pg}}
	for _, seeded := range []bool{false, true} {
		key := fmt.Sprintf("test_reporter_display_%d_%t", time.Now().UnixNano(), seeded)
		a, err := pg.CreateAgent(key, legacyReporterName, legacyReporterDescription)
		if err != nil {
			t.Fatal(err)
		}
		defer pg.DeleteAgent(key)
		prompt := agent.DefaultAssistantPrompt
		if seeded {
			prompt = agent.ReporterDefaultPrompt
		}
		if err := pg.SeedPromptIfEmpty(a.ID, prompt); err != nil {
			t.Fatal(err)
		}
		// The temporary row has a unique key to avoid touching an installed reporter.
		// The view candidate uses its real prompt-history ID and the reporter role/key.
		a.Key = "reporter"
		before := *a
		got := s.agentDisplayDTO(a)
		if seeded {
			if got.Name != reporterAgentName || got.Description != reporterAgentDescription {
				t.Fatal("proven seeded reporter not localized")
			}
			a.Description = "Customized description 사용자"
			custom := s.agentDisplayDTO(a)
			if custom.Name != a.Name || custom.Description != a.Description {
				t.Fatal("customized seeded reporter changed")
			}
			a.Description = before.Description
		} else {
			if got.Name != a.Name || got.Description != a.Description {
				t.Fatal("custom reporter localized without seed proof")
			}
			// Copying the reporter prompt later must not retroactively prove seeding.
			if _, err := pg.SavePrompt(a.ID, agent.ReporterDefaultPrompt, "custom edit", "user"); err != nil {
				t.Fatal(err)
			}
			if got := s.agentDisplayDTO(a); got.Name != a.Name || got.Description != a.Description {
				t.Fatal("later custom prompt mistaken for system seed")
			}
		}
		stored, err := pg.GetAgentByKey(key)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Name != before.Name || stored.Description != before.Description {
			t.Fatal("display changed stored metadata")
		}
		// Reopening reruns database initialization; customization must survive it.
		const customName = "Custom report writer 사용자"
		const customDescription = "Custom report description 中文"
		if err := pg.UpdateAgentMeta(key, customName, customDescription); err != nil {
			t.Fatal(err)
		}
		reopened, err := db.Open(dsn)
		if err != nil {
			t.Fatal(err)
		}
		persisted, err := reopened.GetAgentByKey(key)
		reopened.Close()
		if err != nil {
			t.Fatal(err)
		}
		if persisted == nil || persisted.Name != customName || persisted.Description != customDescription {
			t.Fatal("database initialization overwrote custom reporter metadata")
		}
	}
}
