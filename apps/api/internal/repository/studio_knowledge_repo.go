package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

/* ------------------------------------------------------------------ *
 * 哨兵错误
 * ------------------------------------------------------------------ */

// ErrKnowledgeEntryNotFound 知识库条目不存在。
var ErrKnowledgeEntryNotFound = errors.New("知识库条目不存在")

// ErrKnowledgeRevisionNotFound 指定的版本快照不存在。
var ErrKnowledgeRevisionNotFound = errors.New("该版本的知识库修订不存在")

// ErrKnowledgeParentNotFound parent_id 指向的条目不存在。
var ErrKnowledgeParentNotFound = errors.New("父条目不存在")

// ErrKnowledgeParentCycle parent_id 会形成环（自己指向自己，或指到自己的子孙）。
var ErrKnowledgeParentCycle = errors.New("父条目不能是自己或自己的子条目")

/* ------------------------------------------------------------------ *
 * 过滤条件
 * ------------------------------------------------------------------ */

// StudioKnowledgeListFilter 知识库列表的筛选条件，零值表示不额外限制。
type StudioKnowledgeListFilter struct {
	// Query 关键词：标题 / 正文的模糊匹配。
	Query string
	// Permission 权限精确匹配（空串 = 不筛选）。
	Permission string
	// ParentID 三态：
	//   nil      = 不按父子筛选（返回全部，含子条目）
	//   指向 nil = 只看顶层（parent_id IS NULL）
	//   指向数字 = 只看该条目的直接子条目
	//
	// 用 **int 是因为「不筛选」与「只看顶层」都是合法且有区别的语义，
	// 而 ParentID 本身可空——两层指针是唯一能同时表达三态的形状。
	ParentID **int
}

/* ------------------------------------------------------------------ *
 * 仓储接口
 * ------------------------------------------------------------------ */

// StudioKnowledgeRepo 知识库条目与修订的持久化（#196）。
//
// 条目与修订放在同一个接口下：它们的写操作共享事务封闭语义——
// 保存 = 条目 version+1 **且** 插入一条同版本快照，两件事必须同生共死。
// 拆成两个仓储会让「只成功了一半」成为可能，而那种状态没有任何修复手段。
type StudioKnowledgeRepo interface {
	// CreateEntry 在同一事务里写入条目（version = 1）与它的 v1 快照。
	// parent_id 不存在返回 ErrKnowledgeParentNotFound。
	CreateEntry(ctx context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision) error
	// GetEntry 按 id 取条目；不存在返回 ErrKnowledgeEntryNotFound。
	GetEntry(ctx context.Context, id int) (*domain.StudioKnowledgeEntry, error)
	// ListEntries 分页列出条目（新建在前）。
	ListEntries(ctx context.Context, filter StudioKnowledgeListFilter, page int, pageSize int) ([]*domain.StudioKnowledgeEntry, int64, error)
	// UpdateEntry 在同一事务里保存正文（version+1）并插入对应快照。
	//
	// entry.Version 由调用方给出**期望的旧版本号**，仓储据此做条件更新：
	// 并发保存时只有一个能成功，另一个拿到 ErrKnowledgeVersionConflict。
	// 这样版本号不会因为两个人同时编辑而出现跳号或丢版本。
	UpdateEntry(ctx context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision, expectedVersion int) error
	// DeleteEntry 删除条目；修订由迁移的 ON DELETE CASCADE 一并删除。
	DeleteEntry(ctx context.Context, id int) error
	// ListRevisions 按版本倒序列出某条目的全部修订。
	ListRevisions(ctx context.Context, entryID int) ([]*domain.StudioKnowledgeRevision, error)
	// GetRevision 取某条目的指定版本快照。
	GetRevision(ctx context.Context, entryID int, version int) (*domain.StudioKnowledgeRevision, error)
	// IsDescendant 报告 candidate 是否是 ancestor 的子孙（含自身）。
	//
	// 用于 parent_id 的环检测：把 A 的 parent 设成 A 的子孙会让树变成环，
	// 之后的递归读取就会无限循环。
	IsDescendant(ctx context.Context, ancestorID int, candidateID int) (bool, error)
}

var _ StudioKnowledgeRepo = (*gormStudioKnowledgeRepo)(nil)

// ErrKnowledgeVersionConflict 并发保存导致版本号冲突。
var ErrKnowledgeVersionConflict = errors.New("该条目已被其他人修改，请刷新后重试")

type gormStudioKnowledgeRepo struct {
	db *gorm.DB
}

// NewStudioKnowledgeRepo 构造函数。
func NewStudioKnowledgeRepo(db *gorm.DB) StudioKnowledgeRepo {
	return &gormStudioKnowledgeRepo{db: db}
}

/* ------------------------------------------------------------------ *
 * 实现
 * ------------------------------------------------------------------ */

func (r *gormStudioKnowledgeRepo) CreateEntry(ctx context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entry.ParentID != nil {
			var count int64
			if err := tx.Model(&domain.StudioKnowledgeEntry{}).
				Where("id = ?", *entry.ParentID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrKnowledgeParentNotFound
			}
		}

		entry.Version = 1
		if err := tx.Create(entry).Error; err != nil {
			return err
		}

		revision.EntryID = entry.ID
		revision.Version = entry.Version
		return tx.Create(revision).Error
	})
}

func (r *gormStudioKnowledgeRepo) GetEntry(ctx context.Context, id int) (*domain.StudioKnowledgeEntry, error) {
	var entry domain.StudioKnowledgeEntry
	if err := r.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeEntryNotFound
		}
		return nil, err
	}
	return &entry, nil
}

func (r *gormStudioKnowledgeRepo) ListEntries(ctx context.Context, filter StudioKnowledgeListFilter, page int, pageSize int) ([]*domain.StudioKnowledgeEntry, int64, error) {
	var entries []*domain.StudioKnowledgeEntry
	var total int64

	// Count 与 Find 必须带同一组条件，否则 total 是全表数、分页却是过滤结果。
	applyFilter := func(q *gorm.DB) *gorm.DB {
		if keyword := strings.TrimSpace(filter.Query); keyword != "" {
			pattern := "%" + escapeLike(keyword) + "%"
			q = q.Where("title ILIKE ? OR body ILIKE ?", pattern, pattern)
		}
		if permission := strings.TrimSpace(filter.Permission); permission != "" {
			q = q.Where("permission = ?", permission)
		}
		if filter.ParentID != nil {
			if *filter.ParentID == nil {
				q = q.Where("parent_id IS NULL")
			} else {
				q = q.Where("parent_id = ?", **filter.ParentID)
			}
		}
		return q
	}

	if err := applyFilter(r.db.WithContext(ctx).Model(&domain.StudioKnowledgeEntry{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := applyFilter(r.db.WithContext(ctx)).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&entries).Error
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// UpdateEntry 保存正文并升版。
//
// 条件更新（WHERE id = ? AND version = expectedVersion）是并发闸门：
// 两个人同时打开同一版编辑时，先保存的把版本推到 2，后保存的
// 条件不再命中（RowsAffected = 0）→ 返回冲突，让调用方刷新后重试。
// 没有这道闸门，两人都会写出 version=2，而 (entry_id, version)
// 唯一索引只会让第二个人撞 23505——那个错误对用户毫无意义。
func (r *gormStudioKnowledgeRepo) UpdateEntry(ctx context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision, expectedVersion int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entry.ParentID != nil {
			var count int64
			if err := tx.Model(&domain.StudioKnowledgeEntry{}).
				Where("id = ?", *entry.ParentID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrKnowledgeParentNotFound
			}
		}

		nextVersion := expectedVersion + 1
		result := tx.Model(&domain.StudioKnowledgeEntry{}).
			Where("id = ? AND version = ?", entry.ID, expectedVersion).
			Updates(map[string]any{
				"title":      entry.Title,
				"body":       entry.Body,
				"permission": entry.Permission,
				"is_public":  entry.IsPublic,
				"parent_id":  entry.ParentID,
				"version":    nextVersion,
				"updated_at": time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 0 行只有两种可能：条目不存在，或版本已被别人推进。
			var count int64
			if err := tx.Model(&domain.StudioKnowledgeEntry{}).
				Where("id = ?", entry.ID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrKnowledgeEntryNotFound
			}
			return ErrKnowledgeVersionConflict
		}

		entry.Version = nextVersion
		revision.EntryID = entry.ID
		revision.Version = nextVersion
		if err := tx.Create(revision).Error; err != nil {
			// 撞 (entry_id, version) 唯一索引：并发窗口下另一个人刚插了同一版。
			// 翻译成冲突，而不是把驱动的 23505 抛给调用方。
			if IsDuplicateKey(err) {
				return ErrKnowledgeVersionConflict
			}
			return err
		}
		return nil
	})
}

// DeleteEntry 删除条目。
//
// 修订由 studio_knowledge_revisions 的 ON DELETE CASCADE 一并清理；
// 子条目的 parent_id 由 ON DELETE SET NULL 置空（不连带删子树）。
func (r *gormStudioKnowledgeRepo) DeleteEntry(ctx context.Context, id int) error {
	result := r.db.WithContext(ctx).Delete(&domain.StudioKnowledgeEntry{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrKnowledgeEntryNotFound
	}
	return nil
}

func (r *gormStudioKnowledgeRepo) ListRevisions(ctx context.Context, entryID int) ([]*domain.StudioKnowledgeRevision, error) {
	var revisions []*domain.StudioKnowledgeRevision
	err := r.db.WithContext(ctx).
		Where("entry_id = ?", entryID).
		Order("version DESC").
		Find(&revisions).Error
	if err != nil {
		return nil, err
	}
	return revisions, nil
}

func (r *gormStudioKnowledgeRepo) GetRevision(ctx context.Context, entryID int, version int) (*domain.StudioKnowledgeRevision, error) {
	var revision domain.StudioKnowledgeRevision
	err := r.db.WithContext(ctx).
		Where("entry_id = ? AND version = ?", entryID, version).
		First(&revision).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeRevisionNotFound
		}
		return nil, err
	}
	return &revision, nil
}

// IsDescendant 用递归 CTE 判断 candidate 是否在 ancestor 的子树里。
//
// 为什么在数据库里算而不是服务层拉全表：环检测必须看到**完整**的子树，
// 而子树规模无上界（知识库可以很深）。一条递归 CTE 把这件事交给 PG，
// 服务层只拿一个布尔值，内存与往返都只付一次。
//
// 深度上限是**必需的**防御：数据里若已经存在环（历史脏数据、或迁移前的
// 手工改库），没有上限的递归会一直跑下去，把请求挂死在一个无法取消的查询上。
// 到顶就停并按「是子孙」返回——宁可拒绝一次合法设置，也不让请求挂死。
func (r *gormStudioKnowledgeRepo) IsDescendant(ctx context.Context, ancestorID int, candidateID int) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE subtree AS (
			SELECT id, 0 AS depth FROM studio_knowledge_entries WHERE id = ?
			UNION ALL
			SELECT e.id, s.depth + 1 FROM studio_knowledge_entries e
			JOIN subtree s ON e.parent_id = s.id
			WHERE s.depth < ?
		)
		SELECT EXISTS(SELECT 1 FROM subtree WHERE id = ?)
	`, ancestorID, knowledgeMaxTreeDepth, candidateID).Scan(&found).Error
	if err != nil {
		return false, err
	}
	return found, nil
}

// knowledgeMaxTreeDepth 父子树递归的深度上限（防御环导致的无限递归）。
const knowledgeMaxTreeDepth = 100
