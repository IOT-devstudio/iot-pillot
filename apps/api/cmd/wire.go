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

		repository.NewUserRepo,

		service.NewAuthUseCase,

		handler.NewHealthHandler,
		handler.NewAuthHandler,

		handler.NewRouter,
	)
	return &handler.Router{}, nil, nil
}
