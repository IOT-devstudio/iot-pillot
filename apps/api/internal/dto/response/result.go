package response

// Result 统一响应体
type Result struct {
	Code    int    `json:"code"`           // 业务码：0 成功，非 0 失败
	Message string `json:"message"`        // 提示信息
	Data    any    `json:"data,omitempty"` // 业务数据
}

// 业务码常量
const (
	CodeSuccess      = 0    // 成功
	CodeInvalidParam = 4001 // 参数错误
	CodeUnauthorized = 4002 // 未认证 / token 失效
	CodeForbidden    = 4003 // 无权限
	CodeServerError  = 5000 // 服务器内部错误
)

// Success 构造成功响应
func Success(data any) *Result {
	return &Result{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	}
}

// Error 构造失败响应
func Error(code int, msg string) *Result {
	return &Result{
		Code:    code,
		Message: msg,
	}
}
