package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"aegisguard/internal/audit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (r *Router) auditManualRequest(c *gin.Context, payload any) (string, time.Time) {
	if r == nil || r.auditor == nil {
		return "", time.Now()
	}

	requestID := uuid.NewString()
	body, _ := json.Marshal(payload)
	r.auditor.LogRequest(audit.LogInput{
		RequestID: requestID,
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Body:      body,
		ClientIP:  c.ClientIP(),
	})
	return requestID, time.Now()
}

func (r *Router) auditManualResponse(
	requestID string,
	start time.Time,
	statusCode int,
	decision string,
	reason string,
	gateType string,
	riskScore int,
	riskLevel string,
	matchedRules []string,
	tokenStatus string,
	authMode string,
) {
	if r == nil || r.auditor == nil || requestID == "" {
		return
	}

	r.auditor.LogResponse(requestID, audit.LogResponseInput{
		StatusCode:   statusCode,
		Duration:     time.Since(start),
		Decision:     decision,
		Reason:       reason,
		GateType:     gateType,
		RiskScore:    riskScore,
		RiskLevel:    riskLevel,
		MatchedRules: matchedRules,
		TokenStatus:  tokenStatus,
		AuthMode:     authMode,
	})
}

func boolDecision(valid bool) string {
	if valid {
		return "Allow"
	}
	return "Block"
}

func statusFromBool(valid bool, okValue, failValue string) string {
	if valid {
		return okValue
	}
	return failValue
}

func httpStatusForDecision(decision string) int {
	switch decision {
	case "Block", "Deny":
		return http.StatusForbidden
	default:
		return http.StatusOK
	}
}
