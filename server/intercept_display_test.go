package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

func TestInterceptDisplayPreservesAuditAndDecisions(t *testing.T) {
	for _, action := range []string{"allow", "deny", "ask"} {
		detail := &db.InterceptDetail{
			InterceptApprovalRow: db.InterceptApprovalRow{InterceptPending: db.InterceptPending{
				ID: 7, Status: "pending", DecisionSource: "model", ToolName: "Bash",
				ToolInput: json.RawMessage(`{"command":"printf '그대로 原文'"}`),
				Reason:    "[模型] 모델 승인에 실패하여 실패 정책에 따라 처리합니다: upstream detail 그대로 原文",
			}},
			Audit: &db.InterceptAudit{
				InitialAction: action, EffectiveAction: action, InitialReason: "원래 판정 原文",
				ModelInput:     json.RawMessage(`{"user_message":"用户 입력"}`),
				DecisionReason: "사람이 실행을 거부했습니다", Output: "원래 증거 原文",
			},
		}
		before, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}
		view := detailedInterceptView(detail)
		if view.DisplayReason != "[模型] Model review failed; applying the failure policy: upstream detail 그대로 原文" || view.DisplayDecisionReason != "Execution was denied by a person" {
			t.Fatalf("unexpected English display: %+v", view)
		}
		after, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Fatal("display translation mutated the source record")
		}
		raw, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip db.InterceptDetail
		if err := json.Unmarshal(raw, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*detail, roundTrip) {
			t.Fatal("display response changed policy, original messages, model input, or evidence")
		}
	}
}

func TestInterceptDisplayPreservesCustomRulesAndModelProse(t *testing.T) {
	ruleID := int64(42)
	for _, reason := range []string{
		"[模型] 모델 승인에 실패하여 실패 정책에 따라 처리합니다: custom policy",
		"가로채기 규칙 [사용자 규칙] 에 따라 이 도구를 실행할 수 없습니다",
		"Custom rule text 사용자 原文",
	} {
		p := db.InterceptPending{RuleID: &ruleID, DecisionSource: "rule", Reason: reason}
		if got := interceptReasonForDisplay(p); got != reason {
			t.Fatalf("custom rule changed: %q", got)
		}
	}
	p := db.InterceptPending{DecisionSource: "model", Reason: "[模型] 판단 근거 原始证据"}
	if got := interceptReasonForDisplay(p); got != p.Reason {
		t.Fatalf("model prose changed: %q", got)
	}
	for _, reason := range []string{"허용", "차단", "확인 요청"} {
		p.Reason = "[模型] " + reason
		if got := interceptReasonForDisplay(p); !strings.HasPrefix(got, "[模型] ") || got == p.Reason {
			t.Fatalf("marker or default verdict translation incorrect: %q", got)
		}
	}
	if len(pendingInterceptViews(nil)) != 0 || len(approvalInterceptViews(nil)) != 0 {
		t.Fatal("empty lists changed")
	}
}
