// Package middleware 提供 gin 中间件。
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"

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

		userID, username, role, err := validator.ValidateAccessToken(c.Request.Context(), token)
		if err != nil {
			response.FailUnauthorized(c, err.Error())
			c.Abort()
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextUsername, username)
		c.Set(ContextRole, role)
		c.Next()
	}
}

// RequireRole 要求当前登录用户具备指定角色，否则 403。
//
// 必须挂在 AuthRequired 之后：没有身份信息时按 401 处理（而不是放行），
// 这样"忘记加 AuthRequired"会表现为拒绝访问，而不是静默放行。
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, exists := c.Get(ContextRole)
		if !exists {
			response.FailUnauthorized(c, "缺少认证信息，请先通过 AuthRequired 中间件")
			c.Abort()
			return
		}

		currentRole, _ := current.(string)
		if currentRole != role {
			// 403 而不是 404：这里不回显所需角色，避免向普通成员暴露管理端结构
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
