package request

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterReq struct {
	Name     string `json:"name" binding:"required,min=2,max=20"`
	Password string `json:"password" binding:"required,min=6,max=20"`
	Email    string `json:"email" binding:"required,email,max=50"`
	Code     string `json:"code" binding:"required,min=6,max=6"`
}

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type VerifyCodeReq struct {
	VerifierType string `json:"verifier_type" binding:"required,oneof=email phone"`
	Verifier     string `json:"verifier" binding:"required,min=6,max=50"`
	// Purpose 用途（issue #84）：register | reset。
	//
	// 不带的旧请求一律按 register 处理，保证既有前端不改也能继续注册。
	Purpose string `json:"purpose" binding:"omitempty,oneof=register reset"`
}

// ResetCodeReq 发送找回密码验证码的请求（issue #84）。
//
// 刻意不复用 VerifyCodeReq：那个 DTO 的 verifier_type 允许 phone，
// 而找回密码只走已绑定邮箱（phone 验证码后端未实现）。
// 复用会让 verifier_type=phone 被静默按 email 处理，契约与实现脱节。
type ResetCodeReq struct {
	Email string `json:"email" binding:"required,email,max=50"`
}

// ResetPasswordReq 通过邮箱验证码重置密码（issue #84）。
type ResetPasswordReq struct {
	Email       string `json:"email" binding:"required,email,max=50"`
	Code        string `json:"code" binding:"required,min=6,max=6"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
}

// ChangePasswordReq 登录状态下修改密码（issue #84）。
type ChangePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
}
