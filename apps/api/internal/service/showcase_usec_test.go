package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 作品提交：审阅规则与归属判定
 *
 * 这两件事最容易被写错：
 *   - 驳回落一个空意见，学员不知道改什么；
 *   - 归属判定只看「有没有登录」而不是「是不是本人」。
 * ------------------------------------------------------------------ */

// stubShowcaseRepo 内存实现。
type stubShowcaseRepo struct {
	repository.ShowcaseRepo

	rows map[int64]*domain.ShowcaseSubmission
	// reviewed 记录 Review 收到的入参，用来断言「没被调用」。
	reviewed []string
}

func newStubShowcaseRepo(rows ...*domain.ShowcaseSubmission) *stubShowcaseRepo {
	m := make(map[int64]*domain.ShowcaseSubmission, len(rows))
	for _, r := range rows {
		m[r.ID] = r
	}
	return &stubShowcaseRepo{rows: m}
}

func (s *stubShowcaseRepo) Create(_ context.Context, sub *domain.ShowcaseSubmission) error {
	sub.ID = int64(len(s.rows) + 1)
	cp := *sub
	s.rows[sub.ID] = &cp
	return nil
}

func (s *stubShowcaseRepo) GetByID(_ context.Context, id int64) (*domain.ShowcaseSubmission, error) {
	r, ok := s.rows[id]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (s *stubShowcaseRepo) DeleteOwn(_ context.Context, id, userID int64) error {
	r, ok := s.rows[id]
	if !ok || r.UserID != userID {
		return domain.ErrShowcaseNotFound
	}
	delete(s.rows, id)
	return nil
}

func (s *stubShowcaseRepo) Review(_ context.Context, id int64, status, note string, reviewerID int64) error {
	r, ok := s.rows[id]
	if !ok {
		return domain.ErrShowcaseNotFound
	}
	r.Status = status
	r.ReviewNote = note
	s.reviewed = append(s.reviewed, status+":"+note)
	return nil
}

// TestShowcaseReview_RejectRequiresNote 驳回必须带理由。
//
// 只说「不通过」而不说改什么，学员下次交的大概率还是同样的问题。
func TestShowcaseReview_RejectRequiresNote(t *testing.T) {
	repo := newStubShowcaseRepo(&domain.ShowcaseSubmission{ID: 1, UserID: 7})
	uc := NewShowcaseUseCase(repo)

	err := uc.Review(context.Background(), 1, 99, domain.ShowcaseStatusRejected, "   ")
	if !errors.Is(err, domain.ErrShowcaseInvalid) {
		t.Fatalf("驳回无理由应返回 ErrShowcaseInvalid，实际 %v", err)
	}
	if len(repo.reviewed) != 0 {
		t.Fatal("校验失败时不应写库")
	}
}

// TestShowcaseReview_ApproveAllowsEmptyNote 通过时意见可空。
func TestShowcaseReview_ApproveAllowsEmptyNote(t *testing.T) {
	repo := newStubShowcaseRepo(&domain.ShowcaseSubmission{ID: 1, UserID: 7})
	uc := NewShowcaseUseCase(repo)

	if err := uc.Review(context.Background(), 1, 99, domain.ShowcaseStatusApproved, ""); err != nil {
		t.Fatalf("通过时应允许空意见，实际 %v", err)
	}
	if len(repo.reviewed) != 1 {
		t.Fatalf("应写入一次审核，实际 %d", len(repo.reviewed))
	}
}

// TestShowcaseReview_RejectsSubmittedAsTarget 不能把状态「审核」回 submitted。
//
// 那等于撤销审核却把 reviewed_at 也刷新了，语义混乱。
func TestShowcaseReview_RejectsSubmittedAsTarget(t *testing.T) {
	repo := newStubShowcaseRepo(&domain.ShowcaseSubmission{ID: 1, UserID: 7})
	uc := NewShowcaseUseCase(repo)

	for _, bad := range []string{domain.ShowcaseStatusSubmitted, "pending", ""} {
		if err := uc.Review(context.Background(), 1, 99, bad, "x"); err == nil {
			t.Fatalf("状态 %q 应被拒", bad)
		}
	}
}

// TestShowcaseDelete_CannotDeleteOthers 只能删自己的。
//
// 这里的关键是**不区分**「不存在」与「不是你的」：回同一个错误，
// 否则可以用响应差异探测出别人作品的存在性。
func TestShowcaseDelete_CannotDeleteOthers(t *testing.T) {
	repo := newStubShowcaseRepo(&domain.ShowcaseSubmission{ID: 1, UserID: 7})
	uc := NewShowcaseUseCase(repo)

	err := uc.Delete(context.Background(), 1, 8) // 8 不是作者
	if !errors.Is(err, domain.ErrShowcaseNotFound) {
		t.Fatalf("删除他人作品应返回 ErrShowcaseNotFound，实际 %v", err)
	}
	if _, ok := repo.rows[1]; !ok {
		t.Fatal("他人作品不能被删掉")
	}

	// 不存在的 id 也回同一个错误，两者不可区分。
	errMissing := uc.Delete(context.Background(), 999, 8)
	if !errors.Is(errMissing, domain.ErrShowcaseNotFound) {
		t.Fatalf("不存在的作品也应返回 ErrShowcaseNotFound，实际 %v", errMissing)
	}
}

// TestShowcaseDelete_OwnSucceeds 删自己的可以。
func TestShowcaseDelete_OwnSucceeds(t *testing.T) {
	repo := newStubShowcaseRepo(&domain.ShowcaseSubmission{ID: 1, UserID: 7})
	uc := NewShowcaseUseCase(repo)

	if err := uc.Delete(context.Background(), 1, 7); err != nil {
		t.Fatalf("删除自己的作品应成功，实际 %v", err)
	}
	if _, ok := repo.rows[1]; ok {
		t.Fatal("应已删除")
	}
}

// TestShowcaseCreate_RejectsEmptyPayload 创建时也要挡住空载体。
//
// 数据库有 CHECK 兜底，但那是 500；这里要的是 400 + 可读原因。
func TestShowcaseCreate_RejectsEmptyPayload(t *testing.T) {
	repo := newStubShowcaseRepo()
	uc := NewShowcaseUseCase(repo)

	_, err := uc.Create(context.Background(), 7, CreateShowcaseInput{Title: "只有标题"})
	if !errors.Is(err, domain.ErrShowcaseInvalid) {
		t.Fatalf("无载体应被拒，实际 %v", err)
	}
	if len(repo.rows) != 0 {
		t.Fatal("校验失败时不应落库")
	}
}

// TestShowcaseCreate_SnapshotsRoadmapTitle 提交时记下路线标题快照。
//
// 路线改名或被删后，这次提交仍要能看出当时交的是哪条。
func TestShowcaseCreate_SnapshotsRoadmapTitle(t *testing.T) {
	roadmaps := newStubRoadmapRepo(&domain.LearningRoadmap{ID: 3, Title: "Spring 后端完整学习路线"})
	repo := newStubShowcaseRepo()
	uc := NewShowcaseUseCase(repo).WithRoadmaps(roadmaps)

	rid := int64(3)
	sub, err := uc.Create(context.Background(), 7, CreateShowcaseInput{
		RoadmapID: &rid,
		Title:     "我的项目",
		RepoURL:   "https://github.com/a/b",
	})
	if err != nil {
		t.Fatalf("创建应成功，实际 %v", err)
	}
	if sub.RoadmapTitle != "Spring 后端完整学习路线" {
		t.Fatalf("应记录路线标题快照，实际 %q", sub.RoadmapTitle)
	}
}

// TestShowcaseCreate_RejectsNonZip 非 zip 不该进对象存储。
func TestShowcaseCreate_RejectsNonZip(t *testing.T) {
	repo := newStubShowcaseRepo()
	uc := NewShowcaseUseCase(repo) // 不注入 storage：非 zip 应该在校验阶段就被拦

	_, err := uc.Create(context.Background(), 7, CreateShowcaseInput{
		Title:       "项目",
		ArchiveName: "demo.rar",
		ArchiveData: []byte("x"),
	})
	if !errors.Is(err, domain.ErrShowcaseArchiveUnsupportedType) {
		t.Fatalf("非 zip 应被拒，实际 %v", err)
	}
	if len(repo.rows) != 0 {
		t.Fatal("校验失败时不应落库")
	}
}

// TestShowcaseCreate_RejectsOversizeZip 超过 50MB 应被拒。
func TestShowcaseCreate_RejectsOversizeZip(t *testing.T) {
	repo := newStubShowcaseRepo()
	uc := NewShowcaseUseCase(repo)

	_, err := uc.Create(context.Background(), 7, CreateShowcaseInput{
		Title:       "项目",
		ArchiveName: "big.zip",
		ArchiveData: make([]byte, domain.ShowcaseArchiveMaxBytes+1),
	})
	if !errors.Is(err, domain.ErrShowcaseArchiveTooLarge) {
		t.Fatalf("超大包应被拒，实际 %v", err)
	}
}

// TestShowcaseOpenArchive_Ownership 下载的归属判定。
//
// 本人可下、管理员可下、其他人一律「不存在」（不给 403 以免探测存在性）。
func TestShowcaseOpenArchive_Ownership(t *testing.T) {
	repo := newStubShowcaseRepo(&domain.ShowcaseSubmission{
		ID: 1, UserID: 7, ArchiveObjectKey: "showcase/a.zip",
	})
	uc := NewShowcaseUseCase(repo) // 无 storage：有权限时才会走到存储，这里只会验判定

	// 他人且非管理员：应报不存在（而不是走到存储层报别的错）。
	_, _, _, err := uc.OpenArchive(context.Background(), 1, 8, false)
	if !errors.Is(err, domain.ErrShowcaseNotFound) {
		t.Fatalf("他人下载应报不存在，实际 %v", err)
	}

	// 本人：会继续走到存储层；这里没注入 storage，报的是存储不可用而不是权限错误。
	_, _, _, err = uc.OpenArchive(context.Background(), 1, 7, false)
	if err == nil || errors.Is(err, domain.ErrShowcaseNotFound) {
		t.Fatalf("本人不应被归属判定拒绝，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "对象存储不可用") {
		t.Fatalf("应因缺少存储而失败，实际 %v", err)
	}

	// 管理员：同样放行到存储层。
	_, _, _, err = uc.OpenArchive(context.Background(), 1, 99, true)
	if err == nil || errors.Is(err, domain.ErrShowcaseNotFound) {
		t.Fatalf("管理员不应被归属判定拒绝，实际 %v", err)
	}
}
