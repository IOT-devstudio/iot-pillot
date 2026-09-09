// Package handler 提供 HTTP handler。
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Health 健康检查端点，返回服务状态与当前 UTC 时间。
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}
