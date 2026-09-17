package response

type LoginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
}

// MeResp 当前登录用户的身份。
//
// 角色由令牌携带（来源是配置里的管理员白名单），不落库，所以这里不回显任何
// 用户资料字段——前端只需要知道"我是谁、我能不能进管理端"。
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
}

// AdminUserListResp 管理端用户列表分页结果。
type AdminUserListResp struct {
	Items    []AdminUserResp `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}
