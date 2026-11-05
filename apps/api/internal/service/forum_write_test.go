package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 写入路径的桩
 * ------------------------------------------------------------------ */

// stubForumWriteRepo 记录写入调用，读方法走内嵌的零值接口（调用即 panic）。
type stubForumWriteRepo struct {
	repository.ForumRepo

	post    *domain.ForumPost
	postErr error
	reply   *domain.ForumReply
	replyErr error

	createdPost  *domain.ForumPost
	updatedPost  *domain.ForumPost
	deletedPost  [2]int // [id, byUserID]
	createdReply *domain.ForumReply
	updatedReply *domain.ForumReply
	deletedReply [2]int
}

func (s *stubForumWriteRepo) GetPost(_ context.Context, _ int) (*domain.ForumPost, error) {
	if s.postErr != nil {
		return nil, s.postErr
	}
	return s.post, nil
}

func (s *stubForumWriteRepo) CreatePost(_ context.Context, p *domain.ForumPost) error {
	p.ID = 100
	s.createdPost = p
	return nil
}

func (s *stubForumWriteRepo) UpdatePost(_ context.Context, p *domain.ForumPost) error {
	s.updatedPost = p
	return nil
}

func (s *stubForumWriteRepo) SoftDeletePost(_ context.Context, id int, by int) error {
	s.deletedPost = [2]int{id, by}
	return nil
}

func (s *stubForumWriteRepo) GetReply(_ context.Context, _ int) (*domain.ForumReply, error) {
	if s.replyErr != nil {
		return nil, s.replyErr
	}
	return s.reply, nil
}

func (s *stubForumWriteRepo) CreateReply(_ context.Context, r *domain.ForumReply) error {
	r.ID = 200
	s.createdReply = r
	return nil
}

func (s *stubForumWriteRepo) UpdateReply(_ context.Context, r *domain.ForumReply) error {
	s.updatedReply = r
	return nil
}

func (s *stubForumWriteRepo) SoftDeleteReply(_ context.Context, id int, by int) error {
	s.deletedReply = [2]int{id, by}
	return nil
}

// stubUserDir 提供资料方向。
type stubUserDir struct {
	direction domain.Direction
	err       error
}

func (s *stubUserDir) GetByID(_ context.Context, id int) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &domain.User{ID: id, Detail: domain.Detail{Direction: s.direction}}, nil
}

// stubAppDir 提供最新报名方向。
type stubAppDir struct {
	apps []*domain.Application
	err  error
}

func (s *stubAppDir) ListApplicationsByUser(_ context.Context, _ int, _, _ int) ([]*domain.Application, int64, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.apps, int64(len(s.apps)), nil
}

/* ------------------------------------------------------------------ *
 * 长度边界
 * ------------------------------------------------------------------ */

// TestCreatePostLengthBoundary 标题 200/201、正文 20000/20001 的边界。
func TestCreatePostLengthBoundary(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		body    string
		wantErr bool
	}{
		{"标题正好 200", strings.Repeat("标", 200), "正文", false},
		{"标题 201", strings.Repeat("标", 201), "正文", true},
		{"正文正好 20000", "标题", strings.Repeat("文", 20000), false},
		{"正文 20001", "标题", strings.Repeat("文", 20001), true},
		{"标题空", "   ", "正文", true},
		{"正文空", "标题", "  ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubForumWriteRepo{}
			uc := NewForumUseCase(repo, &stubAdminDirectory{})
			_, err := uc.CreatePost(context.Background(), 1, domain.ForumScopeRecruitment,
				CreateForumPostInput{Title: tc.title, Body: tc.body})
			if tc.wantErr {
				if !IsForumInvalidInput(err) {
					t.Fatalf("expected invalid input error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.createdPost == nil {
				t.Fatal("expected post created")
			}
		})
	}
}

// TestCreateReplyLengthBoundary 回复正文 20000/20001。
func TestCreateReplyLengthBoundary(t *testing.T) {
	post := &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment}

	repo := &stubForumWriteRepo{post: post}
	uc := NewForumUseCase(repo, &stubAdminDirectory{})
	if _, err := uc.CreateReply(context.Background(), 2, 1, CreateForumReplyInput{Body: strings.Repeat("文", 20000)}); err != nil {
		t.Fatalf("20000 runes should pass, got %v", err)
	}

	repo2 := &stubForumWriteRepo{post: post}
	uc2 := NewForumUseCase(repo2, &stubAdminDirectory{})
	_, err := uc2.CreateReply(context.Background(), 2, 1, CreateForumReplyInput{Body: strings.Repeat("文", 20001)})
	if !IsForumInvalidInput(err) {
		t.Fatalf("20001 runes should be rejected, got %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 方向回落四级优先级
 * ------------------------------------------------------------------ */

// TestResolveDirectionPriority 手选 → 资料方向 → 最新报名 → 留空。
func TestResolveDirectionPriority(t *testing.T) {
	chosen := "game"
	empty := "  "

	t.Run("手选优先于一切", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{}).
			WithDirectionFallback(&stubUserDir{direction: domain.FrontEnd}, &stubAppDir{apps: []*domain.Application{{DirectionCode: "back-end"}}})
		got := uc.resolveDirection(context.Background(), 1, &chosen)
		if got == nil || *got != "game" {
			t.Fatalf("expected chosen=game, got %v", got)
		}
	})

	t.Run("未手选取资料方向", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{}).
			WithDirectionFallback(&stubUserDir{direction: domain.FrontEnd}, &stubAppDir{apps: []*domain.Application{{DirectionCode: "back-end"}}})
		got := uc.resolveDirection(context.Background(), 1, nil)
		if got == nil || *got != "front-end" {
			t.Fatalf("expected front-end from profile, got %v", got)
		}
	})

	t.Run("资料方向为空取最新报名", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{}).
			WithDirectionFallback(&stubUserDir{direction: ""}, &stubAppDir{apps: []*domain.Application{{DirectionCode: "back-end"}}})
		got := uc.resolveDirection(context.Background(), 1, nil)
		if got == nil || *got != "back-end" {
			t.Fatalf("expected back-end from latest application, got %v", got)
		}
	})

	t.Run("都没有则留空", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{}).
			WithDirectionFallback(&stubUserDir{direction: ""}, &stubAppDir{apps: nil})
		got := uc.resolveDirection(context.Background(), 1, nil)
		if got != nil {
			t.Fatalf("expected nil direction, got %v", *got)
		}
	})

	t.Run("传空串视同没选，继续回落", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{}).
			WithDirectionFallback(&stubUserDir{direction: domain.Agent}, &stubAppDir{})
		got := uc.resolveDirection(context.Background(), 1, &empty)
		if got == nil || *got != "agent" {
			t.Fatalf("expected fallback to profile=agent for blank input, got %v", got)
		}
	})

	t.Run("读用户失败不阻断，退到报名", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{}).
			WithDirectionFallback(&stubUserDir{err: errors.New("boom")}, &stubAppDir{apps: []*domain.Application{{DirectionCode: "game"}}})
		got := uc.resolveDirection(context.Background(), 1, nil)
		if got == nil || *got != "game" {
			t.Fatalf("expected game despite user read failure, got %v", got)
		}
	})

	t.Run("未注入数据源时留空", func(t *testing.T) {
		uc := NewForumUseCase(&stubForumWriteRepo{}, &stubAdminDirectory{})
		got := uc.resolveDirection(context.Background(), 1, nil)
		if got != nil {
			t.Fatalf("expected nil when no fallback sources, got %v", *got)
		}
	})
}

/* ------------------------------------------------------------------ *
 * 归属校验
 * ------------------------------------------------------------------ */

// TestUpdatePostPermission 作者本人可编辑；非作者非管理员 404；管理员可编辑他人。
func TestUpdatePostPermission(t *testing.T) {
	const authorID = 5
	post := &domain.ForumPost{ID: 1, AuthorID: authorID, Scope: domain.ForumScopeRecruitment, Body: "旧"}

	t.Run("作者本人可编辑", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		got, err := uc.UpdatePost(context.Background(), authorID, 1, UpdateForumPostInput{Title: "新标题", Body: "新正文"})
		if err != nil {
			t.Fatalf("author should edit, got %v", err)
		}
		if repo.updatedPost == nil || got.EditedAt == nil {
			t.Fatal("expected update recorded with edited_at")
		}
	})

	t.Run("非作者非管理员被拒成 404", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{}})
		_, err := uc.UpdatePost(context.Background(), 7, 1, UpdateForumPostInput{Title: "t", Body: "b"})
		if err != ErrForumPostNotFound {
			t.Fatalf("expected ErrForumPostNotFound, got %v", err)
		}
		if repo.updatedPost != nil {
			t.Fatal("must not write when unauthorized")
		}
	})

	t.Run("管理员可编辑他人", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{9}})
		if _, err := uc.UpdatePost(context.Background(), 9, 1, UpdateForumPostInput{Title: "t", Body: "b"}); err != nil {
			t.Fatalf("admin should edit others, got %v", err)
		}
	})

	t.Run("AdminDirectory 失败时保守拒绝", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{err: context.Canceled})
		_, err := uc.UpdatePost(context.Background(), 7, 1, UpdateForumPostInput{Title: "t", Body: "b"})
		if err != ErrForumPostNotFound {
			t.Fatalf("expected deny when admin dir errors, got %v", err)
		}
	})
}

// TestDeletePostPermission 删除权限与编辑一致；软删后详情不可见。
func TestDeletePostPermission(t *testing.T) {
	const authorID = 5

	t.Run("作者可删", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 1, AuthorID: authorID}}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if err := uc.DeletePost(context.Background(), authorID, 1); err != nil {
			t.Fatalf("author should delete, got %v", err)
		}
		if repo.deletedPost != [2]int{1, authorID} {
			t.Fatalf("expected soft delete with deleter recorded, got %v", repo.deletedPost)
		}
	})

	t.Run("他人被拒", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 1, AuthorID: authorID}}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if err := uc.DeletePost(context.Background(), 7, 1); err != ErrForumPostNotFound {
			t.Fatalf("expected ErrForumPostNotFound, got %v", err)
		}
		if repo.deletedPost != [2]int{0, 0} {
			t.Fatal("must not delete when unauthorized")
		}
	})

	t.Run("已软删的帖子不再可见", func(t *testing.T) {
		now := time.Now()
		repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 1, AuthorID: authorID, DeletedAt: &now}}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if err := uc.DeletePost(context.Background(), authorID, 1); err != ErrForumPostNotFound {
			t.Fatalf("expected ErrForumPostNotFound for soft-deleted, got %v", err)
		}
	})
}

// TestCreateReplyOnDeletedPost 帖子已软删时不能回复。
func TestCreateReplyOnDeletedPost(t *testing.T) {
	now := time.Now()
	repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 1, DeletedAt: &now}}
	uc := NewForumUseCase(repo, &stubAdminDirectory{})
	_, err := uc.CreateReply(context.Background(), 2, 1, CreateForumReplyInput{Body: "回复"})
	if err != ErrForumPostNotFound {
		t.Fatalf("expected ErrForumPostNotFound, got %v", err)
	}
	if repo.createdReply != nil {
		t.Fatal("must not create reply on deleted post")
	}
}

// TestReplyPermission 回复的编辑/删除权限同帖子。
func TestReplyPermission(t *testing.T) {
	const authorID = 5
	reply := &domain.ForumReply{ID: 1, PostID: 1, AuthorID: authorID, Body: "旧"}

	t.Run("作者可编辑回复并记录 edited_at", func(t *testing.T) {
		repo := &stubForumWriteRepo{reply: reply}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		got, err := uc.UpdateReply(context.Background(), authorID, 1, CreateForumReplyInput{Body: "新"})
		if err != nil {
			t.Fatalf("author should edit reply, got %v", err)
		}
		if got.EditedAt == nil {
			t.Fatal("expected edited_at recorded")
		}
	})

	t.Run("他人被拒成 404", func(t *testing.T) {
		repo := &stubForumWriteRepo{reply: reply}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if _, err := uc.UpdateReply(context.Background(), 7, 1, CreateForumReplyInput{Body: "x"}); err != ErrForumReplyNotFound {
			t.Fatalf("expected ErrForumReplyNotFound, got %v", err)
		}
	})

	t.Run("作者可删回复", func(t *testing.T) {
		repo := &stubForumWriteRepo{reply: reply}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if err := uc.DeleteReply(context.Background(), authorID, 1); err != nil {
			t.Fatalf("author should delete reply, got %v", err)
		}
		if repo.deletedReply != [2]int{1, authorID} {
			t.Fatalf("expected soft delete recorded, got %v", repo.deletedReply)
		}
	})

	t.Run("repo 未找到透传", func(t *testing.T) {
		repo := &stubForumWriteRepo{replyErr: repository.ErrForumReplyNotFound}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if err := uc.DeleteReply(context.Background(), authorID, 99); err != repository.ErrForumReplyNotFound {
			t.Fatalf("expected ErrForumReplyNotFound, got %v", err)
		}
	})
}
