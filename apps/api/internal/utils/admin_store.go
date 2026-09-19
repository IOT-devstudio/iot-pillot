package utils

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/redis/rueidis"
)

// adminSetKey 管理员名单在 Redis 里的 key。
//
// 存的是**用户 ID**（SET 成员是字符串，序列化成十进制），不是用户名：
//   - 用户名可以被改（domain/user.go 甚至去掉了 name 的唯一约束），ID 不会；
//   - 发信、改角色等管理端操作本来就以 userID 为主，避免来回转换；
//   - 配置里的用户名白名单只在启动引导时用来查一次库，之后不再参与判定。
const adminSetKey = "auth:admins"

// AdminDirectory 是角色判定的全部契约。
//
// 刻意定义在实现旁边（而不是各消费者各写一份）：管理员名单只有这一个真源，
// 中间件与管理员服务都依赖同一份契约，避免两处接口各自漂移后判定不一致。
// 消费者拿接口做参数，测试即可注入桩实现，不必起 Redis。
type AdminDirectory interface {
	IsAdmin(ctx context.Context, userID int) (bool, error)
	AddAdmin(ctx context.Context, userID int) error
	RemoveAdmin(ctx context.Context, userID int) error
	ListAdmins(ctx context.Context) ([]int, error)
}

// AdminStore 基于 Redis SET 的管理员名单。
type AdminStore struct {
	redis rueidis.Client
}

func NewAdminStore(redis rueidis.Client) *AdminStore {
	return &AdminStore{redis: redis}
}

// NewAdminDirectory 把 *AdminStore 作为 AdminDirectory 提供给装配层。
//
// wire 按**类型**连线，Go 的隐式接口满足对它无效，所以装配层需要一个"返回值就是
// 接口"的 provider（原来的写法是 wire.Bind(new(AdminDirectory), new(*AdminStore))）。
// 这类函数只能放在接口所在的包：AdminDirectory 定义在这里，转换也就写在这里。
func NewAdminDirectory(store *AdminStore) AdminDirectory {
	return store
}

// IsAdmin 判定某个用户当前是不是管理员。
//
// 每次请求都查 Redis，而不是信任 JWT 里的 role claim：
// 撤销管理员必须**立即生效**，否则被撤销的人还能用旧令牌继续访问管理端，
// 直到令牌自然过期（默认 1 小时）。JWT 里的 role 只作为前端可见性的提示。
func (s *AdminStore) IsAdmin(ctx context.Context, userID int) (bool, error) {
	cmd := s.redis.B().Sismember().Key(adminSetKey).Member(strconv.Itoa(userID)).Build()

	ok, err := s.redis.Do(ctx, cmd).ToBool()
	if err != nil {
		return false, fmt.Errorf("读取管理员名单失败: %w", err)
	}

	return ok, nil
}

// AddAdmin 把用户加入管理员名单（幂等）。
func (s *AdminStore) AddAdmin(ctx context.Context, userID int) error {
	cmd := s.redis.B().Sadd().Key(adminSetKey).Member(strconv.Itoa(userID)).Build()
	if err := s.redis.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("写入管理员名单失败: %w", err)
	}
	return nil
}

// RemoveAdmin 把用户移出管理员名单（幂等）。
func (s *AdminStore) RemoveAdmin(ctx context.Context, userID int) error {
	cmd := s.redis.B().Srem().Key(adminSetKey).Member(strconv.Itoa(userID)).Build()
	if err := s.redis.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("移除管理员失败: %w", err)
	}
	return nil
}

// ListAdmins 列出全部管理员 userID（升序）。
//
// 名单里混进非法成员时**跳过而不是报错**：一条脏数据不该让整个管理端不可用。
func (s *AdminStore) ListAdmins(ctx context.Context) ([]int, error) {
	cmd := s.redis.B().Smembers().Key(adminSetKey).Build()

	members, err := s.redis.Do(ctx, cmd).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("读取管理员名单失败: %w", err)
	}

	return ParseAdminIDs(members), nil
}

// SeedAdmins 启动引导：把给定 userID 幂等补种进 Redis。
//
// 只做增（不删）：配置里的名单是"初始有哪些管理员"，不是"当前只允许这些管理员"。
// 若改成覆盖，运维在管理端新加的管理员会在下次重启时被静默抹掉。
func (s *AdminStore) SeedAdmins(ctx context.Context, userIDs []int) error {
	if len(userIDs) == 0 {
		return nil
	}

	members := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		members = append(members, strconv.Itoa(id))
	}

	cmd := s.redis.B().Sadd().Key(adminSetKey).Member(members...).Build()
	if err := s.redis.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("补种管理员名单失败: %w", err)
	}

	return nil
}

// ParseAdminIDs 把 Redis 里的字符串成员解析成 userID：升序、去重。
//
// 非数字与 <=0 的成员一律跳过：Redis 是外部可写状态，
// 别人手工 SADD 进去的脏值不该把管理端整个弄崩。
//
// 虽然 Redis SET 天然去重，仍然在这里去一次：这个函数是通用的解析入口，
// 一旦将来名单换成 LIST 或调用方自己拼切片，重复 ID 会让管理端
// 出现同一个人的两行（撤销时只删一行、另一行还在，很难查）。
func ParseAdminIDs(members []string) []int {
	seen := make(map[int]struct{}, len(members))
	ids := make([]int, 0, len(members))

	for _, member := range members {
		id, err := strconv.Atoi(member)
		if err != nil || id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	sort.Ints(ids)
	return ids
}
