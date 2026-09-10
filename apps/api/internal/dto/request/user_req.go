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

type VerifyCodeReq struct {
	VerifierType string `json:"verifier_type" binding:"required,oneof=email phone"`
	Verifier     string `json:"verifier" binding:"required,min=6,max=20"`
}
