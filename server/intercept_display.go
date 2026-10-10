package server

import (
	"strings"

	"github.com/Autumn-27/artex/db"
)

// Display fields are additive. The original reason, policy, model input, and audit
// remain available verbatim; callers must never feed these display fields to agents.
// Rule messages are deliberately untouched: historical rows do not record enough
// configuration to distinguish generated defaults from user-authored rule text.
type interceptPendingView struct {
	db.InterceptPending
	DisplayReason string `json:"display_reason"`
}

type interceptApprovalView struct {
	db.InterceptApprovalRow
	DisplayReason string `json:"display_reason"`
}

type interceptDetailView struct {
	*db.InterceptDetail
	DisplayReason         string `json:"display_reason"`
	DisplayDecisionReason string `json:"display_decision_reason,omitempty"`
}

func interceptReasonForDisplay(p db.InterceptPending) string {
	if p.RuleID != nil || p.DecisionSource == "rule" || (p.DecisionSource != "model" && !strings.HasPrefix(p.Reason, "[模型]")) {
		return p.Reason
	}
	const marker = "[模型] "
	body := strings.TrimPrefix(p.Reason, marker)
	for source, english := range map[string]string{
		"허용":    "Allow",
		"차단":    "Block",
		"확인 요청": "Ask for approval",
		"모델 출력을 해석할 수 없어 실패 정책에 따라 처리합니다": "The model output could not be parsed; applying the failure policy",
	} {
		if body == source {
			return strings.TrimSuffix(p.Reason, body) + english
		}
	}
	for source, english := range map[string]string{
		"검토 맥락이 완전하지 않아 사람의 확인이 필요합니다: ": "The review context is incomplete; human confirmation is required: ",
		"모델 승인에 실패하여 실패 정책에 따라 처리합니다: ":  "Model review failed; applying the failure policy: ",
	} {
		if strings.HasPrefix(body, source) {
			return strings.TrimSuffix(p.Reason, body) + english + strings.TrimPrefix(body, source)
		}
	}
	return p.Reason
}

func interceptDecisionReasonForDisplay(reason string) string {
	switch reason {
	case "작업이 취소되었습니다":
		return "The task was canceled"
	case "실행 전에 작업이 취소되었습니다":
		return "The task was canceled before execution"
	case "승인 시간이 초과되어 시간 초과 정책에 따라 처리합니다":
		return "Approval timed out; applying the timeout policy"
	case "사람이 실행을 거부했습니다":
		return "Execution was denied by a person"
	case "사람이 실행을 허용했습니다":
		return "Execution was approved by a person"
	default:
		return reason
	}
}

func pendingInterceptViews(in []db.InterceptPending) []interceptPendingView {
	out := make([]interceptPendingView, 0, len(in))
	for _, p := range in {
		out = append(out, interceptPendingView{p, interceptReasonForDisplay(p)})
	}
	return out
}

func approvalInterceptViews(in []db.InterceptApprovalRow) []interceptApprovalView {
	out := make([]interceptApprovalView, 0, len(in))
	for _, p := range in {
		out = append(out, interceptApprovalView{p, interceptReasonForDisplay(p.InterceptPending)})
	}
	return out
}

func detailedInterceptView(detail *db.InterceptDetail) interceptDetailView {
	out := interceptDetailView{InterceptDetail: detail, DisplayReason: interceptReasonForDisplay(detail.InterceptPending)}
	if detail.Audit != nil {
		out.DisplayDecisionReason = interceptDecisionReasonForDisplay(detail.Audit.DecisionReason)
	}
	return out
}
