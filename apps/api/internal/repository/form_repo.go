package repository

import (
	"context"
	"errors"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

/* ------------------------------------------------------------------ *
 * 哨兵错误
 * ------------------------------------------------------------------ */

// ErrFormNotFound 表单不存在（或分享令牌无效）。
var ErrFormNotFound = errors.New("表单不存在")

// ErrFormFieldNotFound 字段不存在，或字段不属于该表单。
var ErrFormFieldNotFound = errors.New("表单字段不存在")

// ErrSubmissionNotFound 提交记录不存在。
var ErrSubmissionNotFound = errors.New("提交记录不存在")

// ErrDuplicateSubmission 同一用户对同一表单重复提交。
//
// 防重规则（issue #92 拍板）由迁移 0009 的 (form_id, submitter_id) 唯一索引强制；
// 撞约束时翻译成本错误，让调用方拿到"你已经交过了"而不是驱动层的英文报错。
var ErrDuplicateSubmission = errors.New("你已提交过这份表单，不能重复提交")

// ErrDuplicateShareToken 分享令牌冲突（概率极低，服务层换一个再试）。
var ErrDuplicateShareToken = errors.New("分享令牌冲突，请重试")

// ErrFormAlreadyOpen 同轮同方向已有另一张开放表单（issue #137）。
//
// 由迁移 0010 的部分唯一索引 idx_forms_open_uniq 强制：同一 (round_id,
// direction_code) 至多一行 is_open = true。历史（关闭）表单照旧可以共存。
//
// 两条路径都会产出本错误：
//   - 服务层开放前的前置检查（消息里带上冲突表单的标题，更可读）；
//   - 并发窗口下撞索引，由仓储层在此翻译（消息就是本哨兵原文）。
//
// 调用方一律用 errors.Is 识别 —— 前置检查那条是包装错误。
var ErrFormAlreadyOpen = errors.New("该方向已有开放中的表单，请先关闭它再开放本表单")

/* ------------------------------------------------------------------ *
 * 表单仓储接口
 * ------------------------------------------------------------------ */

// FormRepo 表单模块的持久化：表单定义 / 字段 / 提交记录。
//
// 三者同属表单域，放一个接口下减少装配与测试桩开销，且它们的写操作
// 共享事务封闭语义（建表 = 表单 + 字段；改表 = 表单 + 字段增量）。
//
// 申请域的表不在这里：提交命中统一申请模型（#88 的 RecruitmentRepo），
// 表单模块只持有 application_id 这个外键值。
type FormRepo interface {
	// ── 表单定义 ──
	// CreateForm 在同一事务里写入表单与它的字段；分享令牌撞唯一约束时
	// 返回 ErrDuplicateShareToken（调用方可换令牌重试）。
	CreateForm(ctx context.Context, form *domain.Form, fields []*domain.FormField) error
	// GetFormByID 按 id 取表单；不存在返回 ErrFormNotFound。
	GetFormByID(ctx context.Context, id int) (*domain.Form, error)
	// GetFormByShareToken 按分享令牌取表单；不存在返回 ErrFormNotFound。
	GetFormByShareToken(ctx context.Context, token string) (*domain.Form, error)
	// ListForms 分页列出全部表单（新建在前）。
	ListForms(ctx context.Context, page int, pageSize int) ([]*domain.Form, int64, error)
	// ListOpenFormsByRound 列出某轮次下的全部开放表单，按 (direction_code, updated_at DESC, id DESC) 排序。
	//
	// 方向列表组装（#135）需要知道"每个方向该跳哪张表"。一次查回整轮、在内存里
	// 按方向取第一张，而不是按方向逐个查：方向是个位数，一次查询比 N 次往返干净。
	//
	// 排序即取表规则：同一方向的第一张就是该方向该用的那张。updated_at 相同时
	// （批量建表会落在同一秒）用 id DESC 兜底，保证结果稳定、不随查询计划漂移。
	// 无开放轮次或该轮无开放表单都返回空切片与 nil 错误 —— 调用方据此把 form 置 null。
	//
	// 开放前的前置检查（#137）也走这个方法：按 (round_id, direction_code) 逐方向
	// 取开放表单与它同源同序，不另开一个只差一个过滤条件的查询 —— 那会让"开放
	// 表单的读取"出现两个所有者，两处排序规则迟早漂移。
	ListOpenFormsByRound(ctx context.Context, roundID int) ([]*domain.Form, error)
	// UpdateForm 更新表单基本字段；fields 非 nil 时在同一事务里做字段增量
	// （带 id 的原地更新、无 id 的新增、未出现的删除）。fields 为 nil = 只改表单本身。
	UpdateForm(ctx context.Context, form *domain.Form, fields []*domain.FormField) error
	// SetFormOpen 开启 / 关闭表单提交；表单不存在返回 ErrFormNotFound。
	SetFormOpen(ctx context.Context, id int, open bool) error

	// ── 字段 ──
	// ListFields 按人工排序返回表单字段。
	ListFields(ctx context.Context, formID int) ([]*domain.FormField, error)
	// CountSubmissions 批量统计若干表单的提交数（管理端列表展示用，避免逐条查询）。
	CountSubmissions(ctx context.Context, formIDs []int) (map[int]int64, error)

	// ── 提交记录 ──
	// CreateSubmission 写入一条提交；撞 (form_id, submitter_id) 唯一约束时
	// 返回 ErrDuplicateSubmission。
	CreateSubmission(ctx context.Context, submission *domain.FormSubmission) error
	// GetSubmissionBySubmitter 取某用户在某表单上的提交；没有返回 ErrSubmissionNotFound。
	GetSubmissionBySubmitter(ctx context.Context, formID int, submitterID int) (*domain.FormSubmission, error)
	// ListSubmissions 分页列出某表单的提交（新提交在前）。
	ListSubmissions(ctx context.Context, formID int, page int, pageSize int) ([]*domain.FormSubmission, int64, error)
	// GetSubmission 取某表单下的一条提交；不匹配返回 ErrSubmissionNotFound。
	GetSubmission(ctx context.Context, formID int, submissionID int) (*domain.FormSubmission, error)
}

var _ FormRepo = (*gormFormRepo)(nil)

type gormFormRepo struct {
	db *gorm.DB
}

// NewFormRepo 构造函数。
func NewFormRepo(db *gorm.DB) FormRepo {
	return &gormFormRepo{db: db}
}

/* ------------------------------------------------------------------ *
 * 表单定义
 * ------------------------------------------------------------------ */

func (r *gormFormRepo) CreateForm(ctx context.Context, form *domain.Form, fields []*domain.FormField) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(form).Error; err != nil {
			return err
		}
		for i, f := range fields {
			f.ID = 0
			f.FormID = form.ID
			f.SortOrder = i
			if err := tx.Create(f).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// forms 上只有 share_token 一个唯一约束，撞上就只可能是令牌重复。
		if IsDuplicateKey(err) {
			return ErrDuplicateShareToken
		}
		return err
	}
	return nil
}

func (r *gormFormRepo) GetFormByID(ctx context.Context, id int) (*domain.Form, error) {
	var form domain.Form
	if err := r.db.WithContext(ctx).First(&form, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFormNotFound
		}
		return nil, err
	}
	return &form, nil
}

func (r *gormFormRepo) GetFormByShareToken(ctx context.Context, token string) (*domain.Form, error) {
	var form domain.Form
	if err := r.db.WithContext(ctx).Where("share_token = ?", token).First(&form).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFormNotFound
		}
		return nil, err
	}
	return &form, nil
}

func (r *gormFormRepo) ListForms(ctx context.Context, page int, pageSize int) ([]*domain.Form, int64, error) {
	var forms []*domain.Form
	var total int64

	if err := r.db.WithContext(ctx).Model(&domain.Form{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&forms).Error
	if err != nil {
		return nil, 0, err
	}
	return forms, total, nil
}

// ListOpenFormsByRound 列出某轮次下的开放表单，按取表规则排序。
//
// 取表规则（#135 §2.1 / D1）：同一轮同一方向可能有多张表单（迁移 0009 的
// idx_forms_round_direction 只是普通索引，没有唯一约束），用户端点「面试」只能跳
// 一张。规则是「is_open = true 且 updated_at 最新」；updated_at 打平时用 id DESC
// 兜底 —— 只按 updated_at 排序在批量建表（同一秒）时结果不稳定。
//
// 同轮同方向"只允许一张开放表单"的约束不在本 issue 范围（见 #137）。
func (r *gormFormRepo) ListOpenFormsByRound(ctx context.Context, roundID int) ([]*domain.Form, error) {
	var forms []*domain.Form
	err := r.db.WithContext(ctx).
		Where("round_id = ? AND is_open = ?", roundID, true).
		Order("direction_code ASC, updated_at DESC, id DESC").
		Find(&forms).Error
	if err != nil {
		return nil, err
	}
	return forms, nil
}

// UpdateForm 更新表单本身，并在 fields 非 nil 时做字段增量。
//
// 字段增量按 id 对齐：带 id 的原地更新（保留 id，历史答案仍对得上字段），
// 无 id 的新增，未在入参里出现的删除。整个过程一个事务 ——
// 改表到一半失败会留下"字段对不上"的表单，比整单失败更难排查。
//
// 唯一约束冲突在这里只可能是 idx_forms_open_uniq（#137）：本方法只写
// title / description / is_open / updated_at，不碰 share_token，所以
// forms 上的另一个唯一约束撞不上。翻译成 ErrFormAlreadyOpen，别让调用方
// 看到驱动层的英文报错。
func (r *gormFormRepo) UpdateForm(ctx context.Context, form *domain.Form, fields []*domain.FormField) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&domain.Form{}).
			Where("id = ?", form.ID).
			Updates(map[string]any{
				"title":       form.Title,
				"description": form.Description,
				"is_open":     form.IsOpen,
				"updated_at":  time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 行数 0 也可能是"内容完全没变"，用一次存在性查询区分，
			// 免得把无变化的保存报成 404。
			var count int64
			if err := tx.Model(&domain.Form{}).Where("id = ?", form.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrFormNotFound
			}
		}

		if fields == nil {
			return nil
		}

		var existing []*domain.FormField
		if err := tx.Where("form_id = ?", form.ID).Find(&existing).Error; err != nil {
			return err
		}
		owned := make(map[int]bool, len(existing))
		for _, e := range existing {
			owned[e.ID] = true
		}

		keep := make(map[int]bool, len(fields))
		for i, f := range fields {
			f.FormID = form.ID
			f.SortOrder = i

			if f.ID > 0 {
				if !owned[f.ID] {
					return ErrFormFieldNotFound
				}
				keep[f.ID] = true
				if err := tx.Model(&domain.FormField{}).
					Where("id = ? AND form_id = ?", f.ID, form.ID).
					Updates(map[string]any{
						"field_type":  f.Type,
						"label":       f.Label,
						"required":    f.Required,
						"placeholder": f.Placeholder,
						"options":     f.Options,
						"sort_order":  f.SortOrder,
						"updated_at":  time.Now(),
					}).Error; err != nil {
					return err
				}
				continue
			}

			insert := &domain.FormField{
				FormID:      form.ID,
				Type:        f.Type,
				Label:       f.Label,
				Required:    f.Required,
				Placeholder: f.Placeholder,
				Options:     f.Options,
				SortOrder:   f.SortOrder,
			}
			if err := tx.Create(insert).Error; err != nil {
				return err
			}
			f.ID = insert.ID
		}

		removed := make([]int, 0, len(existing))
		for _, e := range existing {
			if !keep[e.ID] {
				removed = append(removed, e.ID)
			}
		}
		if len(removed) > 0 {
			if err := tx.Where("form_id = ? AND id IN ?", form.ID, removed).
				Delete(&domain.FormField{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && IsDuplicateKey(err) {
		return ErrFormAlreadyOpen
	}
	return err
}

// SetFormOpen 开启 / 关闭表单提交；表单不存在返回 ErrFormNotFound。
//
// 开启时同样受 idx_forms_open_uniq 约束（#137）：同轮同方向已有开放表单
// 则撞索引，翻译成 ErrFormAlreadyOpen。关闭（open = false）永不冲突 ——
// 部分唯一索引只覆盖 is_open = true 的行。
func (r *gormFormRepo) SetFormOpen(ctx context.Context, id int, open bool) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Form{}).
		Where("id = ?", id).
		Update("is_open", open)
	if result.Error != nil {
		if IsDuplicateKey(result.Error) {
			return ErrFormAlreadyOpen
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 状态本来就是这个值也可能 0 行，用存在性查询区分
		var count int64
		if err := r.db.WithContext(ctx).Model(&domain.Form{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrFormNotFound
		}
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 字段
 * ------------------------------------------------------------------ */

func (r *gormFormRepo) ListFields(ctx context.Context, formID int) ([]*domain.FormField, error) {
	var fields []*domain.FormField
	err := r.db.WithContext(ctx).
		Where("form_id = ?", formID).
		Order("sort_order ASC, id ASC").
		Find(&fields).Error
	if err != nil {
		return nil, err
	}
	return fields, nil
}

type submissionCountRow struct {
	FormID int
	Total  int64
}

func (r *gormFormRepo) CountSubmissions(ctx context.Context, formIDs []int) (map[int]int64, error) {
	counts := make(map[int]int64, len(formIDs))
	if len(formIDs) == 0 {
		return counts, nil
	}

	var rows []submissionCountRow
	err := r.db.WithContext(ctx).
		Model(&domain.FormSubmission{}).
		Select("form_id, COUNT(*) AS total").
		Where("form_id IN ?", formIDs).
		Group("form_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.FormID] = row.Total
	}
	return counts, nil
}

/* ------------------------------------------------------------------ *
 * 提交记录
 * ------------------------------------------------------------------ */

func (r *gormFormRepo) CreateSubmission(ctx context.Context, submission *domain.FormSubmission) error {
	if err := r.db.WithContext(ctx).Create(submission).Error; err != nil {
		if IsDuplicateKey(err) {
			return ErrDuplicateSubmission
		}
		return err
	}
	return nil
}

func (r *gormFormRepo) GetSubmissionBySubmitter(ctx context.Context, formID int, submitterID int) (*domain.FormSubmission, error) {
	var submission domain.FormSubmission
	err := r.db.WithContext(ctx).
		Where("form_id = ? AND submitter_id = ?", formID, submitterID).
		First(&submission).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	return &submission, nil
}

func (r *gormFormRepo) ListSubmissions(ctx context.Context, formID int, page int, pageSize int) ([]*domain.FormSubmission, int64, error) {
	var submissions []*domain.FormSubmission
	var total int64

	base := r.db.WithContext(ctx).Model(&domain.FormSubmission{}).Where("form_id = ?", formID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Where("form_id = ?", formID).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&submissions).Error
	if err != nil {
		return nil, 0, err
	}
	return submissions, total, nil
}

func (r *gormFormRepo) GetSubmission(ctx context.Context, formID int, submissionID int) (*domain.FormSubmission, error) {
	var submission domain.FormSubmission
	err := r.db.WithContext(ctx).
		Where("id = ? AND form_id = ?", submissionID, formID).
		First(&submission).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	return &submission, nil
}
