package utils

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/redis/rueidis"
	"golang.org/x/sync/singleflight"
)

func ConnectRedis(cfg *config.Config) rueidis.Client {
	if client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress:      []string{cfg.REDIS.Host + ":" + strconv.Itoa(cfg.REDIS.Port)},
		Password:         cfg.REDIS.Password,
		SelectDB:         cfg.REDIS.DB,
		BlockingPoolSize: cfg.REDIS.PoolSize,
		ConnWriteTimeout: cfg.REDIS.ConnWithTimeout,
	}); err == nil {
		return client
	} else {
		log.Fatalf("Failed to connect to Redis: %v", err)
		return nil
	}

}

type CacheManager struct {
	client  rueidis.Client
	baseKey string
	ttl     time.Duration
	group   singleflight.Group
}

func NewCacheManager(client rueidis.Client, cfg *config.Config) *CacheManager {
	// 注意：baseKey 不带尾部冒号，Key/Fetch 内部用 ":" 拼接各段，
	// 避免生成 "private:cache::xxx" 双冒号 key，与战斗/全局服务读取的
	// "{baseKey}:ws-code:{userID}" 保持一致。
	return &CacheManager{client: client, baseKey: cfg.CACHE.BaseKey, ttl: time.Duration(cfg.CACHE.Expire) * time.Second}
}

// Key 拼接缓存 key：{baseKey}:{part1}:{part2}...
func (cm *CacheManager) Key(parts ...string) string {
	return strings.Join(append([]string{cm.baseKey}, parts...), ":")
}

// TTLSeconds 返回缓存默认过期秒数
func (cm *CacheManager) TTLSeconds() int64 {
	return int64(cm.ttl / time.Second)
}

func Fetch[T any](cm *CacheManager, ctx context.Context, key string, value string, ttl time.Duration, fetcher func() (T, error)) (T, error) {
	var zero T // 用于在发生错误时返回默认零值
	cacheKey := strings.Join([]string{cm.baseKey, key, value}, ":")
	if ttl == 0 {
		ttl = cm.ttl
	}

	// 1. 尝试从缓存获取
	getCmd := cm.client.B().Get().Key(cacheKey).Cache()
	resp := cm.client.DoCache(ctx, getCmd, ttl)

	if err := resp.Error(); err == nil {
		// 缓存命中！获取字符串并反序列化为泛型 T
		val, _ := resp.ToString()
		var result T
		if jsonErr := json.Unmarshal([]byte(val), &result); jsonErr == nil {
			return result, nil // 成功反序列化，直接返回
		}
		// 如果反序列化失败，说明缓存数据脏了，继续往下走去查库
	} else if !rueidis.IsRedisNil(err) {
		log.Printf("Redis error on %s: %v", cacheKey, err)
	}

	// 2. 缓存未命中，使用 Singleflight 并发控制去查底层数据
	res, err, _ := cm.group.Do(cacheKey, func() (any, error) {

		// 2.1 执行真正的业务逻辑 (不管里面需要几个参数，都在外部包装好)
		data, fetchErr := fetcher()
		if fetchErr != nil {
			return nil, fetchErr
		}

		// 2.2 序列化查出的数据
		bytes, jsonErr := json.Marshal(data)
		if jsonErr != nil {
			return data, nil // 序列化失败，但不影响返回真实数据，只是不存缓存了
		}

		// 2.3 异步写入 Redis
		go func() {
			// 使用独立的 context 防止 HTTP 请求被取消导致写入失败
			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			setCmd := cm.client.B().Set().Key(cacheKey).Value(string(bytes)).Ex(ttl).Build()
			if setErr := cm.client.Do(bgCtx, setCmd).Error(); setErr != nil {
				log.Printf("Failed to set cache %s: %v", cacheKey, setErr)
			}
		}()

		return data, nil
	})

	if err != nil {
		return zero, err
	}

	// 3. 安全地转换为泛型 T 并返回
	return res.(T), nil
}
