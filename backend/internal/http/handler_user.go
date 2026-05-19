package httpapi

import (
	"net/http"
<<<<<<< HEAD
	"strings"
=======
	"time"
>>>>>>> 04e571098203046fe3378d7c828cb3349a325bd8

	"github.com/gin-gonic/gin"
)

<<<<<<< HEAD
type userAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (r *Router) handleUserRegister(c *gin.Context) {
	var req userAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	session, err := r.userService.Register(req.Username, req.Password, req.Nickname)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

func (r *Router) handleUserLogin(c *gin.Context) {
	var req userAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	session, err := r.userService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

func (r *Router) handleUserRefresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	session, err := r.userService.Refresh(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

func (r *Router) handleUserProfile(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "missing authorization token"})
		return
	}

	profile, err := r.userService.ParseAccessToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": profile})
}

func (r *Router) handleUserLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
=======
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
>>>>>>> 04e571098203046fe3378d7c828cb3349a325bd8
}
