package server

import (
	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
)

// agentDisplayText translates known built-in display defaults at the API boundary.
// Stored names, descriptions, prompts, and all custom agent text remain intact.
func agentDisplayText(a *db.Agent) (string, string) {
	name, description := a.Name, a.Description
	if !a.Builtin {
		return name, description
	}
	defaults := map[string]struct{ sourceName, sourceDescription, name, description string }{
		"goals":     {"目标拆解", "把渗透任务目标拆解成若干独立、可验证的子目标。", "Goal decomposition", "Breaks the engagement goal into independent, verifiable subgoals."},
		"planner":   {"规划", "读取态势、判定目标，只在确有未覆盖的新方向时补充探索意图（每任务一个规划循环）。", "Planner", "Reviews the current state and goals, adding exploration intents only for new, uncovered directions (one planning loop per task)."},
		"mainagent": {"主", "人机接口：观察进展，把人的意图落成 hint 或高优先级意图。", "Main agent", "Tracks progress and turns user requests into hints or high-priority intents."},
		"worker":    {"执行", "领取一条意图执行，把发现的事实/漏洞写回知识图谱后停止。", "Worker", "Executes one intent, records discovered facts and findings in the knowledge graph, then stops."},
		"auto":      {"Auto", "平台操作助手：用工具管理任务(建/看/暂停/给提示)与资产，并可创建/修改 skill、自定义工具、MCP。", "Auto", "Uses tools to manage tasks and assets, and create or edit skills, custom tools, and MCP connections."},
		"pentest":   {"渗透测试", "独立渗透 agent：一人从侦察→找攻击面→深入利用→验证→收尾走完整条链，自己规划、自己执行、自己对抗式验证。", "Penetration testing", "An independent agent that plans, executes, and verifies the full engagement, from reconnaissance and attack-surface discovery through exploitation, validation, and wrap-up."},
	}
	if d, ok := defaults[a.Key]; ok {
		if name == d.sourceName {
			name = d.name
		}
		if description == d.sourceDescription {
			description = d.description
		}
	}
	return name, description
}

const legacyReporterName = "보고서 작성"
const legacyReporterDescription = "취약점 상세 보고서 작성: 취약점을 발견하면 자동으로 트리거되어, 증거와 실행 과정을 조회한 뒤 Markdown 보고서를 작성해 해당 취약점에 기록합니다."

func isLegacyReporterDisplay(a *db.Agent) bool {
	return !a.Builtin && a.Key == "reporter" && a.Role == "assistant" && a.Name == legacyReporterName && a.Description == legacyReporterDescription
}

// agentDisplayDTO recognizes the exact old auto-seeded reporter without trusting
// the seed-attempt setting: that setting is also written when a custom agent
// already occupies the key. Its first system-authored prompt is the seed proof.
// A customized name or description leaves both fields untouched. Query failure
// conservatively preserves the original values, and nothing is written to storage.
func (s *Server) agentDisplayDTO(a *db.Agent) AgentDTO {
	out := agentDTO(a)
	if !isLegacyReporterDisplay(a) || s.m == nil || s.m.pg == nil {
		return out
	}
	var seeded bool
	err := s.m.pg.QueryRow(`SELECT EXISTS (
 SELECT 1 FROM agent_prompts WHERE agent_id=$1 AND version=1
 AND updated_by='system' AND note IN ('内置默认','Built-in default') AND template_text IN ($2,$3)
 )`, a.ID, legacyPromptDefaults["reporter"][0], agent.ReporterDefaultPrompt).Scan(&seeded)
	if err == nil && seeded {
		out.Name, out.Description = reporterAgentName, reporterAgentDescription
	}
	return out
}

func (s *Server) agentDisplayDTOs(in []*db.Agent) []AgentDTO {
	out := make([]AgentDTO, 0, len(in))
	for _, a := range in {
		out = append(out, s.agentDisplayDTO(a))
	}
	return out
}
