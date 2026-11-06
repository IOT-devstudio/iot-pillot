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

// ErrForumPostNotFound 帖子不存在或已软删。
var ErrForumPostNotFound = errors.New("帖子不存在")

// ErrForumReplyNotFound 回复不存在或已软删。
//
// 与帖子分开是因为回复的可见性还要叠加「所属帖子是否可见」，
// service 需要能区分「回复本身没了」与「帖子没了」两种来源。
var ErrForumReplyNotFound = errors.New("回复不存在")

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
	// Resolved 已解决筛选（#221）：nil = 全部，true = 只看已解决，false = 只看未解决。
	//
	// 三态而不是两态：与方向不同，「已解决」的两种取值都是明确的用户意图，
	// 而零值（false）若表示「只看未解决」会让「不筛选」无法表达。
	Resolved *bool
}

/* ------------------------------------------------------------------ *
 * 列表摘要（聚合视图）
 * ------------------------------------------------------------------ */

// ForumPostSummary 列表项所需的聚合视图（ADR 0003：列表不渲染正文，只显示摘要与元信息）。
//
// 一次查询带出：作者名（LEFT JOIN users）、回复数（子查询）、是否含图（正文 LIKE）、
// 最后活跃时间（帖子创建或最近一条未删回复，取较大者）。详情页另走 GetPost。
//
// 作者名取 users.name（不是 users.username）：users 表没有 username 列，
// 「用户名」只是 JWT 声明与接口响应里的字段名，落库列名是 name（见 domain.User.Name）。
// 写错列名不会在编译期暴露，只会在 SQL 执行时 42703，最终变成 500 + 业务码 5000。
//
// 这个 JOIN 刻意**不过滤** u.deleted_at：注销账号的历史帖子仍要显示当时的作者名，
// 与迁移 0011「按 user_id 保留历史与审计」同口径。登录面（GetByName 等）的排除规则
// 不适用于内容展示——否则注销一个人，所有旧帖作者就集体变成「匿名」。
type ForumPostSummary struct {
	ID            int       `json:"id" gorm:"column:id"`
	Scope         string    `json:"scope" gorm:"column:scope"`
	Title         string    `json:"title" gorm:"column:title"`
	AuthorID      int       `json:"author_id" gorm:"column:author_id"`
	DirectionCode *string   `json:"direction_code" gorm:"column:direction_code"`
	Body          string    `json:"body" gorm:"column:body"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at"`
	AuthorName    string    `json:"author_name" gorm:"column:author_name"`
	ReplyCount    int64     `json:"reply_count" gorm:"column:reply_count"`
	HasImage      bool      `json:"has_image" gorm:"column:has_image"`
	LastActiveAt  time.Time `json:"last_active_at" gorm:"column:last_active_at"`
	// IsResolved 派生自 ResolvedAt：列表用的布尔态，避免把指针透传到前端。
	IsResolved bool `json:"is_resolved" gorm:"column:is_resolved"`
}

// TableName 聚合视图没有真实表，GORM 需要占位；实际查询在 ListPosts 里用 Model(ForumPost) 指定。
func (ForumPostSummary) TableName() string { return "forum_posts" }

/* ------------------------------------------------------------------ *
 * 仓储接口
 * ------------------------------------------------------------------ */

// ForumRepo 帖子与回复的持久化。
//
// #219 落地读路径，#220 补写入（新建帖/回复、编辑、软删）。
// 可见性（scope 越权、软删）的判定放在 service，不在 repo——repo 只按 scope
// 与 deleted_at 过滤出「该 scope 下未软删」的行，跨域越权与角色由上层把关。
type ForumRepo interface {
	// ListPosts 分页列出某 scope 下未软删的帖子，支持关键词与方向筛选，按时间倒序。
	ListPosts(ctx context.Context, scope domain.ForumScope, filter ForumPostListFilter, page int, pageSize int) ([]*ForumPostSummary, int64, error)
	// GetPost 按 id 取帖子原始行（含软删判断由 service 决定）；不存在返回 ErrForumPostNotFound。
	GetPost(ctx context.Context, id int) (*domain.ForumPost, error)
	// ListReplies 取某帖未软删的回复，按时间正序（官方回答置顶由 service 重排）。
	ListReplies(ctx context.Context, postID int) ([]*domain.ForumReply, error)

	// CreatePost 插入帖子，回填自增 id 与 created_at。
	CreatePost(ctx context.Context, post *domain.ForumPost) error
	// UpdatePost 只更新可编辑列（title / body / edited_at），不碰作者与 scope。
	// 条件更新 deleted_at IS NULL：软删后的行不再接受编辑，返回 ErrForumPostNotFound。
	UpdatePost(ctx context.Context, post *domain.ForumPost) error
	// SoftDeletePost 软删帖子（置 deleted_at/deleted_by）。幂等：已软删返回 ErrForumPostNotFound。
	SoftDeletePost(ctx context.Context, id int, byUserID int) error

	// CreateReply 插入回复，回填自增 id 与 created_at。
	CreateReply(ctx context.Context, reply *domain.ForumReply) error
	// GetReply 按 id 取回复原始行；不存在返回 ErrForumReplyNotFound。
	GetReply(ctx context.Context, id int) (*domain.ForumReply, error)
	// UpdateReply 只更新 body / edited_at；软删后的行不再接受编辑。
	UpdateReply(ctx context.Context, reply *domain.ForumReply) error
	// SoftDeleteReply 软删回复（置 deleted_at/deleted_by）。幂等：已软删返回 ErrForumReplyNotFound。
	SoftDeleteReply(ctx context.Context, id int, byUserID int) error

	// LookupPostScope 查帖子的内容域与软删状态（附件归属判定用，#221）。
	//
	// 与 GetPost 分开是因为附件判定只需要这两个字段，且**软删行也要能读到**：
	// 判定要区分「帖子不存在」与「帖子已软删」，两者虽然都回 404，
	// 但前者是数据异常、后者是正常业务，日志与测试口径不同。
	LookupPostScope(ctx context.Context, id int) (scope domain.ForumScope, deleted bool, err error)
	// LookupReplyPostScope 查回复所属帖子的内容域与可见性（附件归属判定用）。
	//
	// 回复经 post_id 继承 scope（ADR 0003），所以判定要穿透到帖子：
	// 回复本身软删、或所属帖子软删，都算不可见。返回的 deleted 是两者的并集。
	LookupReplyPostScope(ctx context.Context, replyID int) (scope domain.ForumScope, deleted bool, err error)

	// SetPostResolved 设置/取消已解决（#221）。
	//
	// resolved=true 写 resolved_at=now + resolved_by；false 时两列都置 NULL（撤销）。
	// 条件更新 deleted_at IS NULL：软删帖不再接受状态变更，返回 ErrForumPostNotFound。
	SetPostResolved(ctx context.Context, id int, byUserID int, resolved bool) error
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
		// 已解决筛选（#221）：nil = 全部，true/false 各自精确匹配。
		if filter.Resolved != nil {
			if *filter.Resolved {
				q = q.Where("forum_posts.resolved_at IS NOT NULL")
			} else {
				q = q.Where("forum_posts.resolved_at IS NULL")
			}
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
			COALESCE(u.name, '') AS author_name,
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

/* ------------------------------------------------------------------ *
 * 写入路径（#220）
 * ------------------------------------------------------------------ */

// CreatePost 插入帖子。GORM 由 primaryKey + autoCreateTime 回填 id 与 created_at，
// 调用方拿到 post 后即可直接用于响应。
func (r *gormForumRepo) CreatePost(ctx context.Context, post *domain.ForumPost) error {
	return r.db.WithContext(ctx).Create(post).Error
}

// UpdatePost 只更新可编辑列。
//
// 用 `deleted_at IS NULL` 做条件更新而不是先读后写：并发下「读到未删 → 另一请求删掉
// → 本请求写回」会让已删内容复活。条件更新把「是否还可见」的判断压进同一条 SQL，
// RowsAffected == 0 即代表行已消失（不存在或已软删），统一回 ErrForumPostNotFound。
func (r *gormForumRepo) UpdatePost(ctx context.Context, post *domain.ForumPost) error {
	res := r.db.WithContext(ctx).
		Model(&domain.ForumPost{}).
		Where("id = ? AND deleted_at IS NULL", post.ID).
		Updates(map[string]any{
			"title":     post.Title,
			"body":      post.Body,
			"edited_at": post.EditedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrForumPostNotFound
	}
	return nil
}

// SoftDeletePost 软删帖子。同样用条件更新保证幂等：重复删除不会覆盖首次的删除人与时间。
func (r *gormForumRepo) SoftDeletePost(ctx context.Context, id int, byUserID int) error {
	res := r.db.WithContext(ctx).
		Model(&domain.ForumPost{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"deleted_by": byUserID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrForumPostNotFound
	}
	return nil
}

// CreateReply 插入回复。
func (r *gormForumRepo) CreateReply(ctx context.Context, reply *domain.ForumReply) error {
	return r.db.WithContext(ctx).Create(reply).Error
}

// GetReply 按 id 取回复原始行。这里**不过滤 deleted_at**：
// service 需要区分「回复软删」与「回复不存在」，统一在 service 里折叠成 404。
func (r *gormForumRepo) GetReply(ctx context.Context, id int) (*domain.ForumReply, error) {
	var reply domain.ForumReply
	if err := r.db.WithContext(ctx).First(&reply, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrForumReplyNotFound
		}
		return nil, err
	}
	return &reply, nil
}

// UpdateReply 只更新 body / edited_at，条件同 UpdatePost。
func (r *gormForumRepo) UpdateReply(ctx context.Context, reply *domain.ForumReply) error {
	res := r.db.WithContext(ctx).
		Model(&domain.ForumReply{}).
		Where("id = ? AND deleted_at IS NULL", reply.ID).
		Updates(map[string]any{
			"body":      reply.Body,
			"edited_at": reply.EditedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrForumReplyNotFound
	}
	return nil
}

// SoftDeleteReply 软删回复。
func (r *gormForumRepo) SoftDeleteReply(ctx context.Context, id int, byUserID int) error {
	res := r.db.WithContext(ctx).
		Model(&domain.ForumReply{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"deleted_by": byUserID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrForumReplyNotFound
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 附件归属判定（#221）
 * ------------------------------------------------------------------ */

// LookupPostScope 查帖子的 scope 与是否软删。
//
// 用 Select 只取两列而不是整行：附件读取是热路径（每张图一次请求），
// 把 20KB 的正文也读出来没有意义。
func (r *gormForumRepo) LookupPostScope(ctx context.Context, id int) (domain.ForumScope, bool, error) {
	var row struct {
		Scope     string     `gorm:"column:scope"`
		DeletedAt *time.Time `gorm:"column:deleted_at"`
	}
	err := r.db.WithContext(ctx).
		Model(&domain.ForumPost{}).
		Select("scope", "deleted_at").
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, ErrForumPostNotFound
		}
		return "", false, err
	}
	return domain.ForumScope(row.Scope), row.DeletedAt != nil, nil
}

// LookupReplyPostScope 穿透回复查到所属帖子的 scope。
//
// 一次 JOIN 而不是两次查询：reply → post 的关联是必需的，分两步会多一次往返，
// 且中间状态可能不一致（回复读到了，帖子刚被删）。
func (r *gormForumRepo) LookupReplyPostScope(ctx context.Context, replyID int) (domain.ForumScope, bool, error) {
	var row struct {
		Scope          string     `gorm:"column:scope"`
		PostDeletedAt  *time.Time `gorm:"column:post_deleted_at"`
		ReplyDeletedAt *time.Time `gorm:"column:reply_deleted_at"`
	}
	err := r.db.WithContext(ctx).
		Table("forum_replies AS r").
		Select("p.scope AS scope, p.deleted_at AS post_deleted_at, r.deleted_at AS reply_deleted_at").
		Joins("JOIN forum_posts AS p ON p.id = r.post_id").
		Where("r.id = ?", replyID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, ErrForumReplyNotFound
		}
		return "", false, err
	}
	// 回复软删或帖子软删，对读者而言都是「看不到」，合并成一个 deleted。
	return domain.ForumScope(row.Scope), row.PostDeletedAt != nil || row.ReplyDeletedAt != nil, nil
}

// SetPostResolved 设置或取消已解决。
//
// 用一条 UPDATE 同时处理两种方向，而不是「先读再写」：
// 标记与取消是同一个开关的两次点击，读-改-写会在并发下互相覆盖。
func (r *gormForumRepo) SetPostResolved(ctx context.Context, id int, byUserID int, resolved bool) error {
	patch := map[string]any{
		"resolved_at": nil,
		"resolved_by": nil,
	}
	if resolved {
		patch["resolved_at"] = time.Now()
		patch["resolved_by"] = byUserID
	}
	res := r.db.WithContext(ctx).
		Model(&domain.ForumPost{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(patch)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrForumPostNotFound
	}
	return nil
}
