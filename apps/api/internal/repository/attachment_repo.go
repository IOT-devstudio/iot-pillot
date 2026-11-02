package repository

import (
	"context"
	"errors"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

	"gorm.io/gorm"
)

// ErrAttachmentNotFound 附件不存在（#217 的读取端点用它区分 404 与 403）。
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
