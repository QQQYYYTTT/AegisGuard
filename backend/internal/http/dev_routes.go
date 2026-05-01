package httpapi

import (
	"net/http"

	"aegisguard/internal/demo"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (r *Router) registerDevRoutes() {
	expMgr := demo.NewExperimentManager(r.cfg.RootDir)

	api := r.engine.Group("/api")
	{
		api.GET("/experiments/summaries", func(c *gin.Context) {
			r.handleListSummaries(c, expMgr)
		})
		api.GET("/experiments/summary/:runId", func(c *gin.Context) {
			r.handleGetSummary(c, expMgr)
		})
		api.GET("/experiments/records/:runId", func(c *gin.Context) {
			r.handleGetRecords(c, expMgr)
		})
		api.GET("/experiments/three-gate", func(c *gin.Context) {
			r.handleThreeGateResult(c, expMgr)
		})
		api.GET("/experiments/attack-families", func(c *gin.Context) {
			r.handleAttackFamilyStats(c, expMgr)
		})
	}

	r.logger.Info("[DevMode] 开发模式 API 已启用",
		zap.String("endpoints", "/api/experiments/*"),
	)
}

func (r *Router) handleListSummaries(c *gin.Context, mgr *demo.ExperimentManager) {
	summaries, err := mgr.ListSummaries()
	if err != nil {
		r.logger.Error("读取实验摘要列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取实验摘要失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summaries,
		"total":   len(summaries),
	})
}

func (r *Router) handleGetSummary(c *gin.Context, mgr *demo.ExperimentManager) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}

	summary, err := mgr.GetSummary(runID)
	if err != nil {
		r.logger.Error("读取实验摘要失败", zap.String("run_id", runID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "实验摘要不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}

func (r *Router) handleGetRecords(c *gin.Context, mgr *demo.ExperimentManager) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}

	records, err := mgr.GetRecords(runID)
	if err != nil {
		r.logger.Error("读取实验记录失败", zap.String("run_id", runID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "实验记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
		"total":   len(records),
	})
}

func (r *Router) handleThreeGateResult(c *gin.Context, mgr *demo.ExperimentManager) {
	result, err := mgr.GetThreeGateResult()
	if err != nil {
		r.logger.Error("读取三门禁结果失败", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "三门禁结果不存在，请先运行 run_three_gate_demo.py"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func (r *Router) handleAttackFamilyStats(c *gin.Context, mgr *demo.ExperimentManager) {
	stats, err := mgr.GetAttackFamilyStats()
	if err != nil {
		r.logger.Error("读取攻击族统计失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取攻击族统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
