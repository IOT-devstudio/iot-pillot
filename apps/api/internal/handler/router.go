package handler

import (
	"fmt"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
)

type Router struct {
	eng *gin.Engine
	cfg *config.Config
}

func NewRouter(cfg *config.Config, healthHandler *HealthHandler, authHandler *AuthHandler, tokenManager *utils.TokenManager) *Router {
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

	// *utils.TokenManager 满足 middleware.TokenValidator，这里做一次接口转换即可，
	// 中间件本身不依赖 Redis、签名算法等细节（测试可注入桩实现）。
	authRequired := middleware.AuthRequired(tokenManager)

	apiV1 := r.Group("/api/v1")
	{
		// ── 公开路由（注册前/未登录即可调用）──
		apiV1.POST("/register", authHandler.Register)
		apiV1.POST("/login", authHandler.Login)
		// 注册必须先拿到邮箱验证码：Register 的 code 字段由这里发出的验证码校验。
		apiV1.POST("/send-verify-code", authHandler.SendVerifyCode)
		apiV1.POST("/refresh", authHandler.Refresh)
		apiV1.POST("/logout", authHandler.Logout)

		// ── 需要登录 ──
		apiV1.GET("/me", authRequired, authHandler.Me)

		// ── 管理端（需要 admin 角色）──
		// 两级中间件缺一不可：RequireRole 依赖 AuthRequired 写入的身份，
		// 且在缺少身份时按 401 拒绝而不是放行（见 middleware/auth.go）。
		admin := apiV1.Group("/admin",
			authRequired,
			middleware.RequireRole(middleware.RoleAdmin),
		)
		admin.GET("/users", authHandler.ListUsers)
	}
	return &Router{eng: r, cfg: cfg}
}

func (r *Router) Run() error {
	addr := fmt.Sprintf(":%d", r.cfg.SERVICE.Port)
	return r.eng.Run(addr)
}
