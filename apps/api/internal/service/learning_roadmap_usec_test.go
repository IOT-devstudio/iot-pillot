package service

import (
	"context"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * #223：学习路线服务的版本冲突与阶段 id 对齐
 * ------------------------------------------------------------------ */

// stubRoadmapRepo 内存实现，只实现被测路径需要的方法。
type stubRoadmapRepo struct {
	repository.LearningRoadmapRepo

	rows map[int]*domain.LearningRoadmap
	// updateCalls 记录 Update 被调用的次数（验证冲突时不写库）。
	updateCalls int
}

func newStubRoadmapRepo(rows ...*domain.LearningRoadmap) *stubRoadmapRepo {
	m := make(map[int]*domain.LearningRoadmap, len(rows))
	for _, r := range rows {
		m[r.ID] = r
	}
	return &stubRoadmapRepo{rows: m}
}

func (s *stubRoadmapRepo) GetByID(_ context.Context, id int) (*domain.LearningRoadmap, error) {
	r, ok := s.rows[id]
	if !ok {
		return nil, domain.ErrRoadmapNotFound
	}
	// 返回副本，避免被测代码直接改动「库里的」对象。
	cp := *r
	return &cp, nil
}

func (s *stubRoadmapRepo) Create(_ context.Context, r *domain.LearningRoadmap) error {
	r.ID = len(s.rows) + 1
	if r.Version == 0 {
		r.Version = 1
	}
	cp := *r
	s.rows[r.ID] = &cp
	return nil
}

func (s *stubRoadmapRepo) Update(_ context.Context, r *domain.LearningRoadmap, expectedVersion int) error {
	s.updateCalls++
	cur, ok := s.rows[r.ID]
	if !ok || cur.Version != expectedVersion {
		return domain.ErrRoadmapVersionConflict
	}
	r.Version = expectedVersion + 1
	cp := *r
	s.rows[r.ID] = &cp
	return nil
}

func (s *stubRoadmapRepo) SoftDelete(_ context.Context, id, _ int) error {
	if _, ok := s.rows[id]; !ok {
		return domain.ErrRoadmapNotFound
	}
	delete(s.rows, id)
	return nil
}

func existingRoadmap() *domain.LearningRoadmap {
	return &domain.LearningRoadmap{
		ID: 1, Title: "前端路线", Body: "body", DirectionCode: "front-end", Version: 3,
		Stages: domain.RoadmapStages{
			{ID: "s_aaaa1111", Title: "入门"},
			{ID: "s_bbbb2222", Title: "进阶"},
		},
	}
}

func TestUpdateLearningRoadmap_VersionConflict(t *testing.T) {
	// 乐观锁：带着过期版本号保存必须冲突，且**不写库**。
	// 用过期 version 调用时仓储层才拦得住；服务层若先覆盖再校验就晚了。
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	_, err := uc.Update(context.Background(), 1, 9, UpdateLearningRoadmapInput{
		Title: "改后的标题", DirectionCode: "front-end",
	}, 2) // 库里是 3
	if err == nil {
		t.Fatal("过期版本号应返回冲突")
	}
	// 这里**允许**调用 Update：版本判定必须与写入原子完成，
	// 只能在仓储层的条件更新里做（先读再比存在检查与写入之间的窗口）。
	// 关键是最终**没写进去**——库里的内容与版本都不变。
	cur, _ := repo.GetByID(context.Background(), 1)
	if cur.Version != 3 || cur.Title != "前端路线" {
		t.Fatalf("冲突后库内容不应改变，得到 version=%d title=%q", cur.Version, cur.Title)
	}
}

func TestUpdateLearningRoadmap_MissingVersion(t *testing.T) {
	// 缺 version 明确报参数错误，而不是假装成功：
	// 「读到什么就覆盖什么」会让乐观锁退化成「总是覆盖」。
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	_, err := uc.Update(context.Background(), 1, 9, UpdateLearningRoadmapInput{
		Title: "标题", DirectionCode: "front-end",
	}, 0)
	if err == nil {
		t.Fatal("缺版本号应返回错误")
	}
	if repo.updateCalls != 0 {
		t.Fatal("缺版本号不应写库")
	}
}

func TestUpdateLearningRoadmap_KeepsStageIDs(t *testing.T) {
	// 阶段 id 对齐以**库里的**为准：原有阶段保留 id，新阶段拿新 id。
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	got, err := uc.Update(context.Background(), 1, 9, UpdateLearningRoadmapInput{
		Title:         "前端路线",
		DirectionCode: "front-end",
		Stages: domain.RoadmapStages{
			{ID: "s_bbbb2222", Title: "进阶（移到第一位）"},
			{ID: "", Title: "全新阶段"},
		},
	}, 3)
	if err != nil {
		t.Fatalf("保存应成功，得到 %v", err)
	}

	if len(got.Stages) != 2 {
		t.Fatalf("应有 2 个阶段，得到 %d", len(got.Stages))
	}
	// 重排后原阶段仍指向同一个 id —— 这正是「用户进度按 id 记」的价值。
	if got.Stages[0].ID != "s_bbbb2222" {
		t.Fatalf("原阶段应保留 id，得到 %q", got.Stages[0].ID)
	}
	if got.Stages[1].ID == "" || got.Stages[1].ID == "s_bbbb2222" {
		t.Fatalf("新阶段应拿到不同的新 id，得到 %q", got.Stages[1].ID)
	}
	if got.Version != 4 {
		t.Fatalf("保存后版本应 +1 到 4，得到 %d", got.Version)
	}
}

func TestUpdateLearningRoadmap_DropsRemovedStage(t *testing.T) {
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	got, err := uc.Update(context.Background(), 1, 9, UpdateLearningRoadmapInput{
		Title:         "前端路线",
		DirectionCode: "front-end",
		Stages:        domain.RoadmapStages{{ID: "s_aaaa1111", Title: "只留这个"}},
	}, 3)
	if err != nil {
		t.Fatalf("保存应成功，得到 %v", err)
	}
	if len(got.Stages) != 1 || got.Stages[0].ID != "s_aaaa1111" {
		t.Fatalf("被删的阶段不应出现，得到 %+v", got.Stages)
	}
}

func TestUpdateLearningRoadmap_RejectsOverLimit(t *testing.T) {
	// 超限在服务层拦，落库前就失败（数据库只有数组类型约束）。
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	stages := make(domain.RoadmapStages, 21)
	for i := range stages {
		stages[i] = domain.RoadmapStage{Title: "阶段"}
	}
	_, err := uc.Update(context.Background(), 1, 9, UpdateLearningRoadmapInput{
		Title: "标题", DirectionCode: "front-end", Stages: stages,
	}, 3)
	if err == nil {
		t.Fatal("21 个阶段应被拒绝")
	}
	if repo.updateCalls != 0 {
		t.Fatal("超限不应写库")
	}
}

func TestUpdateLearningRoadmap_RejectsBadURL(t *testing.T) {
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	_, err := uc.Update(context.Background(), 1, 9, UpdateLearningRoadmapInput{
		Title: "标题", DirectionCode: "front-end",
		Stages: domain.RoadmapStages{{
			Title:     "阶段",
			Resources: []domain.RoadmapResource{{Title: "坏", URL: "javascript:alert(1)"}},
		}},
	}, 3)
	if err == nil {
		t.Fatal("javascript: 链接应在服务层就被拒绝")
	}
	if repo.updateCalls != 0 {
		t.Fatal("非法链接不应写库")
	}
}

func TestCreateLearningRoadmap_AssignsIDsToAllStages(t *testing.T) {
	// 新建时没有「上一条」，所有阶段都是新阶段。
	repo := newStubRoadmapRepo()
	uc := NewLearningRoadmapUseCase(repo)

	got, err := uc.Create(context.Background(), 7, CreateLearningRoadmapInput{
		Title:         "后端路线",
		DirectionCode: "back-end",
		Stages:        domain.RoadmapStages{{Title: "一"}, {Title: "二"}},
	})
	if err != nil {
		t.Fatalf("创建应成功，得到 %v", err)
	}
	for i, s := range got.Stages {
		if !domain.IsValidRoadmapStageID(s.ID) {
			t.Fatalf("第 %d 个阶段应拿到合法 id，得到 %q", i+1, s.ID)
		}
	}
	if got.Version != 1 {
		t.Fatalf("新建应为 v1，得到 %d", got.Version)
	}
	if got.AuthorID != 7 {
		t.Fatalf("应记录作者，得到 %d", got.AuthorID)
	}
}

func TestCreateLearningRoadmap_RequiresTitleAndDirection(t *testing.T) {
	uc := NewLearningRoadmapUseCase(newStubRoadmapRepo())

	if _, err := uc.Create(context.Background(), 7, CreateLearningRoadmapInput{
		Title: "  ", DirectionCode: "front-end",
	}); err == nil {
		t.Fatal("空标题应被拒绝")
	}
	if _, err := uc.Create(context.Background(), 7, CreateLearningRoadmapInput{
		Title: "标题", DirectionCode: "",
	}); err == nil {
		t.Fatal("空方向应被拒绝")
	}
}

func TestDeleteLearningRoadmap(t *testing.T) {
	repo := newStubRoadmapRepo(existingRoadmap())
	uc := NewLearningRoadmapUseCase(repo)

	if err := uc.Delete(context.Background(), 1, 9); err != nil {
		t.Fatalf("删除应成功，得到 %v", err)
	}
	if _, err := uc.Get(context.Background(), 1); err == nil {
		t.Fatal("删除后应查不到")
	}
}
