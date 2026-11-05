package service

import (
	"context"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * #224：学习进度的 UPSERT、撤销与阶段存在性校验
 * ------------------------------------------------------------------ */

// stubProgressRepo 内存实现，语义与真实 UPSERT 一致（一人一路线一行）。
type stubProgressRepo struct {
	repository.RoadmapProgressRepo

	rows map[[2]int]*domain.UserRoadmapProgress
}

func newStubProgressRepo() *stubProgressRepo {
	return &stubProgressRepo{rows: map[[2]int]*domain.UserRoadmapProgress{}}
}

func (s *stubProgressRepo) ListByUser(_ context.Context, userID int) ([]*domain.UserRoadmapProgress, error) {
	var out []*domain.UserRoadmapProgress
	for k, v := range s.rows {
		if k[0] == userID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (s *stubProgressRepo) Get(_ context.Context, userID, roadmapID int) (*domain.UserRoadmapProgress, error) {
	return s.rows[[2]int{userID, roadmapID}], nil
}

func (s *stubProgressRepo) Upsert(_ context.Context, userID, roadmapID int, stageID string) error {
	s.rows[[2]int{userID, roadmapID}] = &domain.UserRoadmapProgress{
		UserID: userID, RoadmapID: roadmapID, StageID: stageID,
	}
	return nil
}

func (s *stubProgressRepo) Clear(_ context.Context, userID, roadmapID int) error {
	// 没有记录时静默成功：撤销本来就是幂等的。
	delete(s.rows, [2]int{userID, roadmapID})
	return nil
}

func progressRoadmap() *domain.LearningRoadmap {
	return &domain.LearningRoadmap{
		ID: 10, Title: "前端路线", DirectionCode: "front-end", Version: 1,
		Stages: domain.RoadmapStages{
			{ID: "s_aaaa1111", Title: "入门"},
			{ID: "s_bbbb2222", Title: "进阶"},
		},
	}
}

func newProgressUseCase(progress *stubProgressRepo, roadmaps ...*domain.LearningRoadmap) *RoadmapProgressUseCase {
	return NewRoadmapProgressUseCase(progress, newStubRoadmapRepo(roadmaps...))
}

func TestMarkProgress_StoresStageID(t *testing.T) {
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap())

	if err := uc.Mark(context.Background(), 7, 10, "s_bbbb2222"); err != nil {
		t.Fatalf("标记应成功，得到 %v", err)
	}
	got, _ := repo.Get(context.Background(), 7, 10)
	if got == nil || got.StageID != "s_bbbb2222" {
		t.Fatalf("进度应记为 s_bbbb2222，得到 %+v", got)
	}
}

func TestMarkProgress_UpsertKeepsOneRow(t *testing.T) {
	// 改标到另一个阶段：同一行被更新，不新增（一人一路线一行）。
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap())

	_ = uc.Mark(context.Background(), 7, 10, "s_aaaa1111")
	_ = uc.Mark(context.Background(), 7, 10, "s_bbbb2222")

	if len(repo.rows) != 1 {
		t.Fatalf("一人一路线应只有一行，实际 %d 行", len(repo.rows))
	}
	got, _ := repo.Get(context.Background(), 7, 10)
	if got.StageID != "s_bbbb2222" {
		t.Fatalf("应更新为新阶段，得到 %q", got.StageID)
	}
}

func TestMarkProgress_EmptyStageClears(t *testing.T) {
	// 再点同一阶段 = 撤销：前端传空串，后端删行。
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap())

	_ = uc.Mark(context.Background(), 7, 10, "s_aaaa1111")
	if err := uc.Mark(context.Background(), 7, 10, ""); err != nil {
		t.Fatalf("撤销应成功，得到 %v", err)
	}
	got, _ := repo.Get(context.Background(), 7, 10)
	if got != nil {
		t.Fatalf("撤销后不应有进度，得到 %+v", got)
	}
}

func TestMarkProgress_ClearIsIdempotent(t *testing.T) {
	// 从没标记过就点撤销：不应报错（两个标签页各点一次是正常的）。
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap())

	if err := uc.Mark(context.Background(), 7, 10, ""); err != nil {
		t.Fatalf("无进度时撤销应成功，得到 %v", err)
	}
}

func TestMarkProgress_RejectsUnknownStage(t *testing.T) {
	// 阶段不存在：不能写进库，否则进度指向一个不存在的阶段，
	// 页面会显示「已开始」却找不到对应阶段，且没有任何提示。
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap())

	err := uc.Mark(context.Background(), 7, 10, "s_notexist")
	if err == nil {
		t.Fatal("不存在的阶段应被拒绝")
	}
	if len(repo.rows) != 0 {
		t.Fatal("被拒绝时不应写库")
	}
}

func TestMarkProgress_RejectsCrossRoadmapStage(t *testing.T) {
	// 拿 A 路线的 stage id 标记 B 路线：跨路线串号应被挡住。
	other := &domain.LearningRoadmap{
		ID: 11, Title: "后端路线", DirectionCode: "back-end", Version: 1,
		Stages: domain.RoadmapStages{{ID: "s_cccc3333", Title: "后端入门"}},
	}
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap(), other)

	if err := uc.Mark(context.Background(), 7, 10, "s_cccc3333"); err == nil {
		t.Fatal("跨路线的阶段 id 应被拒绝")
	}
}

func TestMarkProgress_RejectsDeletedRoadmap(t *testing.T) {
	// 已软删的路线写进度没有意义（GetByID 会返回 ErrRoadmapNotFound）。
	repo := newStubProgressRepo()
	uc := NewRoadmapProgressUseCase(repo, newStubRoadmapRepo()) // 库里没有这条路线

	err := uc.Mark(context.Background(), 7, 10, "s_aaaa1111")
	if err == nil {
		t.Fatal("不存在的路线应被拒绝")
	}
}

func TestListMine_ScopedToUser(t *testing.T) {
	// 进度只属于本人：user 7 看不到 user 8 的进度。
	repo := newStubProgressRepo()
	uc := newProgressUseCase(repo, progressRoadmap())

	_ = uc.Mark(context.Background(), 7, 10, "s_aaaa1111")

	mine7, _ := uc.ListMine(context.Background(), 7)
	if mine7[10] != "s_aaaa1111" {
		t.Fatalf("user 7 应看到自己的进度，得到 %+v", mine7)
	}
	mine8, _ := uc.ListMine(context.Background(), 8)
	if len(mine8) != 0 {
		t.Fatalf("user 8 不应看到他人进度，得到 %+v", mine8)
	}
}

func TestListMine_EmptyIsNotError(t *testing.T) {
	// 新用户还没开始学任何路线：空进度是合法状态，不是错误。
	uc := newProgressUseCase(newStubProgressRepo(), progressRoadmap())

	got, err := uc.ListMine(context.Background(), 99)
	if err != nil {
		t.Fatalf("空进度不应报错，得到 %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("应为空，得到 %+v", got)
	}
}

func TestMarkProgress_SurvivesStageReorder(t *testing.T) {
	// AC 核心：管理员重排阶段后，进度仍指向原来那个阶段。
	//
	// 进度记的是阶段 id 而不是序号，所以重排（数组顺序变化）不影响它。
	repo := newStubProgressRepo()
	roadmap := progressRoadmap()
	uc := newProgressUseCase(repo, roadmap)

	// 标记「进阶」（原第 2 阶段）。
	_ = uc.Mark(context.Background(), 7, 10, "s_bbbb2222")

	// 管理员把「进阶」移到第一位。
	reordered := progressRoadmap()
	reordered.Stages = domain.RoadmapStages{
		{ID: "s_bbbb2222", Title: "进阶"},
		{ID: "s_aaaa1111", Title: "入门"},
	}
	uc2 := newProgressUseCase(repo, reordered)

	mine, _ := uc2.ListMine(context.Background(), 7)
	if mine[10] != "s_bbbb2222" {
		t.Fatalf("重排后进度应仍是 s_bbbb2222，得到 %q", mine[10])
	}
	// 确认它现在确实指向「进阶」——即第一个阶段。
	if reordered.Stages[0].ID != mine[10] {
		t.Fatal("进度应指向重排后的第一个阶段")
	}
}
