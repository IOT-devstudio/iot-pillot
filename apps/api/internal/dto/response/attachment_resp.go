package response

import (
	"fmt"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

// AttachmentUploadResp 附件上传成功的响应。
//
// URL 是**相对**地址（/api/v1/attachments/{id}），前端渲染正文时直接可用，
// 换域名或加网关都不用改正文里的引用。
type AttachmentUploadResp struct {
	ID          int    `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

// NewAttachmentUploadResp 领域实体 → DTO。
//
// 刻意逐字段映射而不是透传 domain.Attachment：将来 Attachment 加了内部字段
// （比如 object_key），不会悄悄暴露给前端。
func NewAttachmentUploadResp(att *domain.Attachment) AttachmentUploadResp {
	return AttachmentUploadResp{
		ID:          att.ID,
		URL:         fmt.Sprintf("/api/v1/attachments/%d", att.ID),
		ContentType: att.ContentType,
		SizeBytes:   att.SizeBytes,
	}
}
