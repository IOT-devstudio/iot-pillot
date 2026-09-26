package response

import "github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

type LoginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
}

// RefreshResp 刷新令牌的响应。
//
// 刻意**不含 user_id**：刷新只轮换令牌，不解出用户身份。
// 原实现复用了 LoginResp 并塞进 user_id=-1 当哨兵值，前端一旦照抄就会把 -1
// 写进用户状态（类型上是合法的 number，看不出是假的）。
type RefreshResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// UserProfileDetail 用户资料明细（本人 GET/PUT /me 与管理员只读列表共用）。
//
// 空串 / 0 表示「未填」：domain.Detail 是值类型，GORM 落库也是零值而非 NULL，
// 前端（apps/web/src/api/profile.ts）已按同一口径处理。
type UserProfileDetail struct {
	Class     string `json:"class"`
	StudentID int    `json:"student_id"`
	QQ        string `json:"qq"`
	Direction string `json:"direction"`
	Email     string `json:"email"`
}

// NewUserProfileDetail 从领域 Detail 构造资料明细。
//
// 显式逐字段映射而不是直接序列化 domain.User：Password 等实体字段
// 必须永远没有机会跟着 DTO 一起出去。
func NewUserProfileDetail(d domain.Detail) UserProfileDetail {
	return UserProfileDetail{
		Class:     d.Class,
		StudentID: d.StudentID,
		QQ:        d.QQ,
		Direction: string(d.Direction),
		Email:     d.Email,
	}
}

// MeResp 当前登录用户的身份与资料（GET /me 与 PUT /me 共用）。
//
// Role 由服务端**现查 Redis 名单**得到（不是令牌里的快照），
// 因此前端看到的可见性判断与服务端强制的 403 永远一致。
type MeResp struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	// Name 是姓名（domain.User.Name），与 username 同源但语义独立：
	// 前端个人设置页把它当只读展示项。
	Name   string            `json:"name"`
	Detail UserProfileDetail `json:"detail"`
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
	// Detail 只读资料（issue #57）：管理员能看到完整信息，但改不了——
	// 编辑入口只有本人的 PUT /me。
	Detail UserProfileDetail `json:"detail"`
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
