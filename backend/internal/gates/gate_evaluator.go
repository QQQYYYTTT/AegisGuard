package gates

import (
	"net/http"
	"time"

	"aegisguard/internal/contract"
	"aegisguard/internal/interfaces"
)

type GateEvaluatorImpl struct {
	messageGate *MessageGate
	actionGate  *ActionGate
	returnGate  *ReturnGate
	store       *DecisionStore
}

func NewGateEvaluator(messageGate *MessageGate, actionGate *ActionGate, returnGate *ReturnGate, store *DecisionStore) contract.GateEvaluator {
	return &GateEvaluatorImpl{
		messageGate: messageGate,
		actionGate:  actionGate,
		returnGate:  returnGate,
		store:       store,
	}
}

func (ge *GateEvaluatorImpl) EvaluateMessage(body []byte) (interfaces.Decision, string) {
	decision, reason := ge.messageGate.Evaluate(body)
	score, level, rules := ExtractReasonMetadata(reason)
	ge.store.Add(interfaces.GateDecision{
		RequestID:    "manual",
		Timestamp:    time.Now(),
		GateType:     "message",
		Decision:     decision.String(),
		RiskScore:    score,
		RiskLevel:    level,
		MatchedRules: rules,
		Reason:       reason,
	})
	return interfaces.Decision(decision), reason
}

func (ge *GateEvaluatorImpl) EvaluateAction(toolName string, params map[string]interface{}, headers http.Header) (interfaces.Decision, string) {
	decision, reason := ge.actionGate.Evaluate(toolName, params, headers)
	score, level, rules := ExtractReasonMetadata(reason)
	ge.store.Add(interfaces.GateDecision{
		RequestID:    "manual",
		Timestamp:    time.Now(),
		GateType:     "action",
		Decision:     decision.String(),
		RiskScore:    score,
		RiskLevel:    level,
		MatchedRules: rules,
		Reason:       reason,
		ToolName:     toolName,
	})
	return interfaces.Decision(decision), reason
}

func (ge *GateEvaluatorImpl) EvaluateReturn(body []byte) (interfaces.Decision, string) {
	decision, reason := ge.returnGate.Evaluate(body)
	score, level, rules := ExtractReasonMetadata(reason)
	ge.store.Add(interfaces.GateDecision{
		RequestID:    "manual",
		Timestamp:    time.Now(),
		GateType:     "return",
		Decision:     decision.String(),
		RiskScore:    score,
		RiskLevel:    level,
		MatchedRules: rules,
		Reason:       reason,
	})
	return interfaces.Decision(decision), reason
}
