package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 对外错误
 * ------------------------------------------------------------------ */

// ErrStudioKnowledgeInvalid 字段不合法（直接复用领域层哨兵）。
//
// 不另起一个同名变量：两处各自 errors.New 出来的是两个不同的值，
// handler 用 errors.Is 判断时会漏掉其中一个，校验失败就变成 500。
var ErrStudioKnowledgeInvalid = domain.ErrStudioKnowledgeInvalid

// ErrStudioKnowledgeParentInvalid 父条目不存在或会形成环。
//
// 两种原因合成一个对外错误：对调用方来说「你选的父条目不可用」是同一件事，
// 而区分「不存在」与「成环」只会暴露树结构细节，对管理员没有帮助。
var ErrStudioKnowledgeParentInvalid = errors.New("父条目不可用：请选择一个存在且不是自身子条目的条目")

// ErrStudioKnowledgeVersionConflict 并发保存冲突（直接复用仓储哨兵）。
var ErrStudioKnowledgeVersionConflict = repository.ErrKnowledgeVersionConflict

/* ------------------------------------------------------------------ *
 * 入参
 * ------------------------------------------------------------------ */

// CreateStudioKnowledgeInput 创建条目入参。
//
// AuthorID 来自鉴权身份（不接收请求体里的 author_id），
// 避免「替别人署名」这种越权写。
type CreateStudioKnowledgeInput struct {
	Fields   domain.StudioKnowledgeFields
	AuthorID int
}

// UpdateStudioKnowledgeInput 更新条目入参：nil 表示「不改这一项」。
//
// Body 与 Title 分开用指针：正文可以是空串（先建个骨架、回头再写），
// 但「没传」与「传了空串」必须能区分，否则每次保存都会把正文清空。
type UpdateStudioKnowledgeInput struct {
	Title      *string
	Body       *string
	Permission *domain.StudioKnowledgePermission
	// ParentID 三态：nil = 不改；非 nil 且值为 nil = 移到顶层；
	// 非 nil 且值非 nil = 挂到该条目下。
	ParentID **int
	EditorID int
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// StudioKnowledgeUseCase 工作室知识库（#196）。
//
// 保存语义：每次成功保存 → version + 1 且写入同版本快照。
// **没有**一键回滚（ADR 0001 §3 明确拒绝）：恢复 = 基于旧版内容再保存一版，
// 于是历史始终是追加式的，不会被一次「回滚」抹掉中间过程。
type StudioKnowledgeUseCase struct {
	entries repository.StudioKnowledgeRepo
}

// NewStudioKnowledgeUseCase 构造函数。
func NewStudioKnowledgeUseCase(entries repository.StudioKnowledgeRepo) *StudioKnowledgeUseCase {
	return &StudioKnowledgeUseCase{entries: entries}
}

// Create 创建条目并写入 v1 快照。
func (uc *StudioKnowledgeUseCase) Create(ctx context.Context, in CreateStudioKnowledgeInput) (*domain.StudioKnowledgeEntry, error) {
	fields := in.Fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}
	if err := uc.checkParent(ctx, fields.ParentID, 0); err != nil {
		return nil, err
	}

	entry := &domain.StudioKnowledgeEntry{AuthorID: in.AuthorID}
	fields.ApplyTo(entry)

	// v1 快照就是创建时的内容：创建本身也是一次「版本」，
	// 否则 history 里会缺第一版，管理员看到的版本号从 1 开始却没有 1 的内容。
	revision := &domain.StudioKnowledgeRevision{
		Title:    fields.Title,
		Body:     fields.Body,
		EditorID: in.AuthorID,
	}
	if err := uc.entries.CreateEntry(ctx, entry, revision); err != nil {
		return nil, translateKnowledgeError(err)
	}
	return entry, nil
}

// List 分页列出条目。
func (uc *StudioKnowledgeUseCase) List(ctx context.Context, filter repository.StudioKnowledgeListFilter, page int, pageSize int) ([]*domain.StudioKnowledgeEntry, int64, error) {
	return uc.entries.ListEntries(ctx, filter, page, pageSize)
}

// Get 取条目当前正文。
func (uc *StudioKnowledgeUseCase) Get(ctx context.Context, id int) (*domain.StudioKnowledgeEntry, error) {
	return uc.entries.GetEntry(ctx, id)
}

// Update 保存正文：version + 1 并写入新快照。
//
// expectedVersion 由调用方给出（管理端页面带着它当前看到的版本号）：
// 这是乐观锁，两个管理员同时编辑同一版时，后保存的那个会收到冲突而不是
// 悄悄覆盖前一个人的改动。
func (uc *StudioKnowledgeUseCase) Update(ctx context.Context, id int, in UpdateStudioKnowledgeInput, expectedVersion int) (*domain.StudioKnowledgeEntry, error) {
	current, err := uc.entries.GetEntry(ctx, id)
	if err != nil {
		return nil, translateKnowledgeError(err)
	}

	fields := domain.StudioKnowledgeFields{
		Title:      current.Title,
		Body:       current.Body,
		Permission: current.Permission,
		ParentID:   current.ParentID,
	}
	if in.Title != nil {
		fields.Title = *in.Title
	}
	if in.Body != nil {
		fields.Body = *in.Body
	}
	if in.Permission != nil {
		fields.Permission = *in.Permission
	}
	if in.ParentID != nil {
		fields.ParentID = *in.ParentID
	}

	fields = fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}
	if err := uc.checkParent(ctx, fields.ParentID, id); err != nil {
		return nil, err
	}

	// 版本号必须由调用方显式给出。
	//
	// 这里**不能**在缺省时默默回退到 current.Version —— 那等于
	// 「读到什么就覆盖什么」，乐观锁直接退化成「总是覆盖」：
	// 两个管理员同时编辑时，后保存的会把前一个人的改动静默抹掉，
	// 而且历史里看不出发生过覆盖（只有一条内容不同的新快照）。
	// 缺 version 的调用方应当收到明确的参数错误，而不是一个「成功」的假象。
	if expectedVersion <= 0 {
		return nil, fmt.Errorf("%w: version 必填（用于并发保护），请带上你读到的版本号", ErrStudioKnowledgeInvalid)
	}

	entry := &domain.StudioKnowledgeEntry{ID: id}
	fields.ApplyTo(entry)
	revision := &domain.StudioKnowledgeRevision{
		Title:    fields.Title,
		Body:     fields.Body,
		EditorID: in.EditorID,
	}

	if err := uc.entries.UpdateEntry(ctx, entry, revision, expectedVersion); err != nil {
		return nil, translateKnowledgeError(err)
	}
	// 回读一次：updated_at 与 version 由数据库决定，直接返回内存对象会过时。
	return uc.entries.GetEntry(ctx, id)
}

// Delete 删除条目（修订级联删除，子条目的 parent 置空）。
func (uc *StudioKnowledgeUseCase) Delete(ctx context.Context, id int) error {
	if err := uc.entries.DeleteEntry(ctx, id); err != nil {
		return translateKnowledgeError(err)
	}
	return nil
}

// Revisions 列出某条目的全部修订（版本倒序）。
func (uc *StudioKnowledgeUseCase) Revisions(ctx context.Context, entryID int) ([]*domain.StudioKnowledgeRevision, error) {
	// 先确认条目存在：不存在时返回空列表会让调用方以为「这条目没有历史」，
	// 而实际是「这条目根本不存在」——两者该给出不同的提示。
	if _, err := uc.entries.GetEntry(ctx, entryID); err != nil {
		return nil, translateKnowledgeError(err)
	}
	return uc.entries.ListRevisions(ctx, entryID)
}

// Revision 取指定版本快照。
func (uc *StudioKnowledgeUseCase) Revision(ctx context.Context, entryID int, version int) (*domain.StudioKnowledgeRevision, error) {
	return uc.entries.GetRevision(ctx, entryID, version)
}

// checkParent 校验可选的父条目：必须存在，且不能形成环。
//
// selfID 为 0 表示创建（此时不可能成环，只需存在性校验）；
// 更新时传入自身 id，用来挡住「自己指自己」与「指到自己的子孙」。
func (uc *StudioKnowledgeUseCase) checkParent(ctx context.Context, parentID *int, selfID int) error {
	if parentID == nil {
		return nil
	}
	if _, err := uc.entries.GetEntry(ctx, *parentID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeEntryNotFound) {
			return ErrStudioKnowledgeParentInvalid
		}
		return err
	}
	if selfID == 0 {
		return nil
	}
	// 把自己挂到自己下面，或挂到自己的子孙下面，都会让树变成环。
	descendant, err := uc.entries.IsDescendant(ctx, selfID, *parentID)
	if err != nil {
		return err
	}
	if descendant {
		return ErrStudioKnowledgeParentInvalid
	}
	return nil
}

// translateKnowledgeError 把仓储哨兵翻译成对外错误。
//
// 父条目错误统一成 ErrStudioKnowledgeParentInvalid：仓储层区分「不存在」，
// 但服务层的环检测用的是同一条路径，对调用方来说都是「父条目不可用」。
func translateKnowledgeError(err error) error {
	switch {
	case errors.Is(err, repository.ErrKnowledgeParentNotFound),
		errors.Is(err, repository.ErrKnowledgeParentCycle):
		return ErrStudioKnowledgeParentInvalid
	default:
		return err
	}
}
