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

// 依赖装配图（生成物是 wire_gen.go：`wire gen ./cmd`，或 go generate ./cmd）。
//
// 关于接口：wire 按**类型**连线，Go 的隐式接口满足对它无效 —— provider 产出
// *utils.AdminStore 时它不会当成 utils.AdminDirectory。原来靠一串
// wire.Bind(new(接口), new(*实现)) 解决，现在改成"返回值就是接口"的 NewXxx，
// 每个都写在接口声明的旁边（utils/admin_store.go、service/auth_usec.go、
// handler/health_handle.go …）。本文件里因此既没有 new()，也没有 cmd 侧的胶水 provider。
//
// 返回 *service.FreezeScheduler 会让装配变得别扭（wire 的 injector 只接受
// 单个主返回值），而且定时任务本来就该跟 HTTP 服务同生共死——因此它作为
// NewRouter 的依赖注入，由 Router.Run 在服务启动/退出时 Start/Stop。
func InitializeApp() (*handler.Router, func(), error) {
	wire.Build(
		config.Load,
		repository.ProvideDB,
		utils.ConnectRedis,

		utils.NewCodeManager,
		utils.NewMailManager,
		utils.NewTokenManager,
		utils.NewAdminStore,
		utils.NewRedisCounter,
		utils.NewLimiter,

		repository.NewDatabasePinger,
		utils.NewRedisPinger,

		utils.NewAdminDirectory,
		utils.NewCounter,
		service.NewTokenIssuer,
		service.NewVerifyCodeChecker,
		service.NewMailSender,
		service.NewSessionRevoker,
		service.NewAccountMailer,
		handler.NewDatabasePinger,
		handler.NewRedisPinger,

		repository.NewUserRepo,
		repository.NewMailModelRepo,
		repository.NewMailRepo,
		repository.NewRecruitmentRepo,
		repository.NewInvitationRepo,
		repository.NewCandidateRepo,
		repository.NewFormRepo,

		service.NewAuthUseCase,
		service.NewMailUseCase,
		service.NewAdminUseCase,
		service.NewRecruitmentUseCase,
		service.NewApplicationStore,
		service.NewBusinessMailer,
		service.NewRecruitmentInvitationUseCase,
		service.NewApplicationCreator,
		service.NewRecruitmentDirectory,
		service.NewFormUseCase,

		// 资料不合规账号的定时冻结（0012）
		service.NewFreezeMailer,
		service.NewFreezeUseCase,
		service.NewFreezeScheduler,

		handler.NewHealthHandler,
		handler.NewAuthHandler,
		handler.NewAdminHandler,
		handler.NewMailHandler,
		handler.NewRecruitmentHandler,
		handler.NewRecruitmentInvitationHandler,
		handler.NewFormHandler,

		handler.NewRouter,
	)
	return &handler.Router{}, nil, nil
}
