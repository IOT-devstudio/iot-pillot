package service

import (
	"context"
	"sort"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// ErrForumPostNotFound 帖子不存在或不可见（软删 / 跨域越权统一映射成 404，见 ADR 0003）。
var ErrForumPostNotFound = repository.ErrForumPostNotFound

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// ForumUseCase 招新问答 / 内部论坛的只读用例（#219）。
//
// 写入（新建帖/回复、软删、标记已解决、官方回答持久化）属于 #220/#221，本期不暴露。
//
// 可见性分工（ADR 0003 决策 3）：
//   - 粗粒度「哪个 scope 要什么角色」由路由分组负责（recruitment 挂登录用户组、
//     studio 挂 admin 组），handler 从 context 读 scope，永不解析 URL；
//   - 细粒度「这条记录归不归你」本期只读用不到（没有改/删入口）；
//   - 软删对所有人不可见（包括管理员），因此在这里统一拦截成 ErrForumPostNotFound，
//     让上层回 404 而不是 403——既符合「软删从常规列表消失」，也避免用 403 暴露
//     资源存在（ADR 0004 §1 同口径）。
type ForumUseCase struct {
	posts repository.ForumRepo
	admins utils.AdminDirectory
	// users / applications 只在发帖时算「方向回落」用（#220），
	// 可为 nil —— 为 nil 时该级直接跳过，只读用例因此不受影响。
	users        userDirectionReader
	applications applicationDirectionReader
}

// NewForumUseCase 构造函数。admins 用于「官方回答（管理员回复）置顶」的实时判定。
func NewForumUseCase(posts repository.ForumRepo, admins utils.AdminDirectory) *ForumUseCase {
	return &ForumUseCase{posts: posts, admins: admins}
}

// WithDirectionFallback 注入方向回落所需的两级数据源（资料方向 / 最新报名方向）。
//
// 单独开一个链式方法而不是加进 NewForumUseCase：只读用例（#219）不需要这两个依赖，
// 改构造函数签名会波及既有调用与测试桩。
func (uc *ForumUseCase) WithDirectionFallback(users userDirectionReader, apps applicationDirectionReader) *ForumUseCase {
	uc.users = users
	uc.applications = apps
	return uc
}

// ListPosts 分页列出某 scope 下未软删的帖子，支持关键词与方向筛选。
//
// scope 的合法性由 handler 在路由层保证（recruitment / studio 走不同分组），
// 这里直接信任入参。可见性（软删过滤）已下沉到 repo 的 `deleted_at IS NULL`。
func (uc *ForumUseCase) ListPosts(ctx context.Context, scope domain.ForumScope, filter repository.ForumPostListFilter, page int, pageSize int) ([]*repository.ForumPostSummary, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return uc.posts.ListPosts(ctx, scope, filter, page, pageSize)
}

// ForumPostDetail 帖子详情：帖子正文 + 已排序的回复 + 每条回复是否官方回答。
//
// replies 已按「官方回答优先、其余按时间正序」排好；official[i] 与 replies[i] 对齐。
// 软删帖与不存在一律返回 ErrForumPostNotFound（上层回 404）。
type ForumPostDetail struct {
	Post     *domain.ForumPost
	Replies  []*domain.ForumReply
	Official []bool
}

// GetPostDetail 取帖子详情与回复，并实时判定官方回答（ADR 0003 决策 8：
// 不新增数据库列，是否官方 = 回复作者是否为管理员，现查 AdminDirectory）。
func (uc *ForumUseCase) GetPostDetail(ctx context.Context, id int) (*ForumPostDetail, error) {
	post, err := uc.posts.GetPost(ctx, id)
	if err != nil {
		return nil, err // ErrForumPostNotFound 透传
	}
	// 软删对所有人不可见：常规列表根本不返回它，按 id 直访也 404 而非 403。
	if post.DeletedAt != nil {
		return nil, ErrForumPostNotFound
	}

	replies, err := uc.posts.ListReplies(ctx, id)
	if err != nil {
		return nil, err
	}

	official := make([]bool, len(replies))
	for i, r := range replies {
		isAdmin, err := uc.admins.IsAdmin(ctx, r.AuthorID)
		if err != nil {
			// Redis 读不到名单时**不能**把普通回复误判成官方（那会篡改排序），
			// 也不能直接失败——只读详情失败会让整个页面挂掉。按「非官方」保守处理，
			// 排序只受已确认的管理员影响，名单临时不可用时页面依旧可用。
			isAdmin = false
		}
		official[i] = isAdmin
	}

	// 官方回答置顶：isOfficial 降序、created_at 升序（同档内时间正序）。
	// replies 与 official 必须同步重排，否则排序后两者错位（官方标记错配到别的回复）。
	type pair struct {
		reply   *domain.ForumReply
		official bool
	}
	pairs := make([]pair, len(replies))
	for i := range replies {
		pairs[i] = pair{replies[i], official[i]}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].official != pairs[j].official {
			return pairs[i].official // true 在前
		}
		return pairs[i].reply.CreatedAt.Before(pairs[j].reply.CreatedAt)
	})
	outReplies := make([]*domain.ForumReply, len(pairs))
	outOfficial := make([]bool, len(pairs))
	for i := range pairs {
		outReplies[i] = pairs[i].reply
		outOfficial[i] = pairs[i].official
	}

	return &ForumPostDetail{Post: post, Replies: outReplies, Official: outOfficial}, nil
}
