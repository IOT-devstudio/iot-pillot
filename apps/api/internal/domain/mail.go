package domain

import "time"

// MailModel 邮件模板。
//
// 一份模板由「主题 + 正文」两段组成，两段都支持 {{变量}} 占位符；
// MailExample 是填好变量后的效果示例，仅用于管理端预览，不参与发送。
type MailModel struct {
	ID          int    `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"column:name;uniqueIndex"`
	Type        string `json:"type" gorm:"column:type;uniqueIndex"`
	Title       string `json:"title" gorm:"column:title"`
	MailExample string `json:"mail_example" gorm:"column:mail_example"`
	MailModel   string `json:"mail_model" gorm:"column:mail_model"`
}

// Mail 发信记录。
//
// FromUserID 是操作者（管理端那个点发送的人），ToUserID 是收件人。
// 按邮箱直发且对方未注册时 ToUserID 为 0，此时收件地址落在 ToEmail ——
// 两种收件人各占一列，语义不重叠，不会出现"两个字段都可能装邮箱"的歧义。
type Mail struct {
	ID         int       `json:"id" gorm:"primaryKey"`
	Title      string    `json:"title" gorm:"column:title"`
	Content    string    `json:"content" gorm:"column:content"`
	FromUserID int       `json:"from_user_id" gorm:"column:from_user_id"`
	ToUserID   int       `json:"to_user_id" gorm:"column:to_user_id"`
	ToEmail    string    `json:"to_email" gorm:"column:to_email"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}
