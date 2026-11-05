package service

import (
	"context"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// stubForumRepo 只实现 forum 用例用到的三个方法，越界即 panic（见桩约定）。
type stubForumRepo struct {
	repository.ForumRepo
	post     *domain.ForumPost
	postErr  error
	replies  []*domain.ForumReply
	replyErr error
}

func (s *stubForumRepo) GetPost(_ context.Context, _ int) (*domain.ForumPost, error) {
	return s.post, s.postErr
}

func (s *stubForumRepo) ListReplies(_ context.Context, _ int) ([]*domain.ForumReply, error) {
	return s.replies, s.replyErr
}

func (s *stubForumRepo) ListPosts(_ context.Context, _ domain.ForumScope, _ repository.ForumPostListFilter, _, _ int) ([]*repository.ForumPostSummary, int64, error) {
	return nil, 0, nil
}

// TestForumOfficialReplyPinning 官方回答（管理员回复）置顶，且随管理员名单变化。
//
// 验收要求「官方回答置顶随管理员名单变化」：同一组回复，当作者被加入/移出
// 管理员名单时，排序结果应当翻转。这里用两个 AdminDirectory 验证。
func TestForumOfficialReplyPinning(t *testing.T) {
	base := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	replies := []*domain.ForumReply{
		{ID: 1, AuthorID: 5, CreatedAt: base.Add(time.Minute)},      // 普通用户，后发
		{ID: 2, AuthorID: 2, CreatedAt: base},                       // 管理员，先发
		{ID: 3, AuthorID: 9, CreatedAt: base.Add(2 * time.Minute)},  // 普通用户，最晚发
	}
	post := &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment}

	// 场景一：id=2 是管理员 → 它的回复应排到最前，其余按时间正序。
	uc := NewForumUseCase(&stubForumRepo{post: post, replies: replies}, &stubAdminDirectory{ids: []int{2}})
	detail, err := uc.GetPostDetail(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(detail.Replies) != 3 {
		t.Fatalf("expected 3 replies, got %d", len(detail.Replies))
	}
	if detail.Replies[0].ID != 2 || !detail.Official[0] {
		t.Fatalf("expected admin reply (id=2) pinned first, got id=%d official=%v", detail.Replies[0].ID, detail.Official[0])
	}
	// 余下两位是普通用户，按时间正序：id=1(先) 然后 id=3(后)。
	if detail.Replies[1].ID != 1 || detail.Replies[2].ID != 3 {
		t.Fatalf("expected non-official order [1,3], got [%d,%d]", detail.Replies[1].ID, detail.Replies[2].ID)
	}

	// 场景二：名单变化，id=2 不再是管理员、id=9 成为管理员 → 置顶者翻转。
	uc2 := NewForumUseCase(&stubForumRepo{post: post, replies: replies}, &stubAdminDirectory{ids: []int{9}})
	detail2, err := uc2.GetPostDetail(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail2.Replies[0].ID != 3 || !detail2.Official[0] {
		t.Fatalf("expected admin reply (id=3) pinned first after roster change, got id=%d official=%v", detail2.Replies[0].ID, detail2.Official[0])
	}
}

// TestForumSoftDeletedInvisible 软删帖对所有人不可见：详情返回 ErrForumPostNotFound。
func TestForumSoftDeletedInvisible(t *testing.T) {
	now := time.Now()
	post := &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment, DeletedAt: &now}
	uc := NewForumUseCase(&stubForumRepo{post: post}, &stubAdminDirectory{})
	_, err := uc.GetPostDetail(context.Background(), 1)
	if err == nil || err != ErrForumPostNotFound {
		t.Fatalf("expected ErrForumPostNotFound for soft-deleted post, got %v", err)
	}
}

// TestForumPostNotFoundPropagates repo 未找到时透传 ErrForumPostNotFound。
func TestForumPostNotFoundPropagates(t *testing.T) {
	uc := NewForumUseCase(&stubForumRepo{postErr: repository.ErrForumPostNotFound}, &stubAdminDirectory{})
	_, err := uc.GetPostDetail(context.Background(), 999)
	if err == nil || err != ErrForumPostNotFound {
		t.Fatalf("expected ErrForumPostNotFound propagated, got %v", err)
	}
}

// TestForumAdminDirectoryErrorIsSafe AdminDirectory 读失败时按「非官方」保守处理，
// 不报错、不把普通回复误升为官方（保证列表/详情页在 Redis 抖动时仍可用）。
func TestForumAdminDirectoryErrorIsSafe(t *testing.T) {
	base := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	replies := []*domain.ForumReply{{ID: 1, AuthorID: 2, CreatedAt: base}}
	post := &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment}
	uc := NewForumUseCase(&stubForumRepo{post: post, replies: replies}, &stubAdminDirectory{err: context.Canceled})
	detail, err := uc.GetPostDetail(context.Background(), 1)
	if err != nil {
		t.Fatalf("admin dir error should be safe, got %v", err)
	}
	for i, official := range detail.Official {
		if official {
			t.Fatalf("reply %d must not be official when admin dir errors", detail.Replies[i].ID)
		}
	}
}
