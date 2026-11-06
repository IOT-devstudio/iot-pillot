package response

import "github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

// LoginResp 登录响应。
//
// 冻结分支（0012 自助解冻）不再返回 403，而是照常给令牌，但 scope 是
// profile_only 且 Frozen=true：用户拿这张受限令牌只能补资料，补齐后自动解冻。
// 不这么做就会死锁——冻结即 403 → 拿不到令牌 → 永远调不了补资料的 PUT /me。
type LoginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
	// Scope 令牌范围：full（完整）/ profile_only（仅补资料）。
	// 前端据此决定进主界面还是进补资料页。
	Scope string `json:"scope"`
	// Frozen 是否因资料不合规处于冻结状态。
	Frozen bool `json:"frozen"`
	// FreezeIssues 待补的资料项，仅在 Frozen 为真时非空。
	// 一次给全部而不是首条：让用户一次改完，而不是改一项再被拦一次。
	FreezeIssues []FreezeIssue `json:"freeze_issues,omitempty"`
}

// FreezeIssue 一条待补的资料项。
type FreezeIssue struct {
	// Field 字段名（name / class / student_id），前端据此定位输入框。
	Field string `json:"field"`
	// Message 可读的整改要求。
	Message string `json:"message"`
}

// RefreshResp 刷新令牌的响应。
//
// 刻意**不含 user_id**：刷新只轮换令牌，不解出用户身份。
// 原实现复用了 LoginResp 并塞进 user_id=-1 当哨兵值，前端一旦照抄就会把 -1
// 写进用户状态（类型上是合法的 number，看不出是假的）。
type RefreshResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// Scope 保持原令牌范围：受限令牌刷新后仍是受限令牌，
	// 否则冻结账号刷新一次就拿到了完整权限。
	Scope string `json:"scope"`
}

// UnfreezeSelfResp 本人自助解冻的结果（0012）。
//
// 解冻成功时同时换发完整权限令牌：用户不必重新登录，
// 也避免「刚解冻但手里还是受限令牌」这种半套状态。
type UnfreezeSelfResp struct {
	UserID       int    `json:"user_id"`
	Frozen       bool   `json:"frozen"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	// FreezeIssues 解冻失败时（资料仍不合规）返回剩余问题，此时令牌字段为空。
	FreezeIssues []FreezeIssue `json:"freeze_issues,omitempty"`
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
	// IsFounding 创始管理员标记（issue #120）：ADMIN_USERS 启动种入的账号为 true，
	// 管理端晋升与普通成员为 false。仅作前端展示分流（隐藏个人设置入口、
	// 抑制补全提示），不参与权限判定；GET 与 PUT /me 都必须返回——
	// 前端保存资料后会用 PUT 响应覆盖本地 profile，缺字段会让补全提示复活。
	IsFounding bool `json:"is_founding"`
	// Name 是姓名（domain.User.Name），与 username 同源但语义独立：
	// 前端个人设置页把它当只读展示项。
	Name   string            `json:"name"`
	Detail UserProfileDetail `json:"detail"`
	// 自动解冻相关（0012 自助解冻）：
	//
	// 受限会话在 PUT /me 补齐资料后，后端当场解冻并换发完整令牌，
	// 这三个字段只有在**本次请求真的解冻了**时才非空/为真。
	// 其余情况一律省略，前端据此决定是否替换本地会话——
	// 无条件替换会让每次保存资料都重置会话，把用户其它标签页顶下线。
	Scope        string `json:"scope,omitempty"`
	Unfrozen     bool   `json:"unfrozen,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
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
	// IsDeleted 账号是否已注销（issue #142 Q3：历史保留，列表标注）。
	// false = 正常账号；true = 软删行，前端显示「账号已注销」。
	IsDeleted bool `json:"is_deleted"`
	// IsFrozen 账号是否因资料不合规被冻结（0012）。
	// 列表默认不返回冻结行，只有显式 include_frozen=true 时才会出现 true。
	IsFrozen bool `json:"is_frozen"`
	// FrozenAt 冻结时刻（RFC3339），未冻结为空串。
	FrozenAt string `json:"frozen_at"`
	// FreezeReason 冻结原因快照（例如「班级须为「汉字 + 6 位数字」」），未冻结为空串。
	FreezeReason string `json:"freeze_reason"`
	// Detail 只读资料（issue #57）：管理员能看到完整信息，但改不了——
	// 编辑入口只有本人的 PUT /me。
	Detail UserProfileDetail `json:"detail"`
}

// AdminUnfreezeResp 解冻账号的结果（0012）。
type AdminUnfreezeResp struct {
	UserID int `json:"user_id"`
	// IsFrozen 恒为 false：解冻成功后回显目标状态，前端据此就地更新行。
	IsFrozen bool `json:"is_frozen"`
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
