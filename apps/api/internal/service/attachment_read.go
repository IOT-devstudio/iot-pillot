package service

import (
	"context"
	"errors"
	"io"
	"regexp"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// attachmentRefPattern 正文里引用附件的语法（见 ADR 0004 §2）。
//
// 只匹配本站前缀 /api/v1/attachments/<数字>，外链图片不参与绑定。
// 绑定真源是正文，服务端解析（ADR 0004 §3）。
var attachmentRefPattern = regexp.MustCompile(`/api/v1/attachments/(\d+)`)

// ExtractAttachmentIDs 从正文提取被引用的附件 id，去重、保序。
func ExtractAttachmentIDs(body string) []int {
	matches := attachmentRefPattern.FindAllStringSubmatch(body, -1)
	seen := make(map[int]bool, len(matches))
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		id := atoiSafe(m[1])
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// RecomputeBindings 按正文引用重算某内容行的附件绑定。
//
// col 来自固定的 scope→列 映射（post_id / reply_id / roadmap_id），非用户输入，安全。
// 校验失败（id 不存在 / 非本人上传 / 已被别的内容占用）整次失败，由上层 service
// 映射成 400（ADR 0004 §3：精确错误让用户改正文，不静默忽略）。
func (uc *AttachmentUseCase) RecomputeBindings(ctx context.Context, col string, ownerID int64, authorID int, body string) error {
	ids := ExtractAttachmentIDs(body)
	return uc.repo.BindForBody(ctx, col, ownerID, authorID, ids)
}

// atoiSafe 把十进制串解析成正数，解析失败或 <=0 返回 0（由调用方当作「非法 id」忽略）。
func atoiSafe(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 1<<30 {
			return 0
		}
	}
	return n
}

// AttachmentScopeResolver 判定附件归属 scope。
//
// #217 阶段只有孤儿 / 路线能判定，post / reply 绑定的判定（依赖 forum 表）留给
// #218 / #223。默认实现解析到位后，#218 通过 AttachmentUseCase.SetScopeResolver
// 换成真实现。这样绑定真源（#217 的正则重算）与判定真源（业务表）职责不混。
type AttachmentScopeResolver interface {
	// Resolve 返回附件的 scope 与（若是 studio）是否需要管理员。
	//
	// 返回空串 "" 表示尚无法判定（forum 表未建），service 按 404 处理，
	// 与 ADR 0004 §1「未判定一律 404」一致。
	Resolve(ctx context.Context, att *domain.Attachment) (scope string, needAdmin bool, err error)
}

// defaultScopeResolver #217 阶段的默认实现。
//
// 孤儿 → orphan（仅上传者）；路线 → roadmap（登录即可）；
// post / reply 列非空 → 返回 ""（未判定，等 #218 注入真实现）。
type defaultScopeResolver struct{}

func (defaultScopeResolver) Resolve(_ context.Context, att *domain.Attachment) (string, bool, error) {
	switch {
	case att.RoadmapID != nil:
		return domain.AttachmentScopeRoadmap, false, nil
	case att.PostID != nil || att.ReplyID != nil:
		// #217 阶段论坛表还没建，无法区分 recruitment / studio → 未判定。
		return "", false, nil
	default:
		return domain.AttachmentScopeOrphan, false, nil
	}
}

// GetAttachment 读取一张图（#217）。
//
// 归属判定在 service 层（见 ADR 0004 §1）：先取行，再让 resolver 给出 scope；
// 未判定（""）→ 404；studio → 需管理员，非管理员 → 404（防枚举）；
// 孤儿 → 仅上传者；recruitment / roadmap → 登录即可。
// 错误一律 404 输出（NotFound / Forbidden 都压成 404）。
func (uc *AttachmentUseCase) GetAttachment(ctx context.Context, viewerID int, isAdmin bool, attachmentID int) (*domain.Attachment, error) {
	att, err := uc.repo.Get(ctx, attachmentID)
	if err != nil {
		if errors.Is(err, domain.ErrAttachmentNotFound) || errors.Is(err, repository.ErrAttachmentNotFound) {
			return nil, domain.ErrAttachmentNotFound
		}
		return nil, err
	}

	scope, needAdmin, err := uc.scopeResolver.Resolve(ctx, att)
	if err != nil {
		return nil, err
	}
	// 未判定：#217 阶段论坛表未建，按 404 处理（不泄漏资源存在）。
	if scope == "" {
		return nil, domain.ErrAttachmentNotFound
	}

	switch scope {
	case domain.AttachmentScopeOrphan:
		// 仅上传者本人。
		if att.UploaderID != viewerID {
			return nil, domain.ErrAttachmentForbidden
		}
	case domain.AttachmentScopeStudio:
		// 需管理员；非管理员一律 404（防枚举）。
		if !isAdmin {
			return nil, domain.ErrAttachmentForbidden
		}
	case domain.AttachmentScopeRecruitment, domain.AttachmentScopeRoadmap:
		// 登录即可，鉴权在中间件已保证。
	}

	// needAdmin 兜底（studio 场景由 resolver 显式返回）。非管理员 404。
	if needAdmin && !isAdmin {
		return nil, domain.ErrAttachmentForbidden
	}

	return att, nil
}

// ReadObject 代理读取对象字节（handler 在判定通过后调用）。
func (uc *AttachmentUseCase) ReadObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	return uc.storage.GetObject(ctx, objectKey)
}
