package utils

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/rueidis"
)

// Counter 限流所需的 Redis 原语。
//
// 定义成窄接口而不是直接用 rueidis.Client：限流的分支逻辑（第 5 次失败锁定、
// 冷却窗口内拒绝、窗口只在第一次计数时设置）是真正会写错的部分，
// 注入桩就能把它们全部覆盖，不必起一个 Redis。
type Counter interface {
	// Incr 自增并返回新值；第一次自增时设置窗口（后续自增不再延长窗口，
	// 否则持续尝试会让窗口永远不过期）。
	Incr(ctx context.Context, key string, window time.Duration) (int64, error)
	// SetIfAbsent key 不存在时写入 1 并设置 TTL，返回是否为本次写入。
	SetIfAbsent(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Get(ctx context.Context, key string) (int64, error)
	Del(ctx context.Context, keys ...string) error
}

// RedisCounter 基于 Redis 的实现。
type RedisCounter struct {
	redis rueidis.Client
}

func NewRedisCounter(redis rueidis.Client) *RedisCounter {
	return &RedisCounter{redis: redis}
}

// NewCounter 把 *RedisCounter 作为 Counter 提供给装配层。
//
// 与 NewAdminDirectory 同理：wire 不会自动把具体类型当作接口，
// 接口声明在本包，转换函数就放在本包。
func NewCounter(counter *RedisCounter) Counter {
	return counter
}

func (c *RedisCounter) Incr(ctx context.Context, key string, window time.Duration) (int64, error) {
	count, err := c.redis.Do(ctx, c.redis.B().Incr().Key(key).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("计数失败: %w", err)
	}

	// 只在 1 → 2 的那一次设置窗口，保证窗口是固定的而不是滑动的
	if count == 1 {
		expire := c.redis.B().Expire().Key(key).Seconds(int64(window.Seconds())).Build()
		if err := c.redis.Do(ctx, expire).Error(); err != nil {
			return count, fmt.Errorf("设置计数窗口失败: %w", err)
		}
	}

	return count, nil
}

func (c *RedisCounter) SetIfAbsent(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	cmd := c.redis.B().Set().Key(key).Value("1").Nx().Ex(ttl).Build()
	err := c.redis.Do(ctx, cmd).Error()
	if err == nil {
		return true, nil
	}
	// NX 未命中时 Redis 返回 nil，这不是故障，而是"窗口还没过"
	if rueidis.IsRedisNil(err) {
		return false, nil
	}
	return false, fmt.Errorf("写入冷却标记失败: %w", err)
}

func (c *RedisCounter) Get(ctx context.Context, key string) (int64, error) {
	value, err := c.redis.Do(ctx, c.redis.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return 0, nil
		}
		return 0, err
	}

	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		// 脏值按 0 处理：一个坏 key 不该让登录或注册整体不可用
		return 0, nil
	}

	return count, nil
}

func (c *RedisCounter) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.redis.Do(ctx, c.redis.B().Del().Key(keys...).Build()).Error()
}

/* ------------------------------------------------------------------ *
 * 限流 key
 * ------------------------------------------------------------------ */

// 发送验证码：同一邮箱的冷却 + 同一来源 IP 的小时配额。
func VerifyCooldownKey(email string) string {
	return "limit:verify_code:cooldown:" + email
}

func VerifyIPQuotaKey(ip string) string {
	return "limit:verify_code:ip:" + ip
}

// 登录失败锁定。
func LoginFailureKey(username string) string {
	return "limit:login:fail:" + username
}

/* ------------------------------------------------------------------ *
 * 限流规则
 * ------------------------------------------------------------------ */

const (
	// VerifyResendCooldown 同一邮箱重新获取验证码的最小间隔。
	VerifyResendCooldown = 60 * time.Second
	// VerifyIPHourlyQuota 同一来源 IP 每小时最多请求多少次验证码。
	VerifyIPHourlyQuota = 10
	VerifyIPQuotaWindow = time.Hour

	// LoginMaxFailures 连续登录失败上限。
	LoginMaxFailures = 5
	// LoginLockWindow 达到上限后的锁定时长。
	LoginLockWindow = 15 * time.Minute
)

// Limiter 限流判定。逻辑集中在这里，调用方只负责给出 key。
type Limiter struct {
	counter Counter
}

func NewLimiter(counter Counter) *Limiter {
	return &Limiter{counter: counter}
}

// AllowOnce 冷却判定：窗口内只允许一次，返回 true 表示本次放行。
func (l *Limiter) AllowOnce(ctx context.Context, key string, window time.Duration) (bool, error) {
	return l.counter.SetIfAbsent(ctx, key, window)
}

// AllowWithin 配额判定：窗口内最多 limit 次。
func (l *Limiter) AllowWithin(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	count, err := l.counter.Incr(ctx, key, window)
	if err != nil {
		return false, err
	}
	return count <= limit, nil
}

// RecordFailure 记一次失败，返回是否已达锁定上限。
func (l *Limiter) RecordFailure(ctx context.Context, key string, limit int64, window time.Duration) (int64, bool, error) {
	count, err := l.counter.Incr(ctx, key, window)
	if err != nil {
		return 0, false, err
	}
	return count, count >= limit, nil
}

// FailureCount 当前失败次数。
func (l *Limiter) FailureCount(ctx context.Context, key string) (int64, error) {
	return l.counter.Get(ctx, key)
}

// ClearFailures 登录成功后清零失败计数。
func (l *Limiter) ClearFailures(ctx context.Context, key string) error {
	return l.counter.Del(ctx, key)
}
