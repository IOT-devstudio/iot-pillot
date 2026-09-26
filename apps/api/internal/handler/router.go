package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/docs"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Router struct {
	eng *gin.Engine
	cfg *config.Config
}

func NewRouter(
	cfg *config.Config,
	healthHandler *HealthHandler,
	authHandler *AuthHandler,
	adminHandler *AdminHandler,
	mailHandler *MailHandler,
	tokenManager *utils.TokenManager,
	adminStore *utils.AdminStore,
) *Router {
	if cfg.SERVICE.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 不信任任何转发头。
	//
	// gin 默认信任所有代理，ClientIP() 会采信 X-Forwarded-For —— 而限流正好以
	// ClientIP() 作为 key（验证码的 IP 配额），伪造这个头就能绕过限流。
	// 真实来源 IP 由 nginx 层负责，应用层只认直连地址。
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("[router] 设置可信代理失败，将沿用 gin 默认值: %v", err)
	}

	r.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.CORS(*cfg.CORS),
	)

	// 存活探针：不依赖外部，容器存活判断用
	r.GET("/health", healthHandler.Health)
	// 就绪探针：探活 DB 与 Redis，任一不可用返回 503
	r.GET("/health/ready", healthHandler.Ready)

	// 接口文档与在线调试页（Swagger UI），对应 FastAPI 的 /openapi.json + /docs：
	//   GET /openapi.json  —— spec 本体，由 swag 从 handler 注解生成（docs/ 是生成物）
	//   GET /docs/*any     —— Swagger UI，静态资源内嵌在二进制里，不依赖任何 CDN
	//
	// 默认 debug 开、release 关（config.DocsConfig）：文档会把整个接口面暴露出来，
	// 生产环境不该默认开放。需要时用 docs.enabled / IOT_PILOT_DOCS_ENABLED 显式打开。
	//
	// spec 里刻意不写 host（见 cmd/main.go 的注解）：Swagger UI 用访问它的域名，
	// 本地 localhost:8080 与线上域名都能直接 "Try it out"。
	if cfg.DOCS != nil && cfg.DOCS.Enabled {
		r.GET("/openapi.json", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
		})

		// UI 的静态资源由 swaggo/files 内嵌提供；只补一条：裸访问 /docs 或 /docs/
		// （webdav 把 "/" 当目录）时把人送到 index.html，免得看到 404。
		swaggerUI := ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi.json"))
		serveDocs := func(c *gin.Context) {
			if p := c.Param("any"); p == "" || p == "/" {
				c.Redirect(http.StatusFound, "/docs/index.html")
				return
			}
			swaggerUI(c)
		}
		r.GET("/docs", serveDocs)
		r.GET("/docs/*any", serveDocs)
	}

	// *utils.TokenManager 满足 middleware.TokenValidator，
	// *utils.AdminStore 满足 middleware.AdminChecker —— 这里做一次接口转换，
	// 中间件本身不依赖 Redis、签名算法等细节（测试可注入桩实现）。
	authRequired := middleware.AuthRequired(tokenManager)
	requireAdmin := middleware.RequireAdmin(adminStore)

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
		// /me 的读写都只要求登录（不是 admin）：改的是令牌本人的资料，
		// 目标身份由 authRequired 解出的 userID 决定，body 里塞目标参数无效。
		apiV1.GET("/me", authRequired, authHandler.Me)
		apiV1.PUT("/me", authRequired, authHandler.UpdateMe)

		// ── 管理端（需要管理员）──
		// 两级中间件缺一不可：RequireAdmin 依赖 AuthRequired 写入的身份，
		// 且在缺少身份时按 401 拒绝而不是放行（见 middleware/auth.go）。
		// 管理员判定读 Redis 名单，因此撤销管理员对后续请求立即生效。
		admin := apiV1.Group("/admin", authRequired, requireAdmin)
		admin.GET("/users", adminHandler.ListUsers)
		admin.GET("/admins", adminHandler.ListAdmins)
		admin.POST("/admins", adminHandler.GrantAdmin)
		admin.DELETE("/admins/:user_id", adminHandler.RevokeAdmin)

		// ── 管理端-邮件（同样要求管理员）──
		admin.GET("/mail-templates", mailHandler.ListMailTemplates)
		admin.POST("/mail-templates", mailHandler.CreateMailTemplate)
		admin.PUT("/mail-templates/:id", mailHandler.UpdateMailTemplate)
		admin.DELETE("/mail-templates/:id", mailHandler.DeleteMailTemplate)
		admin.POST("/mails/send", mailHandler.SendMailToUser)
		admin.POST("/mails/send-by-email", mailHandler.SendMailToEmail)
		admin.POST("/mails/send-bulk", mailHandler.SendMailBulk)
		admin.GET("/mails", mailHandler.ListMails)
		// 批量删除用 DELETE 方法（语义即删除；body 携带 ids 数组，gin 正常绑定）
		admin.DELETE("/mails/batch-delete", mailHandler.DeleteMails)
	}
	return &Router{eng: r, cfg: cfg}
}

// Run 启动 HTTP 服务，并在收到 SIGINT/SIGTERM 时优雅退出。
//
// 为什么需要优雅退出：发信、群发是长请求，直接杀进程会让调用方拿到连接重置，
// 而且无法判断"邮件到底发出去没有"。这里停止接收新请求后，给在途请求 10 秒收尾。
//
// 刻意没有设置 ReadTimeout/WriteTimeout：本轮只按要求加优雅退出；
// 详见后续待办（超时交给 nginx 层处理）。
func (r *Router) Run() error {
	addr := fmt.Sprintf(":%d", r.cfg.SERVICE.Port)

	server := &http.Server{
		Addr:    addr,
		Handler: r.eng,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Println("[router] 收到退出信号，停止接收新请求并等待在途请求收尾…")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// shutdownGracePeriod 优雅退出的等待上限。
const shutdownGracePeriod = 10 * time.Second
