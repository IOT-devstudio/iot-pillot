package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 桩：候选人目录
 * ------------------------------------------------------------------ */

// stubCandidateDirectory 满足 StudioMemberCandidateDirectory。
//
// 只实现「按 id 取候选人」与「按 id 取申请」两个读方法：
// 转入动作只需要这两个数据源，写成员走成员仓储。
type stubCandidateDirectory struct {
	candidates map[int]*domain.Candidate
	apps       map[int]*domain.Application
}

func (s stubCandidateDirectory) GetByID(_ context.Context, id int) (*domain.Candidate, error) {
	if c, ok := s.candidates[id]; ok {
		return c, nil
	}
	return nil, repository.ErrCandidateNotFound
}

func (s stubCandidateDirectory) GetApplicationByID(_ context.Context, id int) (*domain.Application, error) {
	if a, ok := s.apps[id]; ok {
		return a, nil
	}
	return nil, repository.ErrApplicationNotFound
}

// newMemberUseCaseWithCandidates 构造带候选人目录的用例。
func newMemberUseCaseWithCandidates(t *testing.T, candidate *domain.Candidate, app *domain.Application) (*StudioMemberUseCase, *stubStudioMemberRepo) {
	t.Helper()
	repo := newStubStudioMemberRepo()
	accounts := stubAccountDirectory{users: map[int]*domain.User{
		app.UserID: {ID: app.UserID, Name: app.Name},
	}}
	dir := stubCandidateDirectory{
		candidates: map[int]*domain.Candidate{candidate.ID: candidate},
		apps:       map[int]*domain.Application{app.ID: app},
	}
	uc := NewStudioMemberUseCase(repo, accounts, dir)
	uc.now = func() time.Time { return time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC) }
	return uc, repo
}

func hiredCandidate() (*domain.Candidate, *domain.Application) {
	app := &domain.Application{
		ID:            900,
		UserID:        7,
		RoundID:       1,
		DirectionCode: "front-end",
		Status:        domain.ApplicationAccepted,
		Name:          "张三",
		Email:         "zhangsan@example.test",
	}
	candidate := &domain.Candidate{
		ID:            55,
		ApplicationID: app.ID,
		UserID:        app.UserID,
		Status:        domain.CandidateHired,
	}
	return candidate, app
}

// convertFields 转入时管理员必须补全的成员必填项。
func convertFields() domain.StudioMemberFields {
	return domain.StudioMemberFields{
		BirthYear:    2003,
		Cohort:       "2024 级",
		RoleInStudio: "前端成员",
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：hired 成功转入
 * ------------------------------------------------------------------ */

func TestConvertCandidateToMember_HiredSucceeds(t *testing.T) {
	candidate, app := hiredCandidate()
	uc, repo := newMemberUseCaseWithCandidates(t, candidate, app)

	member, err := uc.ConvertFromCandidate(context.Background(), ConvertCandidateInput{
		CandidateID: candidate.ID,
		Fields:      convertFields(),
	})
	if err != nil {
		t.Fatalf("hired 候选人应可转入，实际 %v", err)
	}

	// 姓名与方向来自申请，不该让管理员重填——重填就有填错的可能。
	if member.Name != "张三" {
		t.Errorf("姓名应取自申请，实际 %q", member.Name)
	}
	if member.Direction != "front-end" {
		t.Errorf("方向应取自申请，实际 %q", member.Direction)
	}
	// 溯源必须写入：这是「防重复转入」与「来源可区分」的唯一依据。
	if member.SourceCandidateID == nil || *member.SourceCandidateID != candidate.ID {
		t.Errorf("应记录来源候选人 %d，实际 %v", candidate.ID, member.SourceCandidateID)
	}
	// 候选人本身有系统账号，成员档案应一并绑定。
	if member.UserID == nil || *member.UserID != app.UserID {
		t.Errorf("应绑定候选人账号 %d，实际 %v", app.UserID, member.UserID)
	}
	if len(repo.members) != 1 {
		t.Errorf("应写入 1 条成员档案，实际 %d", len(repo.members))
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：非 hired 失败
 * ------------------------------------------------------------------ */

func TestConvertCandidateToMember_NonHiredRejected(t *testing.T) {
	for _, status := range []domain.CandidateStatus{
		domain.CandidateInterviewing,
		domain.CandidateOffered,
		domain.CandidateRejected,
	} {
		t.Run(string(status), func(t *testing.T) {
			candidate, app := hiredCandidate()
			candidate.Status = status
			uc, repo := newMemberUseCaseWithCandidates(t, candidate, app)

			_, err := uc.ConvertFromCandidate(context.Background(), ConvertCandidateInput{
				CandidateID: candidate.ID,
				Fields:      convertFields(),
			})
			if !errors.Is(err, ErrCandidateNotHired) {
				t.Fatalf("%s 状态应被拒绝，实际 %v", status, err)
			}
			if len(repo.members) != 0 {
				t.Error("拒绝时不应写库（不能部分写入）")
			}
		})
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：重复转入被拒绝且可理解
 * ------------------------------------------------------------------ */

func TestConvertCandidateToMember_DuplicateRejected(t *testing.T) {
	candidate, app := hiredCandidate()
	uc, repo := newMemberUseCaseWithCandidates(t, candidate, app)

	if _, err := uc.ConvertFromCandidate(context.Background(), ConvertCandidateInput{
		CandidateID: candidate.ID,
		Fields:      convertFields(),
	}); err != nil {
		t.Fatal(err)
	}

	// 第二次转入：必须被明确拒绝，而不是建出第二条档案。
	_, err := uc.ConvertFromCandidate(context.Background(), ConvertCandidateInput{
		CandidateID: candidate.ID,
		Fields:      convertFields(),
	})
	// 前置检查命中 → 明确的「已转过」错误（不是「同名同出生年」）。
	// 数据库的 source_candidate_id 部分唯一索引是最后一道防线，
	// 但只有它的话，重复点击会先写一条再撞约束回滚。
	if !errors.Is(err, ErrCandidateAlreadyConverted) {
		t.Fatalf("重复转入应被拒绝，实际 %v", err)
	}
	if len(repo.members) != 1 {
		t.Errorf("重复转入不应新增档案，实际 %d 条", len(repo.members))
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：缺必填补全字段时明确报错，且不部分写入
 * ------------------------------------------------------------------ */

func TestConvertCandidateToMember_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		patch func(*domain.StudioMemberFields)
	}{
		{"缺出生年", func(f *domain.StudioMemberFields) { f.BirthYear = 0 }},
		{"缺届别", func(f *domain.StudioMemberFields) { f.Cohort = "" }},
		{"缺职能", func(f *domain.StudioMemberFields) { f.RoleInStudio = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate, app := hiredCandidate()
			uc, repo := newMemberUseCaseWithCandidates(t, candidate, app)
			fields := convertFields()
			tc.patch(&fields)

			_, err := uc.ConvertFromCandidate(context.Background(), ConvertCandidateInput{
				CandidateID: candidate.ID,
				Fields:      fields,
			})
			if !errors.Is(err, ErrStudioMemberInvalid) {
				t.Fatalf("缺必填项应返回 ErrStudioMemberInvalid，实际 %v", err)
			}
			if len(repo.members) != 0 {
				t.Error("校验失败不应写库")
			}
		})
	}
}

func TestConvertCandidateToMember_CandidateNotFound(t *testing.T) {
	candidate, app := hiredCandidate()
	uc, _ := newMemberUseCaseWithCandidates(t, candidate, app)

	_, err := uc.ConvertFromCandidate(context.Background(), ConvertCandidateInput{
		CandidateID: 999,
		Fields:      convertFields(),
	})
	if !errors.Is(err, ErrCandidateMissing) {
		t.Fatalf("不存在的候选人应返回 ErrCandidateMissing，实际 %v", err)
	}
}
