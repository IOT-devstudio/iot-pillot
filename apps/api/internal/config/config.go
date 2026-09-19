// Package config 提供基于 viper 的配置加载。
// 优先级：环境变量 > 配置文件 > 默认值。
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	SERVICE *ServiceConfig
	CORS    *CORSConfig
	DB      *DBConfig
	SMTP    *SMTPConfig
	REDIS   *RedisConfig
	JWT     *JWTConfig
	CACHE   *CacheConfig
	AUTH    *AuthConfig
}

// AuthConfig 授权相关的配置。
//
// 这里**只有引导名单**：管理员名单的真源在 Redis（SET auth:admins，存 userID），
// 配置里的用户名只在启动时用来把账号补种进 Redis（见 service.AdminUseCase.SeedAdmins）。
//
// 曾经的 AuthConfig.IsAdmin 已经被删除：它会成为"谁是管理员"的第二个判定入口，
// 与 Redis 名单不一致时就会出现"配置说是、Redis 说不是"的分裂状态。
// 判定只能有一个真源。
type AuthConfig struct {
	// AdminUsers 启动引导用的用户名列表。
	// 为空表示本次启动不补种任何管理员（Redis 里已有的管理员不受影响）。
	AdminUsers []string
}

type ServiceConfig struct {
	Port int
	Mode string // "debug" | "release" | "test"
}

type RedisConfig struct {
	Host            string
	Port            int
	Password        string
	DB              int
	PoolSize        int
	ConnWithTimeout time.Duration
}

type JWTConfig struct {
	Secret string
	Expire int
}

type CORSConfig struct {
	AllowedOrigins []string
}

type DBConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Name     string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TimeoutSeconds SMTP 拨号超时（秒）。<=0 时由发送方回落到内置默认值。
	// 单位是纯秒数，读取时只乘一次 time.Second（不要重蹈 conn_with_timeout 的覆辙）。
	TimeoutSeconds int
}

type CacheConfig struct {
	BaseKey string
	Expire  int
}

func Load() (*Config, error) {
	viper.SetEnvPrefix("IOT_PILOT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults()

	// 配置文件可选：存在则加载，不存在不报错。
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	_ = viper.ReadInConfig()

	cfg := &Config{
		SERVICE: &ServiceConfig{
			Port: viper.GetInt("port"),
			Mode: viper.GetString("mode"),
		},
		CORS: &CORSConfig{
			AllowedOrigins: viper.GetStringSlice("cors.allowed_origins"),
		},
		DB: &DBConfig{
			Host:     viper.GetString("db.host"),
			Port:     viper.GetInt("db.port"),
			Username: viper.GetString("db.user"),
			Password: viper.GetString("db.password"),
			Name:     viper.GetString("db.name"),
		},
		SMTP: &SMTPConfig{
			Host:           viper.GetString("smtp.host"),
			Port:           viper.GetInt("smtp.port"),
			Username:       viper.GetString("smtp.username"),
			Password:       viper.GetString("smtp.password"),
			From:           viper.GetString("smtp.from"),
			TimeoutSeconds: viper.GetInt("smtp.timeout_seconds"),
		},
		REDIS: &RedisConfig{
			Host:            viper.GetString("redis.host"),
			Port:            viper.GetInt("redis.port"),
			Password:        viper.GetString("redis.password"),
			DB:              viper.GetInt("redis.db"),
			PoolSize:        viper.GetInt("redis.pool_size"),
			ConnWithTimeout: loadSeconds("redis.conn_with_timeout", 5*time.Second),
		},
		JWT: &JWTConfig{
			Secret: viper.GetString("jwt.secret"),
			Expire: viper.GetInt("jwt.expire"),
		},
		AUTH: &AuthConfig{
			AdminUsers: loadAdminUsers(),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadAdminUsers 读取管理员引导名单（用户名或 userID，见 AdminUseCase.SeedAdmins）。
//
// 刻意不用 viper.GetStringSlice：它内部走 cast.ToStringSlice，对字符串类型用的是
// strings.Fields（**按空白切分**）。于是 IOT_PILOT_AUTH_ADMIN_USERS=drayee,alice
// 会得到 ["drayee,alice"] 这一个元素 —— 名单静默失效、谁都不是管理员，且不报错。
// 所以这里按值的实际类型分派。
//
// 数字类型也要收：YAML 里写 admin_users: 1（不带引号）viper 返回的就是 int，
// 若落到 default 分支会被静默丢掉，表现同样是"配了却没生效"。
func loadAdminUsers() []string {
	switch raw := viper.Get("auth.admin_users").(type) {
	case []string:
		return normalizeNames(raw)
	case []any:
		names := make([]string, 0, len(raw))
		for _, item := range raw {
			switch value := item.(type) {
			case string:
				names = append(names, value)
			case int:
				names = append(names, strconv.Itoa(value))
			case int64:
				names = append(names, strconv.FormatInt(value, 10))
			case float64:
				names = append(names, strconv.FormatInt(int64(value), 10))
			}
		}
		return normalizeNames(names)
	case string:
		return normalizeNames(strings.Split(raw, ","))
	case int:
		return normalizeNames([]string{strconv.Itoa(raw)})
	case int64:
		return normalizeNames([]string{strconv.FormatInt(raw, 10)})
	case float64:
		return normalizeNames([]string{strconv.FormatInt(int64(raw), 10)})
	default:
		return nil
	}
}

// normalizeNames 去掉空白项与重复项，并裁掉首尾空格（配置里多写的空格不该让人登不进去）。
func normalizeNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))

	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	return result
}

// loadSeconds 读取以「秒」为单位的时长配置。
//
// 同一个键要同时接受三种来源，否则会静默失效：
//   - 配置文件/环境变量里的纯数字 5   → 5 秒（configs/config.example.yaml 的写法）
//   - 时长字符串 "5s"                → 5 秒
//   - SetDefault 传进来的 time.Duration → 原样返回
//
// 历史坑（已由 TestLoad_RedisConnWithTimeout* 锁住）：原来是
// `viper.GetDuration(key) * time.Second`。SetDefault 传的是 5*time.Second（= 5e9 纳秒），
// GetDuration 又把它当纳秒原样返回 5e9，再乘一次 time.Second 就成了 5e18 纳秒
// ≈ 158 年 —— Redis 写超时形同不存在，而且不会报任何错。
func loadSeconds(key string, fallback time.Duration) time.Duration {
	switch raw := viper.Get(key).(type) {
	case time.Duration:
		// SetDefault 存进来的就是 Duration，直接用，不要再乘
		return raw
	case int:
		return time.Duration(raw) * time.Second
	case int64:
		return time.Duration(raw) * time.Second
	case float64:
		return time.Duration(raw * float64(time.Second))
	case string:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return fallback
		}
		// 先按 "5s" / "1m30s" 这类时长字符串解析
		if parsed, err := time.ParseDuration(trimmed); err == nil {
			return parsed
		}
		// 再按纯数字秒解析（"5"）
		if seconds, err := strconv.Atoi(trimmed); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}

	return fallback
}

// validate 校验没有安全默认值的配置项。
//
// 这些项不能靠 SetDefault 兜底：JWT 密钥留空会让 HS256 用空密钥签名，
// 任何人都能伪造令牌，静默降级比启动失败危险得多。
func (c *Config) validate() error {
	if c.JWT.Secret == "" {
		return errors.New("配置缺失：jwt.secret（环境变量 IOT_PILOT_JWT_SECRET）。" +
			"JWT 使用 HS256 签名，密钥为空等于任何人都能伪造令牌，拒绝启动")
	}
	if c.JWT.Expire <= 0 {
		return fmt.Errorf("配置无效：jwt.expire 必须为正整数秒，当前为 %d", c.JWT.Expire)
	}
	return nil
}

func setDefaults() {
	viper.SetDefault("port", 8080)
	viper.SetDefault("mode", "debug")
	viper.SetDefault("cors.allowed_origins", []string{"http://localhost:5173"})
	viper.SetDefault("db.host", "localhost")
	viper.SetDefault("db.port", 5432)
	viper.SetDefault("db.user", "iot_pillot")
	viper.SetDefault("db.password", "iot_pillot")
	viper.SetDefault("db.name", "iot_pillot")
	viper.SetDefault("smtp.port", 587)
	// SMTP 拨号超时（秒）。没有超时的 SMTP 调用会永久挂住发信请求。
	viper.SetDefault("smtp.timeout_seconds", 10)
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.conn_with_timeout", 5*time.Second)
	viper.SetDefault("jwt.expire", 3600)
	viper.SetDefault("auth.admin_users", "")
}
