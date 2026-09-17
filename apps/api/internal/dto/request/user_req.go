package request

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterReq struct {
	Name     string `json:"name" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=6,max=20"`
	Email    string `json:"email" binding:"required,email,max=50"`
	Code     string `json:"code" binding:"required,min=6,max=6"`
}

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// VerifyCodeReq 发送验证码请求。
// Verifier 上界取 50，与 RegisterReq.Email 的 max=50 对齐：
// 之前是 max=20，超过 20 字符的邮箱（如 zhangsan@university.edu.cn）会在
// 绑定阶段就被拒，导致注册流程拿不到验证码。
type VerifyCodeReq struct {
	VerifierType string `json:"verifier_type" binding:"required,oneof=email phone"`
	Verifier     string `json:"verifier" binding:"required,min=6,max=50"`
}
