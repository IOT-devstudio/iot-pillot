// Package config 提供基于 viper 的配置加载。
// 优先级：环境变量 > 配置文件 > 默认值。
package config

import (
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

	return &Config{
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
			Username: viper.GetString("db.username"),
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
	}, nil
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
}
