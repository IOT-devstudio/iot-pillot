package response

// MailModelResp 邮件模板。
type MailModelResp struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	MailExample string `json:"mail_example"`
	MailModel   string `json:"mail_model"`
	// Variables 模板里用到的变量名，管理端据此提示"这封模板需要哪些变量"
	Variables []string `json:"variables"`
}

// MailModelListResp 模板列表。
type MailModelListResp struct {
	Items []MailModelResp `json:"items"`
	Total int             `json:"total"`
}

// MailSendResp 单封发送成功的结果。
type MailSendResp struct {
	ToUserID int    `json:"to_user_id"`
	ToEmail  string `json:"to_email"`
	// Title 是渲染之后的实际主题（不是模板原文），便于确认变量替换是否正确
	Title string `json:"title"`
}

// MailSendFailure 单封发送失败的原因。
type MailSendFailure struct {
	ToUserID int    `json:"to_user_id"`
	ToEmail  string `json:"to_email"`
	Reason   string `json:"reason"`
}

// MailBulkSendResp 群发结果。
//
// 逐条返回成败而不是"要么全成要么全败"：SMTP 是按收件人逐个投递的，
// 第 3 个人邮箱写错不该让前 2 封已发出去的邮件变成"失败"。
type MailBulkSendResp struct {
	Sent   []MailSendResp    `json:"sent"`
	Failed []MailSendFailure `json:"failed"`
	Total  int               `json:"total"`
}

// MailRecordResp 发信记录。
type MailRecordResp struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	FromUserID int    `json:"from_user_id"`
	ToUserID   int    `json:"to_user_id"`
	ToEmail    string `json:"to_email"`
	CreatedAt  string `json:"created_at"`
}

// MailRecordListResp 发信记录分页结果。
type MailRecordListResp struct {
	Items    []MailRecordResp `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// MailBatchDeleteResp 批量删除结果。Deleted 是实际删掉的行数：
// ids 里混入已不存在的 id 时会小于请求数量，让调用方能如实转告。
type MailBatchDeleteResp struct {
	Deleted int64 `json:"deleted"`
}
