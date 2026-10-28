package request

// StudioProjectCreateReq 创建工作室项目（#198）。
//
// status 可选：缺省按 active（与迁移 0014 的列默认值一致）。
type StudioProjectCreateReq struct {
	Name    string `json:"name" binding:"required"`
	Year    string `json:"year" binding:"required"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

// StudioProjectUpdateReq 更新项目元数据：只传要改的项。
type StudioProjectUpdateReq struct {
	Name    *string `json:"name"`
	Year    *string `json:"year"`
	Summary *string `json:"summary"`
	Status  *string `json:"status"`
}

// StudioProjectMemberAddReq 为项目添加参与。
//
// name 是**姓名快照**必填：不从成员档案自动取，这样成员以后改名
// 历史参与仍显示当时的名字，同时外部参与者（没有成员档案）也能记录。
// started_at 可选：缺省为现在。
type StudioProjectMemberAddReq struct {
	MemberID  *int    `json:"member_id"`
	Name      string  `json:"name" binding:"required"`
	Role      string  `json:"role"`
	StartedAt *string `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
}

// StudioProjectMemberUpdateReq 更新参与行：只传要改的项。
//
// ended_at 三态：键不出现 = 不改；null = 仍在参与；ISO 时间 = 结束于该时刻。
// 用 OptionalString（与 OptionalInt 同理）区分「没传」与「传了 null」。
type StudioProjectMemberUpdateReq struct {
	Name      *string        `json:"name"`
	Role      *string        `json:"role"`
	MemberID  OptionalInt    `json:"member_id"`
	StartedAt *string        `json:"started_at"`
	EndedAt   OptionalString `json:"ended_at"`
}
