// Package config 提供基于 viper 的配置加载。
// 优先级：环境变量 > 配置文件 > 默认值。
package config

import (
	"errors"
	"fmt"
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

// AuthConfig 授权相关的部署级配置。
//
// 角色**不落库**：role 只是「谁能进管理端」这一授权事实，属于部署配置而不是
// 用户数据，放配置里既能审计（改配置要走发布流程）又不需要 schema 迁移。
// 等 M7 要做「用户列表里运行时改角色」时，再给 User 加 role 列并配迁移。
type AuthConfig struct {
	// AdminUsers 具备 admin 角色的用户名白名单。
	// 为空表示系统里没有管理员，管理端路由对所有人都是 403。
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
			Host:     viper.GetString("smtp.host"),
			Port:     viper.GetInt("smtp.port"),
			Username: viper.GetString("smtp.username"),
			Password: viper.GetString("smtp.password"),
			From:     viper.GetString("smtp.from"),
		},
		REDIS: &RedisConfig{
			Host:            viper.GetString("redis.host"),
			Port:            viper.GetInt("redis.port"),
			Password:        viper.GetString("redis.password"),
			DB:              viper.GetInt("redis.db"),
			PoolSize:        viper.GetInt("redis.pool_size"),
			ConnWithTimeout: viper.GetDuration("redis.conn_with_timeout") * time.Second,
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

// loadAdminUsers 读取管理员用户名白名单。
//
// 刻意不用 viper.GetStringSlice：它内部走 cast.ToStringSlice，对字符串类型用的是
// strings.Fields（**按空白切分**）。于是 IOT_PILOT_AUTH_ADMIN_USERS=drayee,alice
// 会得到 ["drayee,alice"] 这一个元素 —— 名单静默失效、谁都不是管理员，且不报错。
// 所以这里按值的实际类型分派：字符串按逗号切，配置文件里的 YAML 列表直接用。
func loadAdminUsers() []string {
	switch raw := viper.Get("auth.admin_users").(type) {
	case []string:
		return normalizeNames(raw)
	case []any:
		names := make([]string, 0, len(raw))
		for _, item := range raw {
			if name, ok := item.(string); ok {
				names = append(names, name)
			}
		}
		return normalizeNames(names)
	case string:
		return normalizeNames(strings.Split(raw, ","))
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

// IsAdmin 判断用户名是否在管理员白名单中。
func (a *AuthConfig) IsAdmin(username string) bool {
	if a == nil {
		return false
	}
	for _, name := range a.AdminUsers {
		if name == username {
			return true
		}
	}
	return false
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
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.conn_with_timeout", 5*time.Second)
	// jwt.expire 单位是秒（auth_util.go: time.Duration(expireSeconds)*time.Second）。
	// 没有默认值时为 0，会让 exp = time.Now()，令牌签发即过期、认证完全不可用。
	viper.SetDefault("jwt.expire", 3600)
	// jwt.secret 故意不设默认值，改由 Config.validate() 强制要求。
	// 密钥没有"安全的默认值"可言。
	// 管理员白名单默认为空：默认没有任何管理员是安全的（要显式配置才能开管理端），
	// 反过来"默认有管理员"会让每个环境都带一个已知的提权入口。
	viper.SetDefault("auth.admin_users", "")
}
