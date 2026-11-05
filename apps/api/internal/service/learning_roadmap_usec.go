package service

import (
	"context"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// LearningRoadmapUseCase 学习路线管理（#223，ADR 0006）。
//
// 保存语义：整条覆盖 + version 乐观锁。一个页面里改标题、正文、阶段列表，
// 一次整体提交。不做版本历史——知识库要修订快照是因为它是团队文档、改错了要能追责；
// 路线是推荐内容，改错了直接改回来即可，少一张表、少两个接口、少一个历史面板。
type LearningRoadmapUseCase struct {
	roadmaps repository.LearningRoadmapRepo
	// attachments 正文附件绑定（复用 ADR 0004 的按正文重算逻辑）。
	// 可为 nil —— 为 nil 时跳过绑定，纯列表/读取用例不受影响。
	attachments *AttachmentUseCase
}

// NewLearningRoadmapUseCase 构造函数。
func NewLearningRoadmapUseCase(roadmaps repository.LearningRoadmapRepo) *LearningRoadmapUseCase {
	return &LearningRoadmapUseCase{roadmaps: roadmaps}
}

// WithAttachments 注入附件绑定能力（#223 正文插图）。
func (uc *LearningRoadmapUseCase) WithAttachments(attachments *AttachmentUseCase) *LearningRoadmapUseCase {
	uc.attachments = attachments
	return uc
}

// CreateLearningRoadmapInput 创建入参。
type CreateLearningRoadmapInput struct {
	Title         string
	Body          string
	DirectionCode string
	Stages        domain.RoadmapStages
}

// UpdateLearningRoadmapInput 整体保存入参。
type UpdateLearningRoadmapInput struct {
	Title         string
	Body          string
	DirectionCode string
	Stages        domain.RoadmapStages
}

// List 分页列出路线。
func (uc *LearningRoadmapUseCase) List(ctx context.Context, filter repository.LearningRoadmapListFilter, page, pageSize int) ([]*domain.LearningRoadmap, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return uc.roadmaps.List(ctx, filter, page, pageSize)
}

// Directions 方向筛选项（只含路线的方向）。
func (uc *LearningRoadmapUseCase) Directions(ctx context.Context) ([]repository.LearningRoadmapDirection, error) {
	return uc.roadmaps.Directions(ctx)
}

// Get 取单条。
func (uc *LearningRoadmapUseCase) Get(ctx context.Context, id int) (*domain.LearningRoadmap, error) {
	return uc.roadmaps.GetByID(ctx, id)
}

// Create 新建路线。
//
// 新建时没有「上一条」，因此所有阶段都当作新阶段（拿到新 id）。
func (uc *LearningRoadmapUseCase) Create(ctx context.Context, authorID int, in CreateLearningRoadmapInput) (*domain.LearningRoadmap, error) {
	title, err := normalizeRoadmapTitle(in.Title)
	if err != nil {
		return nil, err
	}
	direction, err := normalizeRoadmapDirection(in.DirectionCode)
	if err != nil {
		return nil, err
	}
	stages := domain.AlignStageIDs(in.Stages, nil)
	if err := domain.ValidateRoadmapStages(stages); err != nil {
		return nil, err
	}

	roadmap := &domain.LearningRoadmap{
		Title:         title,
		Body:          in.Body,
		DirectionCode: direction,
		Stages:        stages,
		Version:       1,
		AuthorID:      authorID,
	}
	if err := uc.roadmaps.Create(ctx, roadmap); err != nil {
		return nil, err
	}

	uc.bindRoadmapAttachments(ctx, authorID, roadmap.ID, roadmap.Body)
	return roadmap, nil
}

// Update 整体保存（带乐观锁）。
//
// 阶段 id 对齐以**库里当前**的阶段为准：这样「保留原有 id」只对真实存在的
// 阶段生效，前端伪造一个形状合法但库里没有的 id 会拿到新 id，
// 不会造出指向不存在阶段的进度锚点。
func (uc *LearningRoadmapUseCase) Update(ctx context.Context, id, operatorID int, in UpdateLearningRoadmapInput, expectedVersion int) (*domain.LearningRoadmap, error) {
	if expectedVersion <= 0 {
		// 缺 version 明确报参数错误，而不是假装成功：
		// 「读到什么就覆盖什么」会让乐观锁退化成「总是覆盖」。
		return nil, &domain.RoadmapValidationError{Reason: "缺少版本号，请刷新页面后重试"}
	}

	current, err := uc.roadmaps.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	title, err := normalizeRoadmapTitle(in.Title)
	if err != nil {
		return nil, err
	}
	direction, err := normalizeRoadmapDirection(in.DirectionCode)
	if err != nil {
		return nil, err
	}
	stages := domain.AlignStageIDs(in.Stages, current.Stages)
	if err := domain.ValidateRoadmapStages(stages); err != nil {
		return nil, err
	}

	current.Title = title
	current.Body = in.Body
	current.DirectionCode = direction
	current.Stages = stages

	if err := uc.roadmaps.Update(ctx, current, expectedVersion); err != nil {
		return nil, err
	}

	uc.bindRoadmapAttachments(ctx, operatorID, current.ID, current.Body)
	return uc.roadmaps.GetByID(ctx, id)
}

// Delete 软删并记录操作人。
func (uc *LearningRoadmapUseCase) Delete(ctx context.Context, id, operatorID int) error {
	return uc.roadmaps.SoftDelete(ctx, id, operatorID)
}

// bindRoadmapAttachments 按正文重算附件绑定（ADR 0004 §3）。
//
// 绑定失败不影响已保存的正文：路线内容已经落库，正文里的图片下次保存
// 或上传时会被重新判定，比让整次保存失败更合理。
func (uc *LearningRoadmapUseCase) bindRoadmapAttachments(ctx context.Context, userID, roadmapID int, body string) {
	if uc.attachments == nil {
		return
	}
	// 忽略错误是有意的：见方法注释。col 来自固定的 scope→列映射，非用户输入。
	_ = uc.attachments.RecomputeBindings(ctx, "roadmap_id", int64(roadmapID), userID, body)
}

/* ------------------------------------------------------------------ *
 * 入参规范化
 * ------------------------------------------------------------------ */

// normalizeRoadmapTitle 去空白并校验：空标题在数据库里也有 CHECK 兜底，
// 但这里要给出可读错误而不是让用户看到违反约束。
func normalizeRoadmapTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", &domain.RoadmapValidationError{Reason: "标题不能为空"}
	}
	if r := []rune(title); len(r) > domain.RoadmapTitleMaxRunes {
		return "", &domain.RoadmapValidationError{
			Reason: "标题最多 " + itoa(domain.RoadmapTitleMaxRunes) + " 个字符",
		}
	}
	return title, nil
}

// normalizeRoadmapDirection 方向是必填的单值软引用。
//
// 不校验方向是否存在于招新方向表（ADR 0006 决策 2）：方向是招新运营数据、
// 路线是长期内容，两者生命周期不同。这里只保证是「一个像样的 code」。
func normalizeRoadmapDirection(raw string) (string, error) {
	direction := strings.TrimSpace(raw)
	if direction == "" {
		return "", &domain.RoadmapValidationError{Reason: "请选择方向"}
	}
	return direction, nil
}
