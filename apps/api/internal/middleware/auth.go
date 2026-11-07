// Package middleware 提供 gin 中间件。
package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
)

// 角色取值。前端按 admin | member 判断可见性，后端按这里的值强制。
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// gin.Context 中保存认证结果的键。
const (
	ContextUserID   = "auth.user_id"
	ContextUsername = "auth.username"
	ContextRole     = "auth.role"
	// ContextScope 令牌范围（full / profile_only），见 utils/token_scope.go。
	ContextScope = "auth.scope"
)

// TokenValidator 是认证中间件对令牌校验的**全部**依赖。
//
// 刻意只依赖这一个方法而不是具体的 *utils.TokenManager：
//  1. 中间件不需要知道 Redis、签名算法、会话唯一码这些细节；
//  2. 测试可以注入桩实现，不必起 Redis 就能验证 401/403 的分支。
//
// *utils.TokenManager 的方法集天然满足这个接口，wire 直接传具体类型即可。
type TokenValidator interface {
	ValidateAccessToken(ctx context.Context, tokenString string) (userID int, username string, role string, err error)
	// ValidateAccessTokenWithScope 额外返回令牌范围。
	// 冻结账号登录拿到的受限令牌（profile_only）只能访问补资料相关端点。
	ValidateAccessTokenWithScope(ctx context.Context, tokenString string) (userID int, username string, role string, scope string, err error)
}

// extractBearerToken 从 Authorization 头取出 Bearer 令牌。
func extractBearerToken(c *gin.Context) (string, error) {
	header := c.GetHeader("Authorization")
	if header == "" {
		return "", errors.New("缺少 Authorization 请求头")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", errors.New("Authorization 头格式应为 Bearer <token>")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", errors.New("Authorization 头里没有令牌")
	}

	return token, nil
}

// AuthRequired 要求请求携带有效的 access token。
//
// "有效"包含签名、类型、过期时间以及 Redis 会话唯一码比对（重新登录或登出后
// 旧令牌立即失效），全部由 TokenValidator 负责。校验通过后把身份写进
// gin.Context，供后续 handler 与 RequireRole 使用。
func AuthRequired(validator TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractBearerToken(c)
		if err != nil {
			response.FailUnauthorized(c, err.Error())
			c.Abort()
			return
		}

		userID, username, role, scope, err := validator.ValidateAccessTokenWithScope(c.Request.Context(), token)
		if err != nil {
			// 精确区分失效原因（issue #84）：
			//   4010 纯过期（签名有效只是 exp 到了）→ 前端安静刷新即可；
			//   4011 会话失效（登出 / 被新登录顶替 / 改密后撤销）→ 必须重新登录；
			//   其余（存储不可用等）走 500 脱敏，绝不冒充成"登录冲突"。
			switch {
			case errors.Is(err, utils.ErrTokenExpired):
				response.FailTokenExpired(c, err.Error())
			case errors.Is(err, utils.ErrSessionRevoked),
				errors.Is(err, utils.ErrSessionSuperseded):
				response.FailSessionInvalidated(c, err.Error())
			default:
				failTokenStoreUnavailable(c, err)
			}
			c.Abort()
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextUsername, username)
		c.Set(ContextRole, role)
		c.Set(ContextScope, utils.NormalizeScope(scope))
		c.Next()
	}
}

// RequireFullScope 要求令牌是**完整权限**（非「仅补资料」受限令牌）。
//
// 背景（0012 冻结自助解冻）：资料不合规被冻结的账号登录时拿到的是
// profile_only 受限令牌，它只能访问 GET/PUT /me、POST /me/unfreeze、
// logout 与 refresh。其余所有业务端点都必须挂这个中间件。
//
// 为什么默认拒绝而不是默认放行：
// 忘记给新路由挂这个中间件，就会让受限令牌拿到完整权限——那是**静默的越权**，
// 没有报错、没有日志。反过来「忘记挂 → 拒绝访问」会立刻暴露出来。
// 所以这里对「上下文里没有 scope」也按拒绝处理。
//
// 必须挂在 AuthRequired 之后：没有身份信息时按 401 拒绝。
func RequireFullScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, _, _, ok := CurrentUser(c); !ok {
			response.FailUnauthorized(c, "缺少认证信息，请先通过 AuthRequired 中间件")
			c.Abort()
			return
		}

		raw, exists := c.Get(ContextScope)
		scope, isString := raw.(string)
		// 读不到或类型不对 → 拒绝（fail-closed）。
		if !exists || !isString {
			response.FailForbidden(c, "当前会话权限受限，请先补齐资料")
			c.Abort()
			return
		}

		if utils.NormalizeScope(scope) != utils.ScopeFull {
			// 受限会话是合法会话，只是权限范围小：
			// 不清会话、不返 401，否则前端会误判成「登录失效」把人甩回登录页。
			response.FailForbidden(c, "账号资料未补全，请先补齐后再使用其它功能")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminChecker 是管理员判定对外的全部依赖（只读那一个方法）。
//
// 用接口而不是 *utils.AdminStore：中间件不该知道管理员名单存在 Redis 还是别处，
// 测试也就能注入桩、不起 Redis 覆盖 403/放行两条分支。
type AdminChecker interface {
	IsAdmin(ctx context.Context, userID int) (bool, error)
}

// FoundingChecker 是创始判定对外的全部依赖（issue #142 Q1）。
//
// 这是创始标记（auth:admins:founding，#120）**第一次参与鉴权**——
// 仅限注销账号这一个端点，推翻 #120「仅展示分流」的裁决但边界锁死：
// 其余所有端点仍只看 IsAdmin，is_founding 字段也依旧只做展示。
type FoundingChecker interface {
	IsFounding(ctx context.Context, userID int) (bool, error)
}

// RequireAdmin 要求当前登录用户是管理员，否则 403。
//
// 判定读的是 **AdminChecker（Redis）**，不是 JWT 里的 role claim：
// 撤销管理员必须立刻生效，否则被撤销的人还能拿旧令牌继续访问管理端，
// 直到令牌自然过期。JWT 里的 role 只用于前端控制入口可见性。
//
// 必须挂在 AuthRequired 之后：缺少身份信息时按 401 拒绝（而不是放行），
// 这样"忘记加 AuthRequired"会表现为拒绝访问，而不是静默开门。
func RequireAdmin(checker AdminChecker) gin.HandlerFunc {
	return RequireAdminWithMessage(checker, "无权限访问该资源")
}

// RequireAdminWithMessage 与 RequireAdmin 相同，但可指定非管理员时的文案。
//
// 存在理由（#241）：默认文案「无权限访问该资源」对管理端点是对的——不能回显
// 管理端结构。但招新问答的「回答」不是隐藏资源，帖子对普通用户本来就可见，
// 通用文案会让前端只能显示一句没有信息量的话。spec 要求可读的中文提示
// （「只有管理员可以回答招新问答」），因此给这一类端点一个能说清原因的入口。
//
// 注意：这**不是**把权限判定挪进 handler——判定仍然只在这里做，
// 区别仅在这条 403 的措辞。
func RequireAdminWithMessage(checker AdminChecker, deniedMessage string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, _, ok := CurrentUser(c)
		if !ok {
			response.FailUnauthorized(c, "缺少认证信息，请先通过 AuthRequired 中间件")
			c.Abort()
			return
		}

		isAdmin, err := checker.IsAdmin(c.Request.Context(), userID)
		if err != nil {
			// 读不到名单时**拒绝**而不是放行：Redis 抽风不该变成一次提权窗口
			response.FailServer(c, "无法确认管理员身份，请稍后重试")
			c.Abort()
			return
		}

		if !isAdmin {
			// 默认不回显所需角色，避免向普通成员暴露管理端结构；
			// 调用方可传入更贴切但仍不泄漏结构的文案（见 RequireAdminWithMessage）。
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, deniedMessage)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireFounding 要求当前登录用户是创始管理员，否则 403（issue #142 Q1）。
//
// 必须挂在 RequireAdmin **之后**（admin 组天然满足）：先过管理员关，
// 再过创始关——普通成员 401/403 按既有口径，晋升管理员拿 403，
// 且不回显「创始」这个概念，避免向普通管理员暴露创始体系的存在。
//
// 读失败按 500 拒绝（同 RequireAdmin 的取舍）：Redis 抽风不能变成
// 一次绕过注销防护的窗口。
func RequireFounding(checker FoundingChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, _, ok := CurrentUser(c)
		if !ok {
			response.FailUnauthorized(c, "缺少认证信息，请先通过 AuthRequired 中间件")
			c.Abort()
			return
		}

		isFounding, err := checker.IsFounding(c.Request.Context(), userID)
		if err != nil {
			response.FailServer(c, "无法确认操作权限，请稍后重试")
			c.Abort()
			return
		}

		if !isFounding {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "无权限访问该资源")
			c.Abort()
			return
		}

		c.Next()
	}
}

// CurrentUser 读出 AuthRequired 写入的身份信息。
//
// 第二个返回值表示上下文里是否存在认证信息；handler 应当据此返回 401，
// 不要把零值当成真实用户。
func CurrentUser(c *gin.Context) (userID int, username string, role string, ok bool) {
	rawID, hasID := c.Get(ContextUserID)
	rawName, hasName := c.Get(ContextUsername)
	rawRole, hasRole := c.Get(ContextRole)
	if !hasID || !hasName || !hasRole {
		return 0, "", "", false
	}

	id, idOK := rawID.(int)
	name, nameOK := rawName.(string)
	roleValue, roleOK := rawRole.(string)
	if !idOK || !nameOK || !roleOK {
		return 0, "", "", false
	}

	return id, name, roleValue, true
}

// CurrentScope 读出当前令牌范围。
//
// 没有任何认证信息时返回空串（调用方应已先过 AuthRequired）。
// 缺失时按 ScopeFull 归一由 utils.NormalizeScope 负责，这里只做取值。
func CurrentScope(c *gin.Context) string {
	raw, exists := c.Get(ContextScope)
	if !exists {
		return ""
	}
	scope, ok := raw.(string)
	if !ok {
		return ""
	}
	return utils.NormalizeScope(scope)
}

// failTokenStoreUnavailable 令牌校验遇到非逻辑故障（Redis 不可用等）时的出口。
//
// 刻意走 500 而不是 401：把 Redis 故障报成"会话失效"会让用户被误导去重新登录，
// 而重新登录同样要写 Redis，只会再失败一次。这类故障必须显式暴露为服务端错误。
func failTokenStoreUnavailable(c *gin.Context, err error) {
	if errors.Is(err, utils.ErrTokenStoreUnavailable) {
		log.Printf("[middleware] 令牌校验因存储不可用而失败: %v", err)
		response.FailServer(c, "会话校验服务暂不可用，请稍后重试")
		return
	}
	// 兜底：未知错误按无效令牌处理，但不外泄原文
	log.Printf("[middleware] 令牌校验出现未知错误: %v", err)
	response.FailUnauthorized(c, "无效的令牌")
}
