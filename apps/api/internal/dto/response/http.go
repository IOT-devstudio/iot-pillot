package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OK 返回成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Success(data))
}

// OKWithMsg 返回带自定义提示的成功响应
func OKWithMsg(c *gin.Context, msg string, data interface{}) {
	r := Success(data)
	r.Message = msg
	c.JSON(http.StatusOK, r)
}

// Fail 返回失败响应
func Fail(c *gin.Context, httpCode, bizCode int, msg string) {
	c.JSON(httpCode, Error(bizCode, msg))
}

// FailInvalidParam 参数错误（HTTP 400）
func FailInvalidParam(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, CodeInvalidParam, msg)
}

// FailUnauthorized 未认证 / token 失效（HTTP 401）
func FailUnauthorized(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, CodeUnauthorized, msg)
}

// FailServer 服务器内部错误（HTTP 500）
func FailServer(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, CodeServerError, msg)
}
