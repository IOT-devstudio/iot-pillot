package request

// MailTemplateReq 新建 / 更新邮件模板。
//
// Title 与 MailModel 都支持 {{变量}} 占位符；MailExample 只是给人看的示例，
// 不参与发送（可在管理端填一份"填好变量后的效果"便于预览）。
type MailTemplateReq struct {
	Name        string `json:"name" binding:"required,min=1,max=50"`
	Type        string `json:"type" binding:"required,min=1,max=30"`
	Title       string `json:"title" binding:"required,min=1,max=200"`
	MailExample string `json:"mail_example" binding:"max=5000"`
	MailModel   string `json:"mail_model" binding:"required,min=1,max=20000"`
}

// MailVars 模板变量：键是模板里的占位符名，值是替换内容。
//
// 不传也没关系：服务端会先用收件人的资料（姓名 / 邮箱 / 班级 / 方向 / 学号 /
// 轮次）预填一份，调用方传进来的同键覆盖预填值。
// 这正是「群发同一个模板、逐个替换内容」最常见的用法——多数情况下一个变量都不用传。
type MailVars map[string]string

// SendMailToUserReq 按 userID 单发。
type SendMailToUserReq struct {
	TemplateID int      `json:"template_id" binding:"required,min=1"`
	ToUserID   int      `json:"to_user_id" binding:"required,min=1"`
	Vars       MailVars `json:"vars"`
}

// SendMailToEmailReq 按邮箱单发（收件人还没注册时用）。
type SendMailToEmailReq struct {
	TemplateID int      `json:"template_id" binding:"required,min=1"`
	Email      string   `json:"email" binding:"required,email,max=50"`
	Vars       MailVars `json:"vars"`
}

// MailRecipientReq 群发里的单个收件人。
type MailRecipientReq struct {
	ToUserID int      `json:"to_user_id" binding:"required,min=1"`
	Vars     MailVars `json:"vars"`
}

// SendMailBulkReq 群发：同一个模板，逐个收件人各自的替换内容。
type SendMailBulkReq struct {
	TemplateID int                `json:"template_id" binding:"required,min=1"`
	Recipients []MailRecipientReq `json:"recipients" binding:"required,min=1,dive"`
}
