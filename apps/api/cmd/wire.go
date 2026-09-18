//go:build wireinject
// +build wireinject

package main

import (
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/handler"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
	"github.com/google/wire"
)

func InitializeApp() (*handler.Router, func(), error) {
	wire.Build(
		config.Load,
		repository.ProvideDB,
		utils.ConnectRedis,

		utils.NewCodeManager,
		utils.NewMailManager,
		utils.NewTokenManager,
		utils.NewAdminStore,

		// Go 的隐式接口满足对 wire 无效：它的依赖图按**类型**连线，
		// 不会自动把 *utils.AdminStore 当成 utils.AdminDirectory。
		// 所以凡是 provider 参数用接口的地方，都要显式 Bind 一次。
		wire.Bind(new(utils.AdminDirectory), new(*utils.AdminStore)),

		repository.NewUserRepo,
		repository.NewMailModelRepo,
		repository.NewMailRepo,

		// 邮件服务依赖 MailSender 接口而非 *utils.MailManager：
		// 同样需要显式 Bind，理由见上面的 AdminDirectory。
		wire.Bind(new(service.MailSender), new(*utils.MailManager)),

		service.NewAuthUseCase,
		service.NewMailUseCase,
		// provideAdminUseCase 代替 service.NewAdminUseCase：
		// 它在构造管理员服务的同时执行一次配置引导（要把用户名查库解析成 userID）。
		provideAdminUseCase,

		handler.NewHealthHandler,
		handler.NewAuthHandler,
		handler.NewAdminHandler,
		handler.NewMailHandler,

		handler.NewRouter,
	)
	return &handler.Router{}, nil, nil
}
