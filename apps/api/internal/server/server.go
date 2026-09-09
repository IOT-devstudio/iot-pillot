// Package server 负责 Gin 路由与中间件装配。
package server

import (
	"fmt"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/handler"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg *config.Config
	eng *gin.Engine
}

func New(cfg *config.Config) *Server {
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.CORS(cfg.CORS),
	)

	// 健康检查（无需鉴权）。
	r.GET("/health", handler.Health)

	// 业务路由分组（后续模块在此挂载）。
	api := r.Group("/api/v1")
	_ = api // 占位：模块路由挂载点

	return &Server{cfg: cfg, eng: r}
}

func (s *Server) Run() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	return s.eng.Run(addr)
}
