package httpapi

import (
	"net/http"
	"sort"

	"aegisguard/internal/gates"

	"github.com/gin-gonic/gin"
)

type PolicyRule = gates.PolicyRuleConfig
type RiskWeights = gates.RiskWeightsConfig
type PolicyConfig = gates.PolicyConfig

func (r *Router) handleGetPolicyConfig(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	resp := r.policyRuntime.Snapshot()
	sort.Slice(resp.Rules, func(i, j int) bool {
		return resp.Rules[i].Priority < resp.Rules[j].Priority
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func (r *Router) handleGetPolicyRules(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	rules := r.policyRuntime.Rules()
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rules})
}

func (r *Router) handleUpdatePolicyRule(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	var req PolicyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	if _, ok, err := r.policyRuntime.UpdateRule(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "persist failed: " + err.Error()})
		return
	} else if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rule updated"})
}

func (r *Router) handleCreatePolicyRule(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	var req PolicyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	created, err := r.policyRuntime.CreateRule(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "persist failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": created, "message": "rule created"})
}

func (r *Router) handleDeletePolicyRule(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	ruleID := c.Param("id")
	if ok := r.policyRuntime.DeleteRule(ruleID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rule deleted"})
}

func (r *Router) handleReorderPolicyRules(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	var req struct {
		RuleIDs []string `json:"rule_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request"})
		return
	}

	if err := r.policyRuntime.Reorder(req.RuleIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "persist failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rules reordered"})
}

func (r *Router) handleUpdatePolicyConfig(c *gin.Context) {
	if r.policyRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "policy runtime unavailable"})
		return
	}

	var req struct {
		RiskWeights     *RiskWeights `json:"risk_weights,omitempty"`
		GlobalThreshold *int         `json:"global_threshold,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request"})
		return
	}

	if err := r.policyRuntime.UpdateConfig(req.RiskWeights, req.GlobalThreshold); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "persist failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "config updated"})
}
