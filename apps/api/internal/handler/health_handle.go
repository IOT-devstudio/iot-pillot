// Package handler 提供 HTTP handler。
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
)

// DatabasePinger / RedisPinger 健康检查对依赖的最小要求。
//
// 刻意拆成**两个不同的命名接口**（方法集相同），而不是共用一个：
// 构造函数的两个参数如果类型相同，wire 无法判断哪个实现该注入哪个位置。
// 拆开之后绑定关系是唯一的，handler 包也不需要认识 gorm / rueidis。
type DatabasePinger interface {
	Ping(ctx context.Context) error
}

type RedisPinger interface {
	Ping(ctx context.Context) error
}

// NewDatabasePinger / NewRedisPinger 把具体实现作为上面两个接口提供给装配层。
//
// 与 utils.NewAdminDirectory 同理：wire 按类型连线，隐式接口满足对它无效
// （原写法是 wire.Bind(new(DatabasePinger), new(*repository.DatabasePinger))）。
// 放在 handler 是因为接口声明在这一侧 —— repository / utils 反过来 import handler 会成环。
// 这不新增层次依赖：handler 早已使用 repository 的仓储接口（auth_handle、mail_handle）。
func NewDatabasePinger(pinger *repository.DatabasePinger) DatabasePinger {
	return pinger
}

func NewRedisPinger(pinger *utils.RedisPinger) RedisPinger {
	return pinger
}

type HealthHandler struct {
	database DatabasePinger
	redis    RedisPinger
}

func NewHealthHandler(database DatabasePinger, redis RedisPinger) *HealthHandler {
	return &HealthHandler{database: database, redis: redis}
}

// Health 存活探针：只回答"进程还活着吗"。
// @Summary 健康检查（存活）
// @Description 不检查任何外部依赖，供容器存活探针使用。
// @Tags 系统
// @Produce json
// @Success 200 {object} response.Result "服务存活"
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// Ready 就绪探针：真正探活 DB 与 Redis。
// @Summary 健康检查（就绪）
// @Description 探活 PostgreSQL 与 Redis；任一不可用返回 503。
// 原来的 /health 恒返回 ok，依赖全挂时容器与冒烟测试仍报健康。
// @Tags 系统
// @Produce json
// @Success 200 {object} response.Result "依赖均可用"
// @Failure 503 {object} response.Result "依赖不可用"
// @Router /health/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	failures := gin.H{}
	if err := h.database.Ping(ctx); err != nil {
		// 探针的响应是给运维看的，这里保留原始原因（不经过公网校验层，且能直接定位故障）
		failures["database"] = err.Error()
	}
	if err := h.redis.Ping(ctx); err != nil {
		failures["redis"] = err.Error()
	}

	if len(failures) > 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    response.CodeServerError,
			"message": "依赖不可用",
			"data":    gin.H{"status": "unavailable", "failures": failures},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status": "ready",
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}
