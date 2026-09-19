package request

// AdminUserIDReq 管理端针对单个用户的操作（提升/撤销管理员等）。
type AdminUserIDReq struct {
	UserID int `json:"user_id" binding:"required,min=1"`
}
