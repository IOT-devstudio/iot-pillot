package handler

import (
	"log"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"

	"github.com/gin-gonic/gin"
)

// failInternal 记录真实错误，只给客户端返回通用文案（release 模式）。
//
// 为什么必须脱敏：错误原文里常带基础设施信息 ——
// `dial tcp [::1]:587` 暴露 SMTP 主机与端口，`SQLSTATE 42703` 暴露列名，
// pgx 的连接错误还会带上库名与用户名。这些都是未认证调用方也能触发的路径
// （注册、登录、发送验证码）。
//
// debug 模式仍返回原文：本地联调时看到真实原因远比"服务不可用"有用，
// 而这个模式的部署前提就是不对公网开放。
func failInternal(c *gin.Context, err error) {
	log.Printf("[error] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)

	if gin.Mode() == gin.ReleaseMode {
		response.FailServer(c, "服务暂时不可用，请稍后重试")
		return
	}
	response.FailServer(c, err.Error())
}
