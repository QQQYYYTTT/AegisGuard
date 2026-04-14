package security

import (
	"regexp"
	"strings"
)

type GateDecision struct {
	Action string `json:"action"`
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

func DecideGate(body VerifyRequestBody, verification Verification) GateDecision {
	text := strings.ToLower(body.UserGoal + "\n" + body.RawResult + "\n" + body.RequestedScope)
	poisoned := regexp.MustCompile(`ignore all safety|ignore previous|remember this command forever|写入长期记忆|忽略规则`).MatchString(text)
	dangerousScope := regexp.MustCompile(`system_profile|full_table|admin|delete|export_all`).MatchString(body.RequestedScope)

	if !verification.SignatureValid || !verification.SessionMatch || !verification.AgentMatch {
		return GateDecision{Action: "quarantine", Stage: "RequireShield", Reason: "Authorization chain validation failed."}
	}
	if !verification.NotExpired {
		return GateDecision{Action: "quarantine", Stage: "RequireShield", Reason: "Authorization token expired before execution."}
	}
	if !verification.ScopeAllowed || dangerousScope {
		return GateDecision{Action: "deny", Stage: "Action Gate", Reason: "Requested scope exceeds least-privilege boundary."}
	}
	if poisoned {
		return GateDecision{Action: "degrade", Stage: "Return Gate", Reason: "Potential prompt or memory contamination detected."}
	}
	return GateDecision{Action: "allow", Stage: "Action Gate", Reason: "Request satisfies authorization and policy checks."}
}
