package service

import (
	"context"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// RoadmapProgressUseCase 用户学习进度（#224，ADR 0006 决策 4）。
//
// **进度只属于本人**：所有方法都以调用方传入的 userID 为作用域，
// 而 userID 来自会话（handler 层），不是请求体——结构上无法越权读写他人进度。
type RoadmapProgressUseCase struct {
	progress repository.RoadmapProgressRepo
	roadmaps repository.LearningRoadmapRepo
}

// NewRoadmapProgressUseCase 构造函数。
func NewRoadmapProgressUseCase(
	progress repository.RoadmapProgressRepo,
	roadmaps repository.LearningRoadmapRepo,
) *RoadmapProgressUseCase {
	return &RoadmapProgressUseCase{progress: progress, roadmaps: roadmaps}
}

// ListMine 返回「路线 id → 阶段 id」的映射，供列表与详情标注当前阶段。
func (uc *RoadmapProgressUseCase) ListMine(ctx context.Context, userID int) (map[int]string, error) {
	rows, err := uc.progress.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(rows))
	for _, r := range rows {
		out[r.RoadmapID] = r.StageID
	}
	return out, nil
}

// Mark 标记当前阶段；stageID 为空表示撤销。
//
// stageID 必须是**该路线当前阶段列表里真实存在**的 id：
//   - 不校验的话，任意字符串都能写进库，用户的进度会指向一个不存在的阶段，
//     页面显示「已开始」却找不到对应阶段，而且没有任何提示；
//   - 也挡住「拿 A 路线的 stage id 标记 B 路线」这种跨路线串号。
//
// 撤销（空 stageID）不校验存在性：它只是把行删掉，不引入新的引用。
func (uc *RoadmapProgressUseCase) Mark(ctx context.Context, userID, roadmapID int, stageID string) error {
	// 先确认路线存在且未软删：对已删除的路线写进度没有意义。
	roadmap, err := uc.roadmaps.GetByID(ctx, roadmapID)
	if err != nil {
		return err
	}

	if stageID == "" {
		return uc.progress.Clear(ctx, userID, roadmapID)
	}

	if !roadmapHasStage(roadmap, stageID) {
		return &domain.RoadmapValidationError{Reason: "该阶段不存在，请刷新页面后重试"}
	}
	return uc.progress.Upsert(ctx, userID, roadmapID, stageID)
}

// roadmapHasStage 判断阶段 id 是否在这条路线的当前阶段列表里。
func roadmapHasStage(roadmap *domain.LearningRoadmap, stageID string) bool {
	for _, s := range roadmap.Stages {
		if s.ID == stageID {
			return true
		}
	}
	return false
}
