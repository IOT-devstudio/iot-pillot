package repository

import (
	"context"
	"errors"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

	"gorm.io/gorm"
)

// ErrAttachmentNotFound 附件不存在（#217 的读取端点用它区分 404 与 500）。
var ErrAttachmentNotFound = errors.New("附件不存在")

// AttachmentRepo 内容附件的持久化端口。
//
// #216 只用 CreateOrphan；Get 是为 #217 的读取端点预留的，读路径与
// 归属判定都放在 service 层，仓储只负责按 id 取行。
type AttachmentRepo interface {
	// CreateOrphan 插入一条尚未绑定任何内容的孤儿附件行。
	CreateOrphan(ctx context.Context, att *domain.Attachment) error
	// Get 按 id 取附件；不存在返回 ErrAttachmentNotFound。
	Get(ctx context.Context, id int) (*domain.Attachment, error)

	// ── #217 ──

	// BindForBody 按正文引用重算某内容行的附件绑定。
	//
	// 在一笔事务里完成「校验 + 绑定 + 解绑」：ids 是正文里出现的附件 id 集合（去重），
	// 不在 ids 里、但此前绑在该内容行上的附件会被解绑回孤儿态。
	// 校验失败（不存在 / 非本人上传 / 已被别的内容占用）整次回滚并返回错误，
	// 不静默忽略（见 ADR 0004 §3）。
	//
	// col 来自固定的 scope→列 映射（post_id / reply_id / roadmap_id），非用户输入，安全。
	BindForBody(ctx context.Context, col string, ownerID int64, authorID int, ids []int) error

	// ListOrphans 列出「三列全空且 created_at 早于 olderThan」的孤儿附件，最多 limit 条。
	//
	// 孤儿清理任务用：单批 500、created_at < now()-24h（见 ADR 0004 §7）。
	ListOrphans(ctx context.Context, olderThan time.Time, limit int) ([]*domain.Attachment, error)

	// DeleteAttachment 删除一条附件行（孤儿清理时连同对象存储一起删）。
	DeleteAttachment(ctx context.Context, id int) error
}

type gormAttachmentRepo struct {
	db *gorm.DB
}

var _ AttachmentRepo = (*gormAttachmentRepo)(nil)

// NewAttachmentRepo 构造附件仓储。
func NewAttachmentRepo(db *gorm.DB) AttachmentRepo {
	return &gormAttachmentRepo{db: db}
}

// CreateOrphan 插入孤儿附件行。
//
// object_key 上有 UNIQUE 约束：随机键碰撞时由数据库拒绝（23505），
// 不做应用层重试——16 字节随机数碰撞概率可以忽略，重试只会掩盖真问题。
func (r *gormAttachmentRepo) CreateOrphan(ctx context.Context, att *domain.Attachment) error {
	return r.db.WithContext(ctx).Create(att).Error
}

// Get 按 id 取附件。
func (r *gormAttachmentRepo) Get(ctx context.Context, id int) (*domain.Attachment, error) {
	var att domain.Attachment
	if err := r.db.WithContext(ctx).First(&att, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAttachmentNotFound
		}
		return nil, err
	}
	return &att, nil
}

// BindForBody 见接口注释。
//
// 单事务内完成：先逐条校验（取行 → 比 UploaderID 与既存绑定），再统一
// 绑定新集合、解绑消失的旧绑定。校验失败直接回滚，不落半成品。
func (r *gormAttachmentRepo) BindForBody(ctx context.Context, col string, ownerID int64, authorID int, ids []int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			var att domain.Attachment
			if err := tx.First(&att, id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAttachmentNotFound
				}
				return err
			}
			// 「别人上传的」一律拒绝：附件只能由上传者本人绑定到自己的内容。
			if att.UploaderID != authorID {
				return domain.ErrAttachmentForbidden
			}
			// 已被别的内容占用（任一列非空且不是本列本 owner）→ 拒绝。
			if !attachmentOwnedBy(att, col, ownerID) && attachmentHasAnyBinding(att) {
				return domain.ErrAttachmentForbidden
			}
		}

		// 绑定新集合。
		for _, id := range ids {
			if err := tx.Model(&domain.Attachment{}).
				Where("id = ?", id).
				Update(col, ownerID).Error; err != nil {
				return err
			}
		}

		// 解绑消失的旧绑定：同列同 owner 但不在新集合里的，回归孤儿态。
		if len(ids) == 0 {
			return tx.Model(&domain.Attachment{}).
				Where(col+" = ?", ownerID).
				Update(col, nil).Error
		}
		return tx.Model(&domain.Attachment{}).
			Where(col+" = ? AND id NOT IN ?", ownerID, ids).
			Update(col, nil).Error
	})
}

// attachmentOwnedBy 判断附件是否已绑在 (col, ownerID) 上。
func attachmentOwnedBy(att domain.Attachment, col string, ownerID int64) bool {
	switch col {
	case "post_id":
		return att.PostID != nil && *att.PostID == ownerID
	case "reply_id":
		return att.ReplyID != nil && *att.ReplyID == ownerID
	case "roadmap_id":
		return att.RoadmapID != nil && *att.RoadmapID == ownerID
	}
	return false
}

// attachmentHasAnyBinding 三列是否有任一非空。
func attachmentHasAnyBinding(att domain.Attachment) bool {
	return att.PostID != nil || att.ReplyID != nil || att.RoadmapID != nil
}

// ListOrphans 见接口注释。
func (r *gormAttachmentRepo) ListOrphans(ctx context.Context, olderThan time.Time, limit int) ([]*domain.Attachment, error) {
	var rows []*domain.Attachment
	if err := r.db.WithContext(ctx).
		Where("post_id IS NULL AND reply_id IS NULL AND roadmap_id IS NULL AND created_at < ?", olderThan).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteAttachment 见接口注释。
func (r *gormAttachmentRepo) DeleteAttachment(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Delete(&domain.Attachment{}, id).Error
}
