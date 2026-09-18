package utils

import (
	"context"

	"github.com/redis/rueidis"
)

// RedisPinger 用 PING 命令探活 Redis。
//
// 它实现 handler.DependencyPinger（接口定义在使用方），让 handler 包不必依赖 rueidis。
type RedisPinger struct {
	client rueidis.Client
}

func NewRedisPinger(client rueidis.Client) *RedisPinger {
	return &RedisPinger{client: client}
}

func (p *RedisPinger) Ping(ctx context.Context) error {
	return p.client.Do(ctx, p.client.B().Ping().Build()).Error()
}
