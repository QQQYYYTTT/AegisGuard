package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (r *Router) registerUserRoutes() {
	r.engine.POST("/login", r.handleLogin)
	r.engine.POST("/refresh-token", r.handleRefreshToken)
	r.engine.GET("/get-async-routes", r.handleAsyncRoutes)
}

func (r *Router) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Username == "" {
		req.Username = "admin"
	}

	roles := []string{"common"}
	permissions := []string{"permission:btn:add", "permission:btn:edit"}
	nickname := "AegisGuard 用户"
	if req.Username == "admin" {
		roles = []string{"admin"}
		permissions = []string{"*:*:*"}
		nickname = "AegisGuard 管理员"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"avatar":       "https://avatars.githubusercontent.com/u/44761321",
			"username":     req.Username,
			"nickname":     nickname,
			"roles":        roles,
			"permissions":  permissions,
			"accessToken":  "aegisguard-" + req.Username + "-token",
			"refreshToken": "aegisguard-" + req.Username + "-refresh",
			"expires":      time.Now().Add(24 * time.Hour).Format("2006/01/02 15:04:05"),
		},
	})
}

func (r *Router) handleRefreshToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"accessToken":  "aegisguard-refreshed-token",
			"refreshToken": "aegisguard-refreshed-refresh",
			"expires":      time.Now().Add(24 * time.Hour).Format("2006/01/02 15:04:05"),
		},
	})
}

func (r *Router) handleAsyncRoutes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    []gin.H{},
	})
}
