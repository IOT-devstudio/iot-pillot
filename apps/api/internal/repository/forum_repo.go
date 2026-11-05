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

// ErrForumPostNotFound 帖子不存在。
var ErrForumPostNotFound = errors.New("帖子不存在")

/* ------------------------------------------------------------------ *
 * 列表筛选
 * ------------------------------------------------------------------ */

// ForumPostListFilter 帖子列表的筛选条件，零值表示不额外限制。
//
// 方向筛选是**两态**而非三态：
//   - DirectionCode == nil  → 「全部」：不加方向条件，空方向帖子也出现；
//   - DirectionCode != nil  → 只看该具体方向（direction_code = ? 自然排除 NULL）。
//
// 这与 ADR 0003 决策 7 一致：空方向是合法状态，只在「全部」下可见。
type ForumPostListFilter struct {
	// Query 关键词：ILIKE 扫标题 + 正文（不搜回复，见 ADR 0003 决策 7）。
	Query string
	// DirectionCode 具体方向码；nil = 不筛选。
	DirectionCode *string
}

/* ------------------------------------------------------------------ *
 * 列表摘要（聚合视图）
 * ------------------------------------------------------------------ */

// ForumPostSummary 列表项所需的聚合视图（ADR 0003：列表不渲染正文，只显示摘要与元信息）。
//
// 一次查询带出：作者名（LEFT JOIN users）、回复数（子查询）、是否含图（正文 LIKE）、
// 最后活跃时间（帖子创建或最近一条未删回复，取较大者）。详情页另走 GetPost。
type ForumPostSummary struct {
	ID            int        `json:"id" gorm:"column:id"`
	Scope         string     `json:"scope" gorm:"column:scope"`
	Title         string     `json:"title" gorm:"column:title"`
	AuthorID      int        `json:"author_id" gorm:"column:author_id"`
	DirectionCode *string    `json:"direction_code" gorm:"column:direction_code"`
	Body          string     `json:"body" gorm:"column:body"`
	CreatedAt     time.Time  `json:"created_at" gorm:"column:created_at"`
	AuthorName    string     `json:"author_name" gorm:"column:author_name"`
	ReplyCount    int64      `json:"reply_count" gorm:"column:reply_count"`
	HasImage      bool       `json:"has_image" gorm:"column:has_image"`
	LastActiveAt  time.Time  `json:"last_active_at" gorm:"column:last_active_at"`
	// IsResolved 派生自 ResolvedAt：列表用的布尔态，避免把指针透传到前端。
	IsResolved bool `json:"is_resolved" gorm:"column:is_resolved"`
}

// TableName 聚合视图没有真实表，GORM 需要占位；实际查询在 ListPosts 里用 Model(ForumPost) 指定。
func (ForumPostSummary) TableName() string { return "forum_posts" }

/* ------------------------------------------------------------------ *
 * 仓储接口
 * ------------------------------------------------------------------ */

// ForumRepo 帖子与回复的持久化（#219 只读）。
//
// 写入（新建帖/回复、软删、标记已解决）属于 #220/#221，本接口只暴露读路径。
// 可见性（scope 越权、软删）的判定放在 service，不在 repo——repo 只按 scope
// 与 deleted_at 过滤出「该 scope 下未软删」的行，跨域越权与角色由上层把关。
type ForumRepo interface {
	// ListPosts 分页列出某 scope 下未软删的帖子，支持关键词与方向筛选，按时间倒序。
	ListPosts(ctx context.Context, scope domain.ForumScope, filter ForumPostListFilter, page int, pageSize int) ([]*ForumPostSummary, int64, error)
	// GetPost 按 id 取帖子原始行（含软删判断由 service 决定）；不存在返回 ErrForumPostNotFound。
	GetPost(ctx context.Context, id int) (*domain.ForumPost, error)
	// ListReplies 取某帖未软删的回复，按时间正序（官方回答置顶由 service 重排）。
	ListReplies(ctx context.Context, postID int) ([]*domain.ForumReply, error)
}

var _ ForumRepo = (*gormForumRepo)(nil)

type gormForumRepo struct {
	db *gorm.DB
}

// NewForumRepo 构造函数。
func NewForumRepo(db *gorm.DB) ForumRepo {
	return &gormForumRepo{db: db}
}

/* ------------------------------------------------------------------ *
 * 实现
 * ------------------------------------------------------------------ */

func (r *gormForumRepo) ListPosts(ctx context.Context, scope domain.ForumScope, filter ForumPostListFilter, page int, pageSize int) ([]*ForumPostSummary, int64, error) {
	var summaries []*ForumPostSummary
	var total int64

	// Count 与 Find 必须带同一组条件，否则 total 与分页结果对不上。
	applyFilter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("forum_posts.scope = ? AND forum_posts.deleted_at IS NULL", scope)
		if kw := strings.TrimSpace(filter.Query); kw != "" {
			// 关键词只扫主帖标题与正文；% / _ 转义防止用户输入被当成通配符（ADR 0003 决策 7）。
			pattern := "%" + escapeLike(kw) + "%"
			q = q.Where("forum_posts.title ILIKE ? OR forum_posts.body ILIKE ?", pattern, pattern)
		}
		if filter.DirectionCode != nil {
			q = q.Where("forum_posts.direction_code = ?", *filter.DirectionCode)
		}
		return q
	}

	base := r.db.WithContext(ctx).Model(&domain.ForumPost{}).
		Joins("LEFT JOIN users u ON u.id = forum_posts.author_id")

	if err := applyFilter(base.Session(&gorm.Session{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := applyFilter(base.Session(&gorm.Session{})).
		Select(`
			forum_posts.*,
			COALESCE(u.username, '') AS author_name,
			(SELECT count(*) FROM forum_replies r WHERE r.post_id = forum_posts.id AND r.deleted_at IS NULL) AS reply_count,
			-- 含图判定：正文里出现 Markdown 图片语法 ![任意](任意) 即视为有图。
			-- 之前只匹配空 alt 的 ![]()，漏掉常规 ![alt](url)。
			(forum_posts.body LIKE '%!\[%\]\(%') AS has_image,
			COALESCE(
				(SELECT max(r.created_at) FROM forum_replies r WHERE r.post_id = forum_posts.id AND r.deleted_at IS NULL),
				forum_posts.created_at
			) AS last_active_at,
			(forum_posts.resolved_at IS NOT NULL) AS is_resolved
		`).
		Order("last_active_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&summaries).Error
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

func (r *gormForumRepo) GetPost(ctx context.Context, id int) (*domain.ForumPost, error) {
	var post domain.ForumPost
	if err := r.db.WithContext(ctx).First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrForumPostNotFound
		}
		return nil, err
	}
	return &post, nil
}

func (r *gormForumRepo) ListReplies(ctx context.Context, postID int) ([]*domain.ForumReply, error) {
	var replies []*domain.ForumReply
	err := r.db.WithContext(ctx).
		Where("post_id = ? AND deleted_at IS NULL", postID).
		Order("created_at ASC").
		Find(&replies).Error
	if err != nil {
		return nil, err
	}
	return replies, nil
}
