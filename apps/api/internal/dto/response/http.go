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

// FailForbidden 无权限（HTTP 403）
func FailForbidden(c *gin.Context, msg string) {
	Fail(c, http.StatusForbidden, CodeForbidden, msg)
}

// FailServer 服务器内部错误（HTTP 500）
func FailServer(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, CodeServerError, msg)
}

// FailTokenExpired token 已过期，需要刷新（HTTP 401，业务码 4010）
func FailTokenExpired(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, CodeTokenExpired, msg)
}

// FailSessionInvalidated 会话已失效（HTTP 401，业务码 4011）
func FailSessionInvalidated(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, CodeSessionInvalidated, msg)
}

// FailTooManyRequests 请求过于频繁（HTTP 429）
func FailTooManyRequests(c *gin.Context, msg string) {
	Fail(c, http.StatusTooManyRequests, CodeTooManyRequests, msg)
}

// FailNotFound 资源不存在（HTTP 404）
//
// 附件读取端点用它统一表达「不存在 / 无权看」：按 ADR 0004 §1，归属判定失败时
// 一律 404 而非 403，不泄漏「这个 id 存在但你没权限」，避免被用来枚举内部论坛规模。
func FailNotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, CodeNotFound, msg)
}
