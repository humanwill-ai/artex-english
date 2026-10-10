package server

import (
	"testing"

	"github.com/Autumn-27/artex/db"
)

func TestAgentDisplayPreservesStoredAndCustomText(t *testing.T) {
	a := &db.Agent{ID: 42, Key: "goals", Builtin: true, Name: "目标拆解", Description: "把渗透任务目标拆解成若干独立、可验证的子目标。"}
	before := *a
	dto := agentDTO(a)
	if dto.Name != "Goal decomposition" || dto.Description != "Breaks the engagement goal into independent, verifiable subgoals." {
		t.Fatalf("built-in default not translated: %+v", dto)
	}
	if a.Name != before.Name || a.Description != before.Description || dto.Key != a.Key || dto.ID != "42" {
		t.Fatal("display translation changed stored values or identifiers")
	}
	a.Builtin = false
	dto = agentDTO(a)
	if dto.Name != a.Name || dto.Description != a.Description {
		t.Fatal("custom agent text was translated")
	}
	a.Builtin = true
	a.Name, a.Description = "사용자 이름", "自定义描述"
	dto = agentDTO(a)
	if dto.Name != a.Name || dto.Description != a.Description {
		t.Fatal("non-default built-in text was translated")
	}
}

func TestEnglishAndLegacyDefaultConversationTitles(t *testing.T) {
	for _, title := range []string{"", "New conversation", "새 대화", "新对话"} {
		if !isDefaultConversationTitle(title) {
			t.Errorf("default title %q would prevent automatic naming", title)
		}
	}
	for _, title := range []string{"사용자 제목", "User title", "自定义标题"} {
		if isDefaultConversationTitle(title) {
			t.Errorf("custom title %q would be overwritten", title)
		}
	}
}

func TestLegacyReporterDisplayRequiresExactIdentity(t *testing.T) {
	base := db.Agent{Key: "reporter", Role: "assistant", Name: legacyReporterName, Description: legacyReporterDescription}
	if !isLegacyReporterDisplay(&base) {
		t.Fatal("legacy seeded display not recognized")
	}
	for _, change := range []func(*db.Agent){
		func(a *db.Agent) { a.Key = "custom" },
		func(a *db.Agent) { a.Role = "worker" },
		func(a *db.Agent) { a.Builtin = true },
		func(a *db.Agent) { a.Name = "사용자 reporter" },
		func(a *db.Agent) { a.Description = "사용자 설명" },
	} {
		a := base
		change(&a)
		if isLegacyReporterDisplay(&a) {
			t.Fatalf("custom identity accepted: %+v", a)
		}
	}
	// Exact labels alone cannot prove seeding; missing storage preserves them.
	s := &Server{}
	got := s.agentDisplayDTO(&base)
	if got.Name != base.Name || got.Description != base.Description {
		t.Fatal("unproven seed translated")
	}
}
