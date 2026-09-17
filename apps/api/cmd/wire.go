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
		// AuthConfig 是 *Config 的字段，没有独立的 provider：
		// 用 FieldsOf 把 *config.AuthConfig 暴露出来给 AuthUseCase 注入。
		// 注意参数必须是 new(*config.Config)：FieldsOf 取的是「该指针类型所指结构体」
		// 的字段，写 new(config.Config) 会要求一个 config.Config **值** provider，
		// 而 config.Load 只提供 *config.Config。
		wire.FieldsOf(new(*config.Config), "AUTH"),
		repository.ProvideDB,
		utils.ConnectRedis,

		utils.NewCodeManager,
		utils.NewMailManager,
		utils.NewTokenManager,

		repository.NewUserRepo,

		service.NewAuthUseCase,

		handler.NewHealthHandler,
		handler.NewAuthHandler,

		handler.NewRouter,
	)
	return &handler.Router{}, nil, nil
}
