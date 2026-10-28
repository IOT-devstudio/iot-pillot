package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

/* ------------------------------------------------------------------ *
 * 权限枚举
 * ------------------------------------------------------------------ */

// StudioKnowledgePermission 知识库条目的权限分级（ADR 0001 §3）。
//
// 首期**路由只挂 admin**：这个字段现在只是写进去、读回来，
// 不参与任何鉴权判定。member / public 的读取入口留到后续决策再开。
//
// 它的意义在于「先把归属定下来」：等真要开 member 读时，
// 不必再做一次数据迁移去猜每条笔记该给谁看。
type StudioKnowledgePermission string

const (
	KnowledgePermissionAdmin  StudioKnowledgePermission = "admin"
	KnowledgePermissionMember StudioKnowledgePermission = "member"
	KnowledgePermissionPublic StudioKnowledgePermission = "public"
)

// Valid 报告权限取值是否在支持范围内。
//
// 与迁移 0015 的 chk_studio_knowledge_permission 一一对应：
// 两边任何一处新增取值都必须同时改，否则合法值会在写库时被 CHECK 拒绝。
func (p StudioKnowledgePermission) Valid() bool {
	switch p {
	case KnowledgePermissionAdmin, KnowledgePermissionMember, KnowledgePermissionPublic:
		return true
	}
	return false
}

// StudioKnowledgePermissions 全部可用权限（校验错误提示里列举用）。
func StudioKnowledgePermissions() []string {
	return []string{
		string(KnowledgePermissionAdmin),
		string(KnowledgePermissionMember),
		string(KnowledgePermissionPublic),
	}
}

/* ------------------------------------------------------------------ *
 * 实体
 * ------------------------------------------------------------------ */

// StudioKnowledgeEntry 知识库条目（迁移 0015）：存**当前**正文。
//
// 每次保存都会递增 Version 并把旧内容写进 studio_knowledge_revisions，
// 因此历史永远可追；恢复 = 基于旧版再保存一版，**没有**一键回滚
// （ADR 0001 §3 明确拒绝自动回滚）。
//
// IsPublic 是迁移里遗留的布尔列，**由 Permission 派生**（见
// StudioKnowledgeFields.ToEntry）：两处各写一份必然会漂移，
// 而「permission=admin 但 is_public=true」这种行会让后续读取判定失去唯一答案。
type StudioKnowledgeEntry struct {
	ID    int    `json:"id" gorm:"primaryKey"`
	Title string `json:"title" gorm:"column:title"`
	Body  string `json:"body" gorm:"column:body"`
	// AuthorID 创建人（users.id，迁移里刻意不建 FK：用户软删后历史作者 id 仍要保留）。
	AuthorID   int                       `json:"author_id" gorm:"column:author_id"`
	IsPublic   bool                      `json:"is_public" gorm:"column:is_public"`
	Permission StudioKnowledgePermission `json:"permission" gorm:"column:permission"`
	// ParentID 可空的树形归属；删除父条目时子条目的 parent_id 置 NULL
	//（迁移 0015 的 ON DELETE SET NULL），不会连带删掉子树。
	ParentID *int `json:"parent_id" gorm:"column:parent_id"`
	// Version 当前版本号，从 1 起；每次成功保存 +1。
	Version   int       `json:"version" gorm:"column:version"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 显式表名（表名归迁移所有，不靠 GORM 复数推导）。
func (StudioKnowledgeEntry) TableName() string { return "studio_knowledge_entries" }

// StudioKnowledgeRevision 一条修订快照（迁移 0015）。
//
// (entry_id, version) 唯一：同一版本不可能有两份快照，
// 这是「版本号就是历史索引」这一设计的强制前提。
type StudioKnowledgeRevision struct {
	ID       int    `json:"id" gorm:"primaryKey"`
	EntryID  int    `json:"entry_id" gorm:"column:entry_id"`
	Version  int    `json:"version" gorm:"column:version"`
	Title    string `json:"title" gorm:"column:title"`
	Body     string `json:"body" gorm:"column:body"`
	EditorID int    `json:"editor_id" gorm:"column:editor_id"`
	// CreatedAt 该版本产生的时刻；列表按 version DESC 展示就是时间倒序。
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// TableName 显式表名。
func (StudioKnowledgeRevision) TableName() string { return "studio_knowledge_revisions" }

/* ------------------------------------------------------------------ *
 * 字段口径
 * ------------------------------------------------------------------ */

const (
	// StudioKnowledgeTitleMaxRunes 标题上限（与表单标题的 200 对齐）。
	StudioKnowledgeTitleMaxRunes = 200
	// StudioKnowledgeBodyMaxRunes 正文上限。
	//
	// 10 万字符足够放一篇长文档，又给单次请求设了明确的滥用上界：
	// 没有上限时一次 POST 就能把整库撑爆（正文是 text，不截断）。
	StudioKnowledgeBodyMaxRunes = 100_000
)

// ErrStudioKnowledgeInvalid 知识库条目字段不合法（消息里带具体原因）。
//
// 用哨兵而不是裸 errors.New：handler 必须能用 errors.Is 认出「这是校验失败」
// 并映射成 400，而不是掉进 500 的兜底分支。
var ErrStudioKnowledgeInvalid = errors.New("知识库条目不合法")

// StudioKnowledgeFields 一次创建 / 更新的字段快照（校验的输入）。
//
// 单独抽出来而不是直接校验实体：服务层要先校验、再决定写哪些列，
// 校验函数因此必须是纯的（不依赖 DB、不读时钟）。
type StudioKnowledgeFields struct {
	Title      string
	Body       string
	Permission StudioKnowledgePermission
	ParentID   *int
}

// Normalize 归一化输入。
//
// 标题去首尾空白（「 架构笔记 」与「架构笔记」必须算同一个标题）；
// 正文**不去空白**：Markdown 的缩进与空行是内容的一部分。
func (f StudioKnowledgeFields) Normalize() StudioKnowledgeFields {
	f.Title = strings.TrimSpace(f.Title)
	// 权限留空按 admin 处理：迁移 0015 的列默认值就是 'admin'，
	// 服务层跟着同一口径，避免「不传 permission」和「传 admin」写出不同的行。
	if f.Permission == "" {
		f.Permission = KnowledgePermissionAdmin
	}
	return f
}

// Validate 校验字段口径，返回带可读原因的包装错误。
func (f StudioKnowledgeFields) Validate() error {
	if f.Title == "" {
		return fmt.Errorf("%w: 标题必填", ErrStudioKnowledgeInvalid)
	}
	if utf8.RuneCountInString(f.Title) > StudioKnowledgeTitleMaxRunes {
		return fmt.Errorf("%w: 标题最多 %d 个字符", ErrStudioKnowledgeInvalid, StudioKnowledgeTitleMaxRunes)
	}
	if utf8.RuneCountInString(f.Body) > StudioKnowledgeBodyMaxRunes {
		return fmt.Errorf("%w: 正文最多 %d 个字符", ErrStudioKnowledgeInvalid, StudioKnowledgeBodyMaxRunes)
	}
	if !f.Permission.Valid() {
		return fmt.Errorf("%w: 权限只能是 %s", ErrStudioKnowledgeInvalid, strings.Join(StudioKnowledgePermissions(), " / "))
	}
	if f.ParentID != nil && *f.ParentID <= 0 {
		return fmt.Errorf("%w: parent_id 必须是正整数", ErrStudioKnowledgeInvalid)
	}
	return nil
}

// ApplyTo 把字段写进实体，并把 is_public 从 permission 派生。
//
// 单一事实来源：permission 是权威字段，is_public 只是迁移里的兼容列。
// 反过来（由 is_public 推 permission）不行——is_public 只有两态，
// 表达不了 admin / member / public 三档。
func (f StudioKnowledgeFields) ApplyTo(entry *StudioKnowledgeEntry) {
	entry.Title = f.Title
	entry.Body = f.Body
	entry.Permission = f.Permission
	entry.IsPublic = f.Permission == KnowledgePermissionPublic
	entry.ParentID = f.ParentID
}
