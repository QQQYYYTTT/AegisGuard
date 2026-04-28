// internal/http/router.go
// HTTP 路由模块 - 基于 Gin 框架实现
// 修复原 main.go 中引用的缺失包
package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"aegisguard/internal/config"
	"aegisguard/internal/gateway"
	"aegisguard/internal/vkey"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// bodyLogWriter 用于捕获响应体的 Writer
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Router HTTP 路由结构体
type Router struct {
	engine    *gin.Engine
	proxy     *gateway.AegisProxy
	vkeyMgr   *vkey.Manager
	logger    *zap.Logger
	targetURL string
}

// NewRouter 创建 HTTP 路由
// 这是应用程序的入口，整合所有组件
func NewRouter(cfg config.Config) (*Router, error) {
	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	// 初始化虚拟密钥管理器
	vkeyMgr, err := vkey.NewManager(logger, cfg.VKeyConfigPath)
	if err != nil {
		logger.Warn("虚拟密钥管理器初始化警告", zap.Error(err))
		// 继续运行，使用空配置
	}

	// 创建 Gin 引擎
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(logger))

	// 创建 AegisGuard 代理
	proxy, err := gateway.NewAegisProxy(cfg.TargetURL, vkeyMgr, logger)
	if err != nil {
		return nil, err
	}

	router := &Router{
		engine:    engine,
		proxy:     proxy,
		vkeyMgr:   vkeyMgr,
		logger:    logger,
		targetURL: cfg.TargetURL,
	}

	// 注册路由
	router.registerRoutes()

	return router, nil
}

// registerRoutes 注册所有路由
func (r *Router) registerRoutes() {
	// 健康检查
	r.engine.GET("/health", r.handleHealth)

	// OpenAI API 兼容路由 - 所有请求都走代理
	// 这是低侵入接入的核心：Agent 以为在调用 OpenAI，实际上被网关拦截
	r.engine.Any("/v1/*path", r.handleProxy)

	// 管理接口 - 虚拟密钥管理
	admin := r.engine.Group("/admin")
	{
		admin.GET("/vkeys", r.handleListVKeys)         // 列出所有虚拟密钥
		admin.POST("/vkeys", r.handleCreateVKey)       // 创建虚拟密钥
		admin.GET("/vkeys/:id", r.handleGetVKey)       // 获取虚拟密钥详情
		admin.DELETE("/vkeys/:id", r.handleRevokeVKey) // 吊销虚拟密钥
	}

	// 审计接口
	r.engine.GET("/audit/logs", r.handleAuditLogs)
}

// handleProxy 处理代理请求
// 所有 LLM 请求都经过这里，实现透明代理
func (r *Router) handleProxy(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	method := c.Request.Method
	clientIP := c.ClientIP()

	// 读取请求体用于日志
	bodyBytes, _ := c.GetRawData()
	if len(bodyBytes) > 0 {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// 提取虚拟密钥
	authHeader := c.GetHeader("Authorization")
	vkeyID := vkey.ExtractVKey(authHeader)

	// 打印请求接收日志
	r.logger.Info("【网关】收到 Agent 请求",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
		zap.String("vkey_id", vkeyID),
		zap.Int("body_size", len(bodyBytes)),
	)

	if vkeyID == "" {
		r.logger.Error("【网关】请求缺少虚拟密钥",
			zap.String("ip", clientIP),
			zap.String("path", path),
			zap.String("auth_header", authHeader),
		)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "缺少虚拟密钥，请使用 vsk- 前缀的密钥",
		})
		return
	}

	// 验证虚拟密钥
	vk, err := r.vkeyMgr.ValidateAndResolve(vkeyID)
	if err != nil {
		r.logger.Error("【网关】虚拟密钥验证失败",
			zap.String("vkey_id", vkeyID),
			zap.String("ip", clientIP),
			zap.Error(err),
		)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 打印密钥验证成功日志
	r.logger.Info("【网关】虚拟密钥验证成功",
		zap.String("vkey_id", vkeyID),
		zap.String("agent_id", vk.AgentID),
		zap.String("scope", vk.Scope),
	)

	// 将虚拟密钥信息存入 HTTP Request 上下文，供 proxy 使用
	ctx := context.WithValue(c.Request.Context(), "vkey_info", vk)
	c.Request = c.Request.WithContext(ctx)

	// 创建响应记录器来捕获响应
	blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
	c.Writer = blw

	// 转发到代理
	r.proxy.ServeHTTP(c.Writer, c.Request)

	// 打印请求完成日志
	duration := time.Since(start)
	r.logger.Info("【网关】请求处理完成",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("vkey_id", vkeyID),
		zap.String("agent_id", vk.AgentID),
		zap.Int("status", c.Writer.Status()),
		zap.Int("response_size", blw.body.Len()),
		zap.Duration("duration", duration),
	)
}

// handleHealth 健康检查
func (r *Router) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"target_url":  r.targetURL,
		"vkeys_count": len(r.vkeyMgr.ListKeys()),
	})
}

// handleListVKeys 列出所有虚拟密钥
func (r *Router) handleListVKeys(c *gin.Context) {
	keys := r.vkeyMgr.ListKeys()
	c.JSON(http.StatusOK, gin.H{
		"virtual_keys": keys,
		"count":        len(keys),
	})
}

// handleCreateVKey 创建虚拟密钥
func (r *Router) handleCreateVKey(c *gin.Context) {
	var req struct {
		AgentID    string `json:"agent_id" binding:"required"`
		RealAPIKey string `json:"real_api_key" binding:"required"`
		Scope      string `json:"scope"` // read/write/admin，默认 read
		RateLimit  int    `json:"rate_limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Scope == "" {
		req.Scope = "read"
	}

	vk, err := r.vkeyMgr.CreateKey(req.AgentID, req.RealAPIKey, req.Scope, req.RateLimit, 0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	r.logger.Info("创建虚拟密钥",
		zap.String("vkey_id", vk.KeyID),
		zap.String("agent_id", vk.AgentID),
	)

	c.JSON(http.StatusCreated, gin.H{
		"virtual_key": vk,
		"message":     "虚拟密钥创建成功，请妥善保管",
	})
}

// handleGetVKey 获取虚拟密钥详情
func (r *Router) handleGetVKey(c *gin.Context) {
	id := c.Param("id")
	vk, err := r.vkeyMgr.GetKeyInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"virtual_key": vk})
}

// handleRevokeVKey 吊销虚拟密钥
func (r *Router) handleRevokeVKey(c *gin.Context) {
	id := c.Param("id")
	if err := r.vkeyMgr.RevokeKey(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	r.logger.Info("吊销虚拟密钥", zap.String("vkey_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "虚拟密钥已吊销"})
}

// handleAuditLogs 获取审计日志
func (r *Router) handleAuditLogs(c *gin.Context) {
	// TODO: 实现审计日志查询
	c.JSON(http.StatusOK, gin.H{
		"logs": []interface{}{},
		"note": "审计日志功能开发中",
	})
}

// requestLogger 请求日志中间件
func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info("HTTP请求",
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
		)
	}
}

// ServeHTTP 实现 http.Handler 接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}

// Engine 返回 Gin 引擎（用于测试）
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
