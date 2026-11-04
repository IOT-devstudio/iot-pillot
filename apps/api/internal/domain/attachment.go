package domain

import (
	"errors"
	"time"
)

// Attachment 内容附件（图片）。
//
// #216 只落「孤儿上传」所需的基础列；#217 补上三列归属绑定（post_id / reply_id /
// roadmap_id）：孤儿态即三列全空，绑定到招新问答 / 内部论坛 / 学习路线时由对应
// 业务表（#218 / #223）在保存正文时按正则 /api/v1/attachments/(\d+) 回填。
type Attachment struct {
	ID          int       `json:"id" gorm:"primaryKey"`
	ObjectKey   string    `json:"object_key" gorm:"column:object_key"`
	UploaderID  int       `json:"uploader_id" gorm:"column:uploader_id"`
	ContentType string    `json:"content_type" gorm:"column:content_type"`
	SizeBytes   int64     `json:"size_bytes" gorm:"column:size_bytes"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`

	// 三列归属绑定：至多一列非空（迁移 CHECK num_nonnulls <= 1）。
	// 用 *int64 以区分「未绑定（NULL）」与「绑定到 id=0」（不会发生）。
	PostID    *int64 `json:"-" gorm:"column:post_id"`
	ReplyID   *int64 `json:"-" gorm:"column:reply_id"`
	RoadmapID *int64 `json:"-" gorm:"column:roadmap_id"`
}

// TableName 显式指定表名。
//
// 仓内没有全局 GORM naming strategy，GORM 默认会把 Attachment 复数化成
// "attachments"，与迁移里的 content_attachments 不符——那会让真实读写打到
// 不存在的表。显式写死，消除歧义（#216 的测试全走内存桩，没暴露这个坑）。
func (Attachment) TableName() string { return "content_attachments" }

// 附件归属 scope：决定「谁能读这张图」。
//
// 孤儿态只给上传者；studio 需管理员；recruitment / roadmap 登录即可。
// 空串 "" 表示 #217 阶段尚无法判定（forum 表未建），service 一律按 404 处理。
const (
	AttachmentScopeOrphan      = "orphan"      // 未绑定：仅上传者本人
	AttachmentScopeStudio      = "studio"      // 内部论坛：需管理员
	AttachmentScopeRecruitment = "recruitment" // 招新问答：登录即可
	AttachmentScopeRoadmap     = "roadmap"     // 学习路线：登录即可
)

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
	// ErrAttachmentNotFound 附件不存在（#217 读取端点用它区分 404 与 500）。
	ErrAttachmentNotFound = errors.New("附件不存在")
	// ErrAttachmentForbidden 无权访问该附件（孤儿非上传者 / studio 非管理员 / 未判定）。
	//
	// 与 NotFound 一样在 handler 层被压成 404：不泄漏「这个 id 存在但你没权限」，
	// 避免被用来枚举内部论坛的内容规模（见 ADR 0004 §1）。
	ErrAttachmentForbidden = errors.New("附件不可访问")
)
