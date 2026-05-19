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

func (ge *GateEvaluatorImpl) EvaluateMessage(requestID string, body []byte) interfaces.EvaluateResult {
	result := ge.messageGate.Evaluate(body)
	ge.store.Add(interfaces.GateDecision{
		RequestID:    requestID,
		Timestamp:    time.Now(),
		GateType:     "message",
		Decision:     result.Decision,
		RiskScore:    result.RiskScore,
		RiskLevel:    result.RiskLevel,
		MatchedRules: result.MatchedRules,
		Reason:       result.Reason,
	})
	return result
}

func (ge *GateEvaluatorImpl) EvaluateAction(requestID string, toolName string, params map[string]interface{}, headers http.Header) interfaces.EvaluateResult {
	result := ge.actionGate.Evaluate(toolName, params, headers)
	ge.store.Add(interfaces.GateDecision{
		RequestID:    requestID,
		Timestamp:    time.Now(),
		GateType:     "action",
		Decision:     result.Decision,
		RiskScore:    result.RiskScore,
		RiskLevel:    result.RiskLevel,
		MatchedRules: result.MatchedRules,
		Reason:       result.Reason,
		ToolName:     toolName,
	})
	return result
}

func (ge *GateEvaluatorImpl) EvaluateReturn(requestID string, body []byte) interfaces.EvaluateResult {
	result := ge.returnGate.Evaluate(body)
	ge.store.Add(interfaces.GateDecision{
		RequestID:    requestID,
		Timestamp:    time.Now(),
		GateType:     "return",
		Decision:     result.Decision,
		RiskScore:    result.RiskScore,
		RiskLevel:    result.RiskLevel,
		MatchedRules: result.MatchedRules,
		Reason:       result.Reason,
	})
	return result
}
