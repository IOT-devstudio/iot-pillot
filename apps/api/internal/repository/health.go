package repository

import (
	"context"

	"gorm.io/gorm"
)

// DatabasePinger 用 sql.DB.PingContext 探活数据库。
//
// 用 Ping 而不是跑一条 SELECT：Ping 会真正取一条连接，能反映连接池与网络状态，
// 代价也更低。它实现 handler.DependencyPinger（接口定义在使用方），
// 让 handler 包不必依赖 gorm。
type DatabasePinger struct {
	db *gorm.DB
}

func NewDatabasePinger(db *gorm.DB) *DatabasePinger {
	return &DatabasePinger{db: db}
}

func (p *DatabasePinger) Ping(ctx context.Context) error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
