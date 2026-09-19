package main

import (
	"context"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// provideAdminUseCase 构造管理员服务，并在启动时执行一次管理员引导。
//
// 为什么引导放在 provider 里而不是 main：它需要 DB（把用户名解析成 userID）
// 与 Redis 两个依赖，而这两个都由 wire 负责装配。放在这里就不用把依赖
// 一路透传到 main.go。
//
// 引导是**尽力而为**的：名单里的用户名找不到账号时只记日志（新环境里
// 管理员往往还没注册），因此不会因为配置里写错一个名字就让服务起不来。
// 但 Redis 报错会返回错误 —— 那种情况下管理员名单不可信，管理端的行为
// 无法预期，宁可拒绝启动。
//
// 注意：SeedAdmins 只做幂等新增（不删除）。配置表达的是"初始管理员"，
// 而不是"当前只允许这些管理员"；覆盖式同步会把运维在管理端新加的人抹掉。
func provideAdminUseCase(
	userRepo repository.UserRepo,
	adminStore *utils.AdminStore,
	tokenManager *utils.TokenManager,
	cfg *config.Config,
) (*service.AdminUseCase, error) {
	useCase := service.NewAdminUseCase(userRepo, adminStore, tokenManager)

	if err := useCase.SeedAdmins(context.Background(), cfg.AUTH.AdminUsers); err != nil {
		return nil, err
	}

	return useCase, nil
}
