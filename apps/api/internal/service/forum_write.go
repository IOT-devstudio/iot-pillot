package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 写入侧的哨兵错误
 * ------------------------------------------------------------------ */

// ErrForumReplyNotFound 回复不存在/已软删，统一映射 404。
//
// ErrForumPostNotFound 已在 forum_usec.go 声明（两处共用同一个语义），
// 这里只补回复那一个，避免重复声明。
var ErrForumReplyNotFound = repository.ErrForumReplyNotFound

// ErrForumInvalidInput 入参不合法（长度超限、标题为空等），映射 400。
//
// 不拆成多个哨兵：handler 只需要「是不是用户的输入问题」这一个区分，
// 具体原因通过 message 透出（前端直接把 message 展示给用户）。
type ErrForumInvalidInput struct {
	Message string
}

func (e *ErrForumInvalidInput) Error() string { return e.Message }

func invalidInput(format string, args ...any) error {
	return &ErrForumInvalidInput{Message: fmt.Sprintf(format, args...)}
}

// IsForumInvalidInput 判断错误是否为「用户输入不合法」。
func IsForumInvalidInput(err error) bool {
	var e *ErrForumInvalidInput
	return errors.As(err, &e)
}

/* ------------------------------------------------------------------ *
 * 写入入参
 * ------------------------------------------------------------------ */

// CreateForumPostInput 新建帖子入参。
//
// DirectionCode 为 nil 表示「没选方向」，走服务端回落链；
// 显式选了（非 nil）则以用户选择为准，即使与资料方向不同。
type CreateForumPostInput struct {
	Title         string
	Body          string
	DirectionCode *string
}

// UpdateForumPostInput 编辑帖子入参。标题与正文都可改，方向不可改（改方向会让历史筛选错乱）。
type UpdateForumPostInput struct {
	Title string
	Body  string
}

// CreateForumReplyInput 新建回复入参。
type CreateForumReplyInput struct {
	Body string
}

/* ------------------------------------------------------------------ *
 * 方向回落
 * ------------------------------------------------------------------ */

// userDirectionReader 取用户资料方向的最小依赖面。
//
// 只声明用到的方法而不是直接依赖 repository.UserRepo 全接口：
// 桩测试不需要实现十几个无关方法，也让这层依赖一目了然。
type userDirectionReader interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

// applicationDirectionReader 取用户最新报名方向的最小依赖面。
type applicationDirectionReader interface {
	ListApplicationsByUser(ctx context.Context, userID int, page int, pageSize int) ([]*domain.Application, int64, error)
}

// resolveDirection 方向回落：手选 → 资料方向 → 最新报名第一条 → 留空。
//
// 这是 #226 定下的四级链路：必须先看手选（用户明确意愿最高），
// 再退到用户资料（自己填过），再退到最近一次报名（报名时选的最能代表当前意向），
// 最后留空（合法状态，只在「全部」筛选下可见，见 ADR 0003 决策 7）。
//
// 任何一步读失败都**不阻断发帖**：方向是筛选辅助信息，不是内容本身，
// 因为查不到方向就让用户发不出帖是本末倒置。失败时继续往下一级退。
func (uc *ForumUseCase) resolveDirection(ctx context.Context, userID int, chosen *string) *string {
	// 1) 手选优先
	if chosen != nil {
		if code := strings.TrimSpace(*chosen); code != "" {
			return &code
		}
		// 传了空串视为「没选」，继续回落
	}
	// 2) 资料方向（User.Detail 是 embedded 结构，方向在 detail_direction 列）
	if uc.users != nil {
		if user, err := uc.users.GetByID(ctx, userID); err == nil && user != nil {
			if code := strings.TrimSpace(string(user.Detail.Direction)); code != "" {
				return &code
			}
		}
	}
	// 3) 最新一条报名记录的方向（列表按时间倒序，取第一页第一条）
	if uc.applications != nil {
		apps, _, err := uc.applications.ListApplicationsByUser(ctx, userID, 1, 1)
		if err == nil && len(apps) > 0 && apps[0] != nil {
			if code := strings.TrimSpace(apps[0].DirectionCode); code != "" {
				return &code
			}
		}
	}
	// 4) 留空
	return nil
}

/* ------------------------------------------------------------------ *
 * 归属校验
 * ------------------------------------------------------------------ */

// canManage 判断当前用户能否编辑/删除该条内容：作者本人或管理员。
//
// 管理员判定**现查 AdminDirectory**（ADR 0003 决策 8 同口径），不用 JWT 里的
// role：撤销管理员后历史令牌仍然有效，用令牌会让权限滞后到令牌过期。
// AdminDirectory 不可用时保守按「非管理员」处理——宁可让管理员重试，
// 也不能在名单读不到时放行任何人。
func (uc *ForumUseCase) canManage(ctx context.Context, authorID int, userID int) bool {
	if authorID == userID {
		return true
	}
	if uc.admins == nil {
		return false
	}
	isAdmin, err := uc.admins.IsAdmin(ctx, userID)
	if err != nil {
		return false
	}
	return isAdmin
}

/* ------------------------------------------------------------------ *
 * 长度校验
 * ------------------------------------------------------------------ */

// validatePostInput 校验标题与正文。计数用 rune 而不是 byte：
// 数据库 CHECK 是 char_length（按字符），用 len() 会让中文帖提前被拒。
func validatePostInput(title string, body string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return invalidInput("标题不能为空")
	}
	if n := len([]rune(title)); n > domain.ForumPostTitleMaxRunes {
		return invalidInput("标题最多 %d 个字符，当前 %d 个", domain.ForumPostTitleMaxRunes, n)
	}
	if n := len([]rune(body)); n > domain.ForumPostBodyMaxRunes {
		return invalidInput("正文最多 %d 个字符，当前 %d 个", domain.ForumPostBodyMaxRunes, n)
	}
	if strings.TrimSpace(body) == "" {
		return invalidInput("正文不能为空")
	}
	return nil
}

// validateReplyBody 校验回复正文。
func validateReplyBody(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return invalidInput("回复内容不能为空")
	}
	if n := len([]rune(body)); n > domain.ForumReplyBodyMaxRunes {
		return invalidInput("回复最多 %d 个字符，当前 %d 个", domain.ForumReplyBodyMaxRunes, n)
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 写入用例
 * ------------------------------------------------------------------ */

// CreatePost 新建帖子。
//
// scope 由路由分组决定（recruitment / studio），不经请求体传入，
// 这样用户无法把招新问答的帖子写进内部论坛。
func (uc *ForumUseCase) CreatePost(ctx context.Context, authorID int, scope domain.ForumScope, in CreateForumPostInput) (*domain.ForumPost, error) {
	title := strings.TrimSpace(in.Title)
	if err := validatePostInput(title, in.Body); err != nil {
		return nil, err
	}
	post := &domain.ForumPost{
		Scope:         scope,
		Title:         title,
		AuthorID:      authorID,
		DirectionCode: uc.resolveDirection(ctx, authorID, in.DirectionCode),
		Body:          in.Body,
	}
	if err := uc.posts.CreatePost(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// UpdatePost 编辑帖子（标题 + 正文）。
//
// 三步顺序不可调换：先取行（拿 author_id 做归属判断），再判权限，最后写。
// 无权与不存在都返回 ErrForumPostNotFound——返 403 等于告诉对方「这帖存在」。
func (uc *ForumUseCase) UpdatePost(ctx context.Context, userID int, postID int, in UpdateForumPostInput) (*domain.ForumPost, error) {
	post, err := uc.posts.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post.DeletedAt != nil {
		return nil, ErrForumPostNotFound
	}
	if !uc.canManage(ctx, post.AuthorID, userID) {
		return nil, ErrForumPostNotFound
	}
	title := strings.TrimSpace(in.Title)
	if err := validatePostInput(title, in.Body); err != nil {
		return nil, err
	}

	now := time.Now()
	post.Title = title
	post.Body = in.Body
	post.EditedAt = &now
	if err := uc.posts.UpdatePost(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// DeletePost 软删帖子。附件行与对象保留（ADR 0003：软删不连带清理存储）。
func (uc *ForumUseCase) DeletePost(ctx context.Context, userID int, postID int) error {
	post, err := uc.posts.GetPost(ctx, postID)
	if err != nil {
		return err
	}
	if post.DeletedAt != nil {
		return ErrForumPostNotFound
	}
	if !uc.canManage(ctx, post.AuthorID, userID) {
		return ErrForumPostNotFound
	}
	return uc.posts.SoftDeletePost(ctx, postID, userID)
}

// CreateReply 在帖子下新建回复。
//
// 必须先确认帖子可见：回复经 post_id 继承 scope，帖子已软删时往里写回复
// 会造出一条永远看不到的孤儿行。
func (uc *ForumUseCase) CreateReply(ctx context.Context, authorID int, postID int, in CreateForumReplyInput) (*domain.ForumReply, error) {
	post, err := uc.posts.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post.DeletedAt != nil {
		return nil, ErrForumPostNotFound
	}
	if err := validateReplyBody(in.Body); err != nil {
		return nil, err
	}
	reply := &domain.ForumReply{
		PostID:   postID,
		AuthorID: authorID,
		Body:     in.Body,
	}
	if err := uc.posts.CreateReply(ctx, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// UpdateReply 编辑回复。
//
// 编辑回复**不触发邮件通知**（ADR 0007：只有新回复才通知，编辑不重复打扰）。
func (uc *ForumUseCase) UpdateReply(ctx context.Context, userID int, replyID int, in CreateForumReplyInput) (*domain.ForumReply, error) {
	reply, err := uc.posts.GetReply(ctx, replyID)
	if err != nil {
		return nil, err
	}
	if reply.DeletedAt != nil {
		return nil, ErrForumReplyNotFound
	}
	if !uc.canManage(ctx, reply.AuthorID, userID) {
		return nil, ErrForumReplyNotFound
	}
	if err := validateReplyBody(in.Body); err != nil {
		return nil, err
	}

	now := time.Now()
	reply.Body = in.Body
	reply.EditedAt = &now
	if err := uc.posts.UpdateReply(ctx, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// DeleteReply 软删回复。
func (uc *ForumUseCase) DeleteReply(ctx context.Context, userID int, replyID int) error {
	reply, err := uc.posts.GetReply(ctx, replyID)
	if err != nil {
		return err
	}
	if reply.DeletedAt != nil {
		return ErrForumReplyNotFound
	}
	if !uc.canManage(ctx, reply.AuthorID, userID) {
		return ErrForumReplyNotFound
	}
	return uc.posts.SoftDeleteReply(ctx, replyID, userID)
}
