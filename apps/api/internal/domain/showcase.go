package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// ShowcaseSubmission 一份作品提交（学员交作业）。
//
// 载体三种，至少一个：仓库链接 / 源码 zip / 两者都给。
// 不建外键指向 learning_roadmaps：路线是内容、作品是学员已交付的事实，
// 路线被软删不该让作品跟着悬空（与 ADR 0006 决策 2 的方向软引用同理）。
type ShowcaseSubmission struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	UserID     int64  `json:"user_id" gorm:"column:user_id"`
	RoadmapID  *int64 `json:"roadmap_id" gorm:"column:roadmap_id"`
	// RoadmapTitle 提交时路线标题的快照：路线改名或删除后仍能看出当时交的是哪条。
	RoadmapTitle string `json:"roadmap_title" gorm:"column:roadmap_title"`

	Title   string `json:"title" gorm:"column:title"`
	Summary string `json:"summary" gorm:"column:summary"`

	RepoURL           string `json:"repo_url" gorm:"column:repo_url"`
	ArchiveObjectKey  string `json:"-" gorm:"column:archive_object_key"`
	ArchiveName       string `json:"archive_name" gorm:"column:archive_name"`
	ArchiveSize       int64  `json:"archive_size" gorm:"column:archive_size"`

	Status     string     `json:"status" gorm:"column:status"`
	ReviewNote string     `json:"review_note" gorm:"column:review_note"`
	ReviewedAt *time.Time `json:"reviewed_at" gorm:"column:reviewed_at"`
	ReviewedBy *int64     `json:"-" gorm:"column:reviewed_by"`

	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName 显式表名（GORM 会猜成 showcase_submissions，恰好一致，仍写出来防改名）。
func (ShowcaseSubmission) TableName() string { return "showcase_submissions" }

// 作品状态。
const (
	// ShowcaseStatusSubmitted 已提交、待审阅。
	ShowcaseStatusSubmitted = "submitted"
	// ShowcaseStatusApproved 已通过。
	ShowcaseStatusApproved = "approved"
	// ShowcaseStatusRejected 已驳回（必须带审阅意见，让学员知道改什么）。
	ShowcaseStatusRejected = "rejected"
)

// 作品提交的边界。
const (
	// ShowcaseTitleMaxRunes 标题上限。
	ShowcaseTitleMaxRunes = 100
	// ShowcaseSummaryMaxRunes 说明上限。
	ShowcaseSummaryMaxRunes = 2000
	// ShowcaseReviewNoteMaxRunes 审阅意见上限。
	ShowcaseReviewNoteMaxRunes = 1000
	// ShowcaseArchiveMaxBytes 源码 zip 大小上限（50 MiB）。
	//
	// 比图片（10MiB）大：源码包本身就不小。但也不能无限大——
	// nginx 的 client_max_body_size 与容器内存都要留余量。
	ShowcaseArchiveMaxBytes = 50 << 20
)

// ShowcaseStatusKnown 判断状态值是否合法。
func ShowcaseStatusKnown(status string) bool {
	switch status {
	case ShowcaseStatusSubmitted, ShowcaseStatusApproved, ShowcaseStatusRejected:
		return true
	}
	return false
}

// 作品提交的错误。
var (
	// ErrShowcaseInvalid 内容不合法（具体原因由 ShowcaseValidationError 携带）。
	ErrShowcaseInvalid = errors.New("作品提交内容不合法")
	// ErrShowcaseNotFound 作品不存在或不属于当前用户。
	ErrShowcaseNotFound = errors.New("作品提交不存在")
	// ErrShowcaseArchiveUnsupportedType 上传的不是 zip。
	ErrShowcaseArchiveUnsupportedType = errors.New("源码包只支持 .zip")
	// ErrShowcaseArchiveTooLarge 源码包超过大小上限。
	ErrShowcaseArchiveTooLarge = errors.New("源码包不能超过 50 MB")
)

// ShowcaseValidationError 带可读原因的内容错误。
//
// 单独一个类型而不是复用 ErrShowcaseInvalid：handler 要把具体原因回给用户
// （「请填写 GitHub 链接或上传源码包」比「内容不合法」有用得多）。
type ShowcaseValidationError struct {
	Reason string
}

func (e *ShowcaseValidationError) Error() string { return e.Reason }

// Is 让 errors.Is(err, ErrShowcaseInvalid) 能匹配任意具体原因。
func (e *ShowcaseValidationError) Is(target error) bool { return target == ErrShowcaseInvalid }

// ValidateShowcaseInput 校验一次作品提交的内容。
//
// 这里只做与「能否落库」相关的判定（长度、必填、载体至少一个）；
// 仓库链接的协议白名单单独用 IsAllowedShowcaseRepoURL，因为它同时被
// 展示层复用（渲染成可点击链接前要再过一次）。
func ValidateShowcaseInput(title, summary, repoURL, archiveKey string) error {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return &ShowcaseValidationError{Reason: "请填写作品标题"}
	}
	if utf8.RuneCountInString(trimmedTitle) > ShowcaseTitleMaxRunes {
		return &ShowcaseValidationError{Reason: "标题最多 100 个字"}
	}
	if utf8.RuneCountInString(summary) > ShowcaseSummaryMaxRunes {
		return &ShowcaseValidationError{Reason: "作品说明最多 2000 个字"}
	}

	hasRepo := strings.TrimSpace(repoURL) != ""
	hasArchive := strings.TrimSpace(archiveKey) != ""
	if !hasRepo && !hasArchive {
		return &ShowcaseValidationError{Reason: "请至少提供一项：GitHub 链接或源码压缩包"}
	}
	if hasRepo && !IsAllowedShowcaseRepoURL(repoURL) {
		return &ShowcaseValidationError{Reason: "仓库链接必须是 http/https 地址，例如 https://github.com/你的账号/仓库"}
	}
	return nil
}

// IsAllowedShowcaseRepoURL 只放行 http/https。
//
// 与资源链接同一口径（见 learning-roadmap/model.ts 的白名单）：
// 挡掉 javascript: / data: 这类在渲染成 <a href> 后可能执行脚本的协议。
func IsAllowedShowcaseRepoURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

// IsZipArchive 按文件名与 MIME 判断是不是 zip。
//
// 只看扩展名不够（改名就能绕），所以 MIME 也一起看；
// 两者任一命中即放行——部分浏览器给出的 zip MIME 并不稳定
// （application/zip / application/x-zip-compressed / application/octet-stream）。
func IsZipArchive(fileName, contentType string) bool {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(fileName)), ".zip") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/zip", "application/x-zip-compressed", "application/x-zip":
		return true
	}
	return false
}
