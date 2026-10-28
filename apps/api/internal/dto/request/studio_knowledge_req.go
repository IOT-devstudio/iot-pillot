package request

// StudioKnowledgeCreateReq 创建知识库条目（#196）。
//
// 不接收 author_id：创建人一律取自登录身份，防止「替别人署名」。
// permission 缺省按 admin（与迁移 0015 的列默认值一致）。
type StudioKnowledgeCreateReq struct {
	Title      string `json:"title" binding:"required"`
	Body       string `json:"body"`
	Permission string `json:"permission"`
	ParentID   *int   `json:"parent_id"`
}

// StudioKnowledgeUpdateReq 保存知识库条目：补丁语义，只传要改的项。
//
// Body 用指针而不是 string：正文可以是空串（先建骨架后补内容），
// 但「没传」与「传了空串」必须能区分——否则每次保存都会把正文清空。
type StudioKnowledgeUpdateReq struct {
	Title      *string `json:"title"`
	Body       *string `json:"body"`
	Permission *string `json:"permission"`
	// ParentID 三态：键不出现 = 不改；传 null = 移到顶层；传数字 = 挂到该条目下。
	// 与成员档案的 user_id 同一个坑：**int 会把 null 与「没传」解成同一个 nil，
	// 「移到顶层」于是被静默忽略。必须用 OptionalInt。
	ParentID OptionalInt `json:"parent_id"`
	// Version 乐观锁：调用方带上看过的版本号。
	//
	// 不传（0）时服务层退化成「用当前版本」，即总是覆盖——
	// 管理端页面必须传，否则并发编辑会静默丢改动。
	Version int `json:"version"`
}
