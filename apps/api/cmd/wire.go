//go:build wireinject
// +build wireinject

package main

import (
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/handler"
	"github.com/google/wire"
)

func InitializeApp() (*handler.Router, func(), error) {
	wire.Build(
		config.Load,

		handler.NewHealthHandler,

		handler.NewRouter,
	)
	return &handler.Router{}, nil, nil
}
