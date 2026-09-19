package response

type LoginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
}

// MeResp 当前登录用户的身份。
//
// 角色真源是 Redis 里的管理员名单；JWT 也带一份 role 作为前端可见性提示，
// 但服务端强制 403 时读的是 Redis，所以撤销管理员会立即生效。
type MeResp struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// AdminUserResp 管理端用户列表项。
//
// 刻意**不含 Password 字段**：即使 repository 返回的是带哈希的完整实体，
// 也不能让它有机会被序列化出去。
type AdminUserResp struct {
	UserID    int    `json:"user_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	// IsAdmin 由 Redis 名单现算，方便管理端直接标注"当前是不是管理员"
	IsAdmin bool `json:"is_admin"`
}

// AdminUserListResp 管理端用户列表分页结果。
type AdminUserListResp struct {
	Items    []AdminUserResp `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// AdminListResp 管理员名单。
type AdminListResp struct {
	Items []AdminUserResp `json:"items"`
	Total int             `json:"total"`
}

// AdminMutationResp 提升/撤销管理员的结果。
//
// 回显 user_id 与变更后的 is_admin，并要求前端在拿到后重新登录：
// 角色写在令牌里，旧令牌的 claim 已经过时。
type AdminMutationResp struct {
	UserID         int  `json:"user_id"`
	IsAdmin        bool `json:"is_admin"`
	SessionRevoked bool `json:"session_revoked"`
}
