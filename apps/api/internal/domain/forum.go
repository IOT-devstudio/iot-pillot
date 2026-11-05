package domain

import (
	"time"
)

/* ------------------------------------------------------------------ *
 * 内容域（scope）
 * ------------------------------------------------------------------ */

// ForumScope 内容域：一套内容模型用 scope 分区（ADR 0003 决策 1）。
//
// scope 是「分区维度」不是「权限等级」：路由分组决定哪个 scope 要什么角色
// （recruitment 挂登录用户组、studio 挂 admin 组），service 再决定这条记录
// 归不归你。两者不可混为一谈。
type ForumScope string

const (
	// ForumScopeRecruitment 招新问答：登录用户可见、可回复。
	ForumScopeRecruitment ForumScope = "recruitment"
	// ForumScopeStudio 内部论坛：本期仅 admin 可见、可回复。
	ForumScopeStudio ForumScope = "studio"
)

/* ------------------------------------------------------------------ *
 * 实体
 * ------------------------------------------------------------------ */

// ForumPost 帖子（迁移 0018）。
//
// 列表/详情只依赖这些列；软删、已解决、编辑时间都是可选列（NULL 表示未触发）。
// 可空列统一用指针，让「NULL」与「零值」可区分——查询层用 `deleted_at IS NULL`
// 过滤，实体侧 nil 即未软删。
type ForumPost struct {
	ID            int        `json:"id" gorm:"primaryKey"`
	Scope         ForumScope `json:"scope" gorm:"column:scope"`
	Title         string     `json:"title" gorm:"column:title"`
	AuthorID      int        `json:"author_id" gorm:"column:author_id"`
	DirectionCode *string    `json:"direction_code" gorm:"column:direction_code"`
	Body          string     `json:"body" gorm:"column:body"`
	ResolvedAt    *time.Time `json:"resolved_at" gorm:"column:resolved_at"`
	ResolvedBy    *int       `json:"resolved_by" gorm:"column:resolved_by"`
	DeletedAt     *time.Time `json:"deleted_at" gorm:"column:deleted_at"`
	DeletedBy     *int       `json:"deleted_by" gorm:"column:deleted_by"`
	EditedAt      *time.Time `json:"edited_at" gorm:"column:edited_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// TableName 显式表名（表名归迁移所有，不靠 GORM 复数推导）。
func (ForumPost) TableName() string { return "forum_posts" }

// ForumReply 回复（迁移 0018）。
//
// 经 post_id 继承 scope，因此没有 title / scope / direction_code / resolved_* 列。
// 官方回答不落库：判定走 service 层的 AdminDirectory.IsAdmin（ADR 0003 决策 8）。
type ForumReply struct {
	ID        int        `json:"id" gorm:"primaryKey"`
	PostID    int        `json:"post_id" gorm:"column:post_id"`
	AuthorID  int        `json:"author_id" gorm:"column:author_id"`
	Body      string     `json:"body" gorm:"column:body"`
	DeletedAt *time.Time `json:"deleted_at" gorm:"column:deleted_at"`
	DeletedBy *int       `json:"deleted_by" gorm:"column:deleted_by"`
	EditedAt  *time.Time `json:"edited_at" gorm:"column:edited_at"`
	CreatedAt time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// TableName 显式表名。
func (ForumReply) TableName() string { return "forum_replies" }

/* ------------------------------------------------------------------ *
 * 长度上限（迁移 0018 的 CHECK 镜像，供写入票 #220 复用，保持单一事实）
 * ------------------------------------------------------------------ */

const (
	// ForumPostTitleMaxRunes 标题上限。
	ForumPostTitleMaxRunes = 200
	// ForumPostBodyMaxRunes 帖正文上限（Markdown，含图片引用占位）。
	ForumPostBodyMaxRunes = 20000
	// ForumReplyBodyMaxRunes 回复正文上限。
	ForumReplyBodyMaxRunes = 20000
)
