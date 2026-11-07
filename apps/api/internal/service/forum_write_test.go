package service

import (
	"bytes"
	"context"
	"errors"
	"log"
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
//
// 新建回复自 #241 起仅限管理员，因此这里的调用者 userID=2 必须在名单内，
// 否则测到的是权限而不是长度边界。
func TestCreateReplyLengthBoundary(t *testing.T) {
	post := &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment}

	repo := &stubForumWriteRepo{post: post}
	uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{2}})
	if _, err := uc.CreateReply(context.Background(), 2, 1, CreateForumReplyInput{Body: strings.Repeat("文", 20000)}); err != nil {
		t.Fatalf("20000 runes should pass, got %v", err)
	}

	repo2 := &stubForumWriteRepo{post: post}
	uc2 := NewForumUseCase(repo2, &stubAdminDirectory{ids: []int{2}})
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

/* ------------------------------------------------------------------ *
 * 附件绑定回填（#241）
 * ------------------------------------------------------------------ */

// stubAttachmentBinder 记录按正文重算绑定的调用，可注入失败。
type stubAttachmentBinder struct {
	calls []bindingCall
	err   error
}

type bindingCall struct {
	col      string
	ownerID  int64
	authorID int
	body     string
}

func (s *stubAttachmentBinder) RecomputeBindings(_ context.Context, col string, ownerID int64, authorID int, body string) error {
	s.calls = append(s.calls, bindingCall{col: col, ownerID: ownerID, authorID: authorID, body: body})
	return s.err
}

// captureStdLog 把标准日志临时重定向到缓冲区，返回后自动还原。
func captureStdLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	orig := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	return buf
}

// TestCreatePostBindsAttachments 发帖后按正文重算附件绑定，列取 post_id。
func TestCreatePostBindsAttachments(t *testing.T) {
	repo := &stubForumWriteRepo{}
	binder := &stubAttachmentBinder{}
	uc := NewForumUseCase(repo, &stubAdminDirectory{}).WithAttachments(binder)

	body := "看图 ![x](/api/v1/attachments/7)"
	if _, err := uc.CreatePost(context.Background(), 1, domain.ForumScopeRecruitment,
		CreateForumPostInput{Title: "标题", Body: body}); err != nil {
		t.Fatalf("create post: %v", err)
	}

	if len(binder.calls) != 1 {
		t.Fatalf("expected 1 binding pass, got %d", len(binder.calls))
	}
	got := binder.calls[0]
	// 落库后才知道帖子 id，绑定必须用返回的 id 而不是 0。
	if got.col != "post_id" || got.ownerID != 100 || got.authorID != 1 || got.body != body {
		t.Fatalf("unexpected binding call: %+v", got)
	}
}

// TestUpdatePostRebindsAttachments 编辑帖子同样重算绑定：正文是唯一真源，
// 移除的图必须走同一次重算被解绑回孤儿态。
func TestUpdatePostRebindsAttachments(t *testing.T) {
	repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 42, AuthorID: 1, Scope: domain.ForumScopeRecruitment}}
	binder := &stubAttachmentBinder{}
	uc := NewForumUseCase(repo, &stubAdminDirectory{}).WithAttachments(binder)

	body := "只剩这张 ![y](/api/v1/attachments/8)"
	if _, err := uc.UpdatePost(context.Background(), 1, 42, UpdateForumPostInput{Title: "t", Body: body}); err != nil {
		t.Fatalf("update post: %v", err)
	}
	if len(binder.calls) != 1 {
		t.Fatalf("expected 1 binding pass, got %d", len(binder.calls))
	}
	if binder.calls[0].col != "post_id" || binder.calls[0].ownerID != 42 {
		t.Fatalf("unexpected binding call: %+v", binder.calls[0])
	}
}

// TestCreateReplyBindsAttachments 新建回复按正文重算绑定，列取 reply_id。
func TestCreateReplyBindsAttachments(t *testing.T) {
	repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment}}
	binder := &stubAttachmentBinder{}
	uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{1}}).WithAttachments(binder)

	body := "补充 ![z](/api/v1/attachments/9)"
	if _, err := uc.CreateReply(context.Background(), 1, 1, CreateForumReplyInput{Body: body}); err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if len(binder.calls) != 1 {
		t.Fatalf("expected 1 binding pass, got %d", len(binder.calls))
	}
	got := binder.calls[0]
	if got.col != "reply_id" || got.ownerID != 200 || got.authorID != 1 || got.body != body {
		t.Fatalf("unexpected binding call: %+v", got)
	}
}

// TestUpdateReplyBindsAttachments 编辑回复也重算绑定。
func TestUpdateReplyBindsAttachments(t *testing.T) {
	repo := &stubForumWriteRepo{reply: &domain.ForumReply{ID: 5, PostID: 1, AuthorID: 1}}
	binder := &stubAttachmentBinder{}
	uc := NewForumUseCase(repo, &stubAdminDirectory{}).WithAttachments(binder)

	body := "换成这张 ![w](/api/v1/attachments/10)"
	if _, err := uc.UpdateReply(context.Background(), 1, 5, CreateForumReplyInput{Body: body}); err != nil {
		t.Fatalf("update reply: %v", err)
	}
	if len(binder.calls) != 1 {
		t.Fatalf("expected 1 binding pass, got %d", len(binder.calls))
	}
	if binder.calls[0].col != "reply_id" || binder.calls[0].ownerID != 5 {
		t.Fatalf("unexpected binding call: %+v", binder.calls[0])
	}
}

// TestBindAttachmentsFailureDoesNotRollback 绑定失败（引用了别人的图等）
// 不能让已落库的正文消失，但必须留下日志线索。
func TestBindAttachmentsFailureDoesNotRollback(t *testing.T) {
	repo := &stubForumWriteRepo{}
	binder := &stubAttachmentBinder{err: domain.ErrAttachmentForbidden}
	uc := NewForumUseCase(repo, &stubAdminDirectory{}).WithAttachments(binder)

	buf := captureStdLog(t)
	post, err := uc.CreatePost(context.Background(), 1, domain.ForumScopeRecruitment,
		CreateForumPostInput{Title: "t", Body: "![x](/api/v1/attachments/999)"})
	if err != nil {
		t.Fatalf("正文已落库时绑定失败不应回滚，got %v", err)
	}
	if post == nil || repo.createdPost == nil {
		t.Fatal("expected post persisted despite binding failure")
	}
	logs := buf.String()
	if !strings.Contains(logs, "[forum]") || !strings.Contains(logs, "绑定") {
		t.Fatalf("绑定失败必须记日志，实际日志=%q", logs)
	}
}

// TestBindAttachmentsNilIsNoop 未注入绑定能力时不 panic（只读/既有桩场景）。
func TestBindAttachmentsNilIsNoop(t *testing.T) {
	repo := &stubForumWriteRepo{}
	uc := NewForumUseCase(repo, &stubAdminDirectory{})
	if _, err := uc.CreatePost(context.Background(), 1, domain.ForumScopeRecruitment,
		CreateForumPostInput{Title: "t", Body: "![x](/api/v1/attachments/1)"}); err != nil {
		t.Fatalf("unexpected error without binder: %v", err)
	}
}

// TestBindAttachmentsUsesContentAuthor 绑定的归属是**内容作者**，不是操作者。
//
// 仓库层的校验是「附件只能由上传者本人绑到自己的内容」。管理员编辑他人帖子时
// 若把编辑者当上传者，该作者名下的图片会整批被判越权，重算回滚、图全部退成孤儿。
func TestBindAttachmentsUsesContentAuthor(t *testing.T) {
	const authorID = 5
	const adminID = 9

	t.Run("管理员编辑他人帖子按帖子作者绑定", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: &domain.ForumPost{ID: 1, AuthorID: authorID, Scope: domain.ForumScopeRecruitment}}
		binder := &stubAttachmentBinder{}
		uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{adminID}}).WithAttachments(binder)

		if _, err := uc.UpdatePost(context.Background(), adminID, 1,
			UpdateForumPostInput{Title: "t", Body: "![x](/api/v1/attachments/3)"}); err != nil {
			t.Fatalf("admin should edit others' post, got %v", err)
		}
		if len(binder.calls) != 1 {
			t.Fatalf("expected 1 binding pass, got %d", len(binder.calls))
		}
		if binder.calls[0].authorID != authorID {
			t.Fatalf("expected content author %d, got %d", authorID, binder.calls[0].authorID)
		}
	})

	t.Run("管理员编辑他人回复按回复作者绑定", func(t *testing.T) {
		repo := &stubForumWriteRepo{reply: &domain.ForumReply{ID: 1, PostID: 1, AuthorID: authorID}}
		binder := &stubAttachmentBinder{}
		uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{adminID}}).WithAttachments(binder)

		if _, err := uc.UpdateReply(context.Background(), adminID, 1,
			CreateForumReplyInput{Body: "![x](/api/v1/attachments/4)"}); err != nil {
			t.Fatalf("admin should edit others' reply, got %v", err)
		}
		if len(binder.calls) != 1 {
			t.Fatalf("expected 1 binding pass, got %d", len(binder.calls))
		}
		if binder.calls[0].authorID != authorID {
			t.Fatalf("expected content author %d, got %d", authorID, binder.calls[0].authorID)
		}
	})
}

/* ------------------------------------------------------------------ *
 * 回答权限收口到管理员（#241）
 * ------------------------------------------------------------------ */

// TestCreateReplyAdminOnly 新建回复仅限管理员；403 语义由 service 哨兵承载。
func TestCreateReplyAdminOnly(t *testing.T) {
	post := &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment}

	t.Run("普通用户被拒", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		_, err := uc.CreateReply(context.Background(), 2, 1, CreateForumReplyInput{Body: "回复"})
		if !errors.Is(err, ErrForumReplyForbidden) {
			t.Fatalf("expected ErrForumReplyForbidden, got %v", err)
		}
		if repo.createdReply != nil {
			t.Fatal("must not create reply for non-admin")
		}
	})

	t.Run("管理员可回复", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{9}})
		if _, err := uc.CreateReply(context.Background(), 9, 1, CreateForumReplyInput{Body: "回复"}); err != nil {
			t.Fatalf("admin should reply, got %v", err)
		}
		if repo.createdReply == nil {
			t.Fatal("expected reply created by admin")
		}
	})

	t.Run("管理员名单读失败时保守拒绝", func(t *testing.T) {
		repo := &stubForumWriteRepo{post: post}
		uc := NewForumUseCase(repo, &stubAdminDirectory{err: context.Canceled})
		_, err := uc.CreateReply(context.Background(), 9, 1, CreateForumReplyInput{Body: "回复"})
		if !errors.Is(err, ErrForumReplyForbidden) {
			t.Fatalf("expected deny when admin dir errors, got %v", err)
		}
	})

	t.Run("自己的回复仍可编辑", func(t *testing.T) {
		repo := &stubForumWriteRepo{reply: &domain.ForumReply{ID: 1, PostID: 1, AuthorID: 2, Body: "旧"}}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if _, err := uc.UpdateReply(context.Background(), 2, 1, CreateForumReplyInput{Body: "新"}); err != nil {
			t.Fatalf("author should still edit own reply, got %v", err)
		}
	})

	t.Run("自己的回复仍可删除", func(t *testing.T) {
		repo := &stubForumWriteRepo{reply: &domain.ForumReply{ID: 1, PostID: 1, AuthorID: 2}}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if err := uc.DeleteReply(context.Background(), 2, 1); err != nil {
			t.Fatalf("author should still delete own reply, got %v", err)
		}
	})

	t.Run("普通用户仍可发帖", func(t *testing.T) {
		repo := &stubForumWriteRepo{}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})
		if _, err := uc.CreatePost(context.Background(), 2, domain.ForumScopeRecruitment,
			CreateForumPostInput{Title: "t", Body: "b"}); err != nil {
			t.Fatalf("non-admin should still create posts, got %v", err)
		}
	})

	// 内部论坛本期明确不改：它的非管理员由路由分组拦成 404（不暴露存在性）。
	// service 若在此也按 403 拦，「JWT 仍标 admin 但 Redis 已撤销」的人就会拿到
	// 403，反过来证实了内部论坛帖子的存在，破坏那条 404 口径。
	t.Run("内部论坛不走这条 403 判定", func(t *testing.T) {
		studioPost := &domain.ForumPost{ID: 3, Scope: domain.ForumScopeStudio}
		repo := &stubForumWriteRepo{post: studioPost}
		uc := NewForumUseCase(repo, &stubAdminDirectory{})

		_, err := uc.CreateReply(context.Background(), 2, 3, CreateForumReplyInput{Body: "回复"})
		if errors.Is(err, ErrForumReplyForbidden) {
			t.Fatalf("studio 不应由 service 判 403（路由层负责 404），got %v", err)
		}
	})
}
