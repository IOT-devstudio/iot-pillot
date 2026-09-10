package repository

import (
	"fmt"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewGORM(cfg *config.DBConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host,
		cfg.Username,
		cfg.Password,
		cfg.Name,
		cfg.Port,
	)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 获取底层的通用数据库对象 sql.DB，以便配置连接池
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}

	// 复用上面讲过的标准连接池配置
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Minute * 15)

	return gormDB, nil
}

func ProvideDB(cfg *config.Config) (*gorm.DB, func(), error) {
	// 1. 初始化
	database, err := NewGORM(cfg.DB)
	if err != nil {
		return nil, nil, err
	}

	// 3. 定义清理逻辑
	cleanup := func() {
		sqlDB, _ := database.DB()
		if sqlDB != nil {
			err := sqlDB.Close()
			if err != nil {
				return
			}
		}
	}

	// 返回 (实例, 清理函数, error)
	return database, cleanup, nil
}
