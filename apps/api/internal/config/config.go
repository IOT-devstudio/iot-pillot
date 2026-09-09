// Package config 提供基于 viper 的配置加载。
// 优先级：环境变量 > 配置文件 > 默认值。
package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port int
	Mode string // "debug" | "release" | "test"
	CORS CORSConfig
	DB   DBConfig
	SMTP SMTPConfig
}

type CORSConfig struct {
	AllowedOrigins []string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
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
		Port: viper.GetInt("port"),
		Mode: viper.GetString("mode"),
		CORS: CORSConfig{
			AllowedOrigins: viper.GetStringSlice("cors.allowed_origins"),
		},
		DB: DBConfig{
			Host:     viper.GetString("db.host"),
			Port:     viper.GetInt("db.port"),
			User:     viper.GetString("db.user"),
			Password: viper.GetString("db.password"),
			Name:     viper.GetString("db.name"),
		},
		SMTP: SMTPConfig{
			Host:     viper.GetString("smtp.host"),
			Port:     viper.GetInt("smtp.port"),
			Username: viper.GetString("smtp.username"),
			Password: viper.GetString("smtp.password"),
			From:     viper.GetString("smtp.from"),
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
}
