package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * #221：已解决标记 + 附件归属真实现
 * ------------------------------------------------------------------ */

// stubResolveRepo 只实现本组用例需要的方法。
type stubResolveRepo struct {
	repository.ForumRepo

	post     *domain.ForumPost
	postErr  error
	setCalls []struct {
		ID       int
		By       int
		Resolved bool
	}
	setErr error

	// GetPost 被调用两次（判定前 + 写后回读），用游标让回读拿到「已更新」的行。
	getCount int
}

func (s *stubResolveRepo) GetPost(_ context.Context, _ int) (*domain.ForumPost, error) {
	if s.postErr != nil {
		return nil, s.postErr
	}
	s.getCount++
	// 第二次读（回读）返回一份已置 resolved 的副本，验证响应带真实状态。
	if s.getCount >= 2 && s.post != nil {
		cp := *s.post
		by := 0
		if len(s.setCalls) > 0 && s.setCalls[len(s.setCalls)-1].Resolved {
			by = s.setCalls[len(s.setCalls)-1].By
		}
		if by != 0 {
			cp.ResolvedBy = &by
		}
		return &cp, nil
	}
	return s.post, nil
}

func (s *stubResolveRepo) SetPostResolved(_ context.Context, id int, by int, resolved bool) error {
	s.setCalls = append(s.setCalls, struct {
		ID       int
		By       int
		Resolved bool
	}{id, by, resolved})
	return s.setErr
}

func newResolveUseCase(repo *stubResolveRepo, admins utils.AdminDirectory) *ForumUseCase {
	return NewForumUseCase(repo, admins)
}

func memberPost(authorID int) *domain.ForumPost {
	return &domain.ForumPost{ID: 7, AuthorID: authorID, Scope: domain.ForumScopeRecruitment, Title: "t", Body: "b"}
}

func TestSetPostResolved_AuthorCanResolve(t *testing.T) {
	repo := &stubResolveRepo{post: memberPost(5)}
	uc := newResolveUseCase(repo, &stubAdminDirectory{})

	post, err := uc.SetPostResolved(context.Background(), 5, 7, true)
	if err != nil {
		t.Fatalf("作者标记已解决应成功，得到 %v", err)
	}
	if len(repo.setCalls) != 1 || !repo.setCalls[0].Resolved || repo.setCalls[0].By != 5 {
		t.Fatalf("应写入 resolved=true 且标记人=操作者，得到 %+v", repo.setCalls)
	}
	// 回读必须带出真实状态，前端才不用猜。
	if post.ResolvedBy == nil || *post.ResolvedBy != 5 {
		t.Fatalf("回读应带 resolved_by=5，得到 %+v", post.ResolvedBy)
	}
}

func TestSetPostResolved_MarksOperatorNotAuthor(t *testing.T) {
	// 管理员替别人的帖子标记：标记人应是管理员本人。
	repo := &stubResolveRepo{post: memberPost(5)}
	uc := newResolveUseCase(repo, &stubAdminDirectory{ids: []int{99}})

	if _, err := uc.SetPostResolved(context.Background(), 99, 7, true); err != nil {
		t.Fatalf("管理员应能标记他人帖子，得到 %v", err)
	}
	if repo.setCalls[0].By != 99 {
		t.Fatalf("标记人应为操作者 99，得到 %d", repo.setCalls[0].By)
	}
}

func TestSetPostResolved_NonAuthorNonAdminRejected(t *testing.T) {
	repo := &stubResolveRepo{post: memberPost(5)}
	uc := newResolveUseCase(repo, &stubAdminDirectory{ids: []int{77}})

	_, err := uc.SetPostResolved(context.Background(), 42, 7, true)
	// 与编辑/删除同口径：无权一律 404，不暴露存在性。
	if !errors.Is(err, ErrForumPostNotFound) {
		t.Fatalf("非作者非管理员应被拒为 404，得到 %v", err)
	}
	if len(repo.setCalls) != 0 {
		t.Fatalf("被拒时不应写库，得到 %+v", repo.setCalls)
	}
}

func TestSetPostResolved_CancelClearsState(t *testing.T) {
	repo := &stubResolveRepo{post: memberPost(5)}
	uc := newResolveUseCase(repo, &stubAdminDirectory{})

	if _, err := uc.SetPostResolved(context.Background(), 5, 7, false); err != nil {
		t.Fatalf("取消应成功，得到 %v", err)
	}
	if repo.setCalls[0].Resolved {
		t.Fatalf("应写入 resolved=false，得到 %+v", repo.setCalls[0])
	}
}

func TestSetPostResolved_SoftDeletedRejected(t *testing.T) {
	p := memberPost(5)
	now := time.Now()
	p.DeletedAt = &now
	repo := &stubResolveRepo{post: p}
	uc := newResolveUseCase(repo, &stubAdminDirectory{})

	if _, err := uc.SetPostResolved(context.Background(), 5, 7, true); !errors.Is(err, ErrForumPostNotFound) {
		t.Fatalf("软删帖不应可标记，得到 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 附件归属：studio 图普通成员读不到
 * ------------------------------------------------------------------ */

func TestForumScopeResolver_StudioNeedsAdmin(t *testing.T) {
	postID := int64(10)
	att := &domain.Attachment{ID: 1, UploaderID: 3, PostID: &postID}
	r := &stubScopeLookup{scope: domain.ForumScopeStudio, deleted: false}

	scope, _, err := NewForumScopeResolver(r).Resolve(context.Background(), att)
	if err != nil {
		t.Fatalf("解析不应报错，得到 %v", err)
	}
	if scope != domain.AttachmentScopeStudio {
		t.Fatalf("内部论坛帖子应判为 studio，得到 %q", scope)
	}
}

func TestForumScopeResolver_RecruitmentLoginEnough(t *testing.T) {
	postID := int64(11)
	att := &domain.Attachment{ID: 1, UploaderID: 3, PostID: &postID}
	r := &stubScopeLookup{scope: domain.ForumScopeRecruitment}

	scope, _, err := NewForumScopeResolver(r).Resolve(context.Background(), att)
	if err != nil || scope != domain.AttachmentScopeRecruitment {
		t.Fatalf("招新问答应判为 recruitment，得到 %q / %v", scope, err)
	}
}

func TestForumScopeResolver_DeletedPostIsUnresolved(t *testing.T) {
	postID := int64(12)
	att := &domain.Attachment{ID: 1, PostID: &postID}
	r := &stubScopeLookup{scope: domain.ForumScopeRecruitment, deleted: true}

	scope, _, err := NewForumScopeResolver(r).Resolve(context.Background(), att)
	if err != nil || scope != "" {
		// 空串 → GetAttachment 压成 404：软删帖的图不该还能读。
		t.Fatalf("软删帖应判为未判定(404)，得到 %q / %v", scope, err)
	}
}

func TestForumScopeResolver_MissingPostIsUnresolved(t *testing.T) {
	postID := int64(13)
	att := &domain.Attachment{ID: 1, PostID: &postID}
	r := &stubScopeLookup{err: repository.ErrForumPostNotFound}

	scope, _, err := NewForumScopeResolver(r).Resolve(context.Background(), att)
	if err != nil || scope != "" {
		t.Fatalf("帖子不存在应判为未判定(404)，得到 %q / %v", scope, err)
	}
}

func TestForumScopeResolver_RepoErrorPropagates(t *testing.T) {
	postID := int64(14)
	att := &domain.Attachment{ID: 1, PostID: &postID}
	boom := errors.New("db down")
	r := &stubScopeLookup{err: boom}

	// 真实故障不能被静默压成 404，否则排查时看不见。
	if _, _, err := NewForumScopeResolver(r).Resolve(context.Background(), att); !errors.Is(err, boom) {
		t.Fatalf("数据库错误应向上抛，得到 %v", err)
	}
}

func TestForumScopeResolver_ReplyInheritsPostScope(t *testing.T) {
	replyID := int64(20)
	att := &domain.Attachment{ID: 1, ReplyID: &replyID}
	r := &stubScopeLookup{scope: domain.ForumScopeStudio}

	scope, _, err := NewForumScopeResolver(r).Resolve(context.Background(), att)
	if err != nil || scope != domain.AttachmentScopeStudio {
		t.Fatalf("回复应继承帖子 scope（studio），得到 %q / %v", scope, err)
	}
}

func TestForumScopeResolver_OrphanAndRoadmap(t *testing.T) {
	r := NewForumScopeResolver(&stubScopeLookup{})

	scope, _, _ := r.Resolve(context.Background(), &domain.Attachment{ID: 1})
	if scope != domain.AttachmentScopeOrphan {
		t.Fatalf("未绑定应判为 orphan，得到 %q", scope)
	}

	roadmapID := int64(30)
	scope, _, _ = r.Resolve(context.Background(), &domain.Attachment{ID: 2, RoadmapID: &roadmapID})
	if scope != domain.AttachmentScopeRoadmap {
		t.Fatalf("路线应判为 roadmap，得到 %q", scope)
	}
}

// stubScopeLookup 实现附件判定需要的两个查询。
type stubScopeLookup struct {
	repository.ForumRepo

	scope   domain.ForumScope
	deleted bool
	err     error
}

func (s *stubScopeLookup) LookupPostScope(_ context.Context, _ int) (domain.ForumScope, bool, error) {
	return s.scope, s.deleted, s.err
}

func (s *stubScopeLookup) LookupReplyPostScope(_ context.Context, _ int) (domain.ForumScope, bool, error) {
	return s.scope, s.deleted, s.err
}

// 管理员名单桩复用同包 freeze_usec_test.go 的 stubAdminDirectory（ids []int）。
