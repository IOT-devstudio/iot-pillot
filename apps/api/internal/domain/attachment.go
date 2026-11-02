package domain

import (
	"errors"
	"time"
)

// Attachment 内容附件（图片）。
//
// #216 只落「孤儿上传」所需的基础列；三列归属绑定（post_id / reply_id /
// roadmap_id）由 #217 的迁移补上，那时再扩这个结构体。
type Attachment struct {
	ID          int       `json:"id" gorm:"primaryKey"`
	ObjectKey   string    `json:"object_key" gorm:"column:object_key"`
	UploaderID  int       `json:"uploader_id" gorm:"column:uploader_id"`
	ContentType string    `json:"content_type" gorm:"column:content_type"`
	SizeBytes   int64     `json:"size_bytes" gorm:"column:size_bytes"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// 附件相关的领域错误。
//
// 放在 domain 而不是 service：它们描述的是「附件本身合不合法」，
// handler 与 service 都要能引用同一套哨兵。
var (
	// ErrAttachmentInvalid 附件整体不合法。
	ErrAttachmentInvalid = errors.New("附件不合法")
	// ErrAttachmentTooLarge 超过单张图片的大小上限。
	ErrAttachmentTooLarge = errors.New("单张图片不得超过 10MiB")
	// ErrAttachmentUnsupportedType 不是 jpg / png / webp。
	ErrAttachmentUnsupportedType = errors.New("只支持 jpg / png / webp 图片")
	// ErrAttachmentQuotaExceeded 同一账号在窗口内达到上传次数上限。
	ErrAttachmentQuotaExceeded = errors.New("上传次数已达上限，请稍后重试")
	// ErrRequestTooLarge 请求体超过单请求上限。
	ErrRequestTooLarge = errors.New("请求体过大")
)
