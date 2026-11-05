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
// 绑定真源是正文正则重算（#217），判定真源是业务表（帖子/路线），两者职责不混。
type AttachmentScopeResolver interface {
	// Resolve 返回附件的 scope 与（若是 studio）是否需要管理员。
	//
	// 返回空串 "" 表示尚无法判定，service 按 404 处理，
	// 与 ADR 0004 §1「未判定一律 404」一致。
	Resolve(ctx context.Context, att *domain.Attachment) (scope string, needAdmin bool, err error)
}

// defaultScopeResolver 未注入真实现时的兜底。
//
// 孤儿 → orphan（仅上传者）；路线 → roadmap（登录即可）；
// post / reply 列非空 → 返回 ""（未判定 → 404），绝不放行。
type defaultScopeResolver struct{}

func (defaultScopeResolver) Resolve(_ context.Context, att *domain.Attachment) (string, bool, error) {
	switch {
	case att.RoadmapID != nil:
		return domain.AttachmentScopeRoadmap, false, nil
	case att.PostID != nil || att.ReplyID != nil:
		// 未注入 forum 判定时不可放行：宁可 404，也不能让内部论坛的图被读到。
		return "", false, nil
	default:
		return domain.AttachmentScopeOrphan, false, nil
	}
}

// forumScopeResolver 基于 forum 表的真实归属判定（#221 补齐 #217 的占位）。
//
// 规则（ADR 0004 §1）：
//   - 绑定到帖子/回复 → 取该帖 scope：studio 需管理员，recruitment 登录即可；
//   - 帖子或回复已软删 → 一律 404（返回 ""，由 GetAttachment 压成 ErrAttachmentNotFound）；
//   - 绑定的行已不存在（数据不一致）→ 同样 404，不放行。
type forumScopeResolver struct {
	posts repository.ForumRepo
}

// NewForumScopeResolver 构造真实判定器，供 AttachmentUseCase.SetScopeResolver 注入。
func NewForumScopeResolver(posts repository.ForumRepo) AttachmentScopeResolver {
	return &forumScopeResolver{posts: posts}
}

func (r *forumScopeResolver) Resolve(ctx context.Context, att *domain.Attachment) (string, bool, error) {
	// 路线优先判定（与 defaultScopeResolver 同序，保证迁移期行为一致）。
	if att.RoadmapID != nil {
		return domain.AttachmentScopeRoadmap, false, nil
	}

	if att.PostID != nil {
		scope, deleted, err := r.posts.LookupPostScope(ctx, int(*att.PostID))
		if err != nil {
			// 帖子不存在 → 视为不可见（404）。其它错误向上抛，不能当成 404 掩盖故障。
			if errors.Is(err, repository.ErrForumPostNotFound) {
				return "", false, nil
			}
			return "", false, err
		}
		if deleted {
			return "", false, nil
		}
		return forumScopeName(scope), true, nil
	}

	if att.ReplyID != nil {
		scope, deleted, err := r.posts.LookupReplyPostScope(ctx, int(*att.ReplyID))
		if err != nil {
			if errors.Is(err, repository.ErrForumReplyNotFound) {
				return "", false, nil
			}
			return "", false, err
		}
		if deleted {
			return "", false, nil
		}
		return forumScopeName(scope), true, nil
	}

	return domain.AttachmentScopeOrphan, false, nil
}

// forumScopeName 把内容域映射成附件判定的 scope 常量。
//
// needAdmin 的语义在 GetAttachment 里按 scope 分支判定（studio 才要管理员），
// 这里返回的 bool 只是让调用方在日志/测试里能看出「这是论坛内容」。
func forumScopeName(scope domain.ForumScope) string {
	if scope == domain.ForumScopeStudio {
		return domain.AttachmentScopeStudio
	}
	return domain.AttachmentScopeRecruitment
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
