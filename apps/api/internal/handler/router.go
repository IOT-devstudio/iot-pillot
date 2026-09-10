package handler

import (
	"fmt"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Router struct {
	eng *gin.Engine
	cfg *config.Config
}

func NewRouter(cfg *config.Config, healthHandler *HealthHandler, authHandler *AuthHandler) *Router {
	if cfg.SERVICE.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.CORS(*cfg.CORS),
	)

	r.GET("/health", healthHandler.Health)

	apiV1 := r.Group("/api/v1")
	{
		// 认证路由
		apiV1.POST("/register", authHandler.Register)
		apiV1.POST("/login", authHandler.Login)
		apiV1.POST("/refresh", authHandler.Refresh)
		apiV1.POST("/logout", authHandler.Logout)
	}
	return &Router{eng: r, cfg: cfg}
}

func (r *Router) Run() error {
	addr := fmt.Sprintf(":%d", r.cfg.SERVICE.Port)
	return r.eng.Run(addr)
}
