package request

// StudioMemberCreateReq 录入工作室成员（#195）。
//
// user_id 可选：缺省 / null = 无系统账号的成员（校友、外部人员）。
// 不接收 is_active / archived_at：新建成员一律是在读状态，
// 归档只能走 POST /admin/studio/members/:id/archive。
type StudioMemberCreateReq struct {
	Name         string `json:"name" binding:"required"`
	BirthYear    int    `json:"birth_year" binding:"required"`
	Cohort       string `json:"cohort" binding:"required"`
	Direction    string `json:"direction" binding:"required"`
	RoleInStudio string `json:"role_in_studio" binding:"required"`
	Internship   string `json:"internship"`
	Employment   string `json:"employment"`
	UserID       *int   `json:"user_id"`
}

// StudioMemberUpdateReq 更新工作室成员：只传要改的项。
//
// 与 Create 分开：更新是补丁语义，用指针区分「没传」与「传了空串」——
// 后者必须被服务端当成非法输入拒绝（姓名/届别/方向/职能都是必填）。
type StudioMemberUpdateReq struct {
	Name         *string `json:"name"`
	BirthYear    *int    `json:"birth_year"`
	Cohort       *string `json:"cohort"`
	Direction    *string `json:"direction"`
	RoleInStudio *string `json:"role_in_studio"`
	Internship   *string `json:"internship"`
	Employment   *string `json:"employment"`
	// UserID 三态：键不出现 = 不改；传 null = 解除绑定；传数字 = 绑定该账号。
	// 必须用 OptionalInt —— **int 会把 null 与「没传」解成同一个 nil，
	// 「解除绑定」于是被静默忽略（见 optional_int.go 的说明）。
	UserID OptionalInt `json:"user_id"`
	// IsActive 三态：不传 = 不改；true = 恢复在读（顺带清归档时刻）。
	IsActive *bool `json:"is_active"`
}
