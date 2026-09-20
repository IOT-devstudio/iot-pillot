// Package main 是 iot-pillot 后端 API 服务的入口。
package main

import (
	"log"
)

// 接口文档的生成指令：`cd apps/api && go generate ./cmd` 重新生成 apps/api/docs。
//
// 参数为什么这么写：go generate 的工作目录是**本包目录**（apps/api/cmd），
// 因此 --dir 指向上一级（apps/api，模块根 —— swag 靠它找到 handler、DTO 与 go.mod），
// -g 相对 --dir，-o 相对当前目录；--parseInternal 是必需的，因为 handler 与 DTO 都在
// internal/ 下。@v1.16.6 固定 CLI 版本，避免"谁生成的"不一致；
// 生成物（docs/docs.go、swagger.json、swagger.yaml）随源码入库 —— CI 与镜像只跑
// go build ./cmd，不会去装 swag CLI。
//
// --templateDelims 不是可选项：swag 把注解塞进一个 Go text/template，而本项目的
// 邮件模块注解里就有双花括号占位符（模板变量语法）。默认分隔符下那会被当成模板动作、
// 解析失败，ReadDoc() 于是原样吐出**未渲染**的模板（文档标题会变成模板占位符本身）。
// 换成 [[ ]] 之后，注解里的花括号只是普通文本。
//
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --dir .. -g cmd/main.go -o ../docs --parseInternal --templateDelims "[[,]]"

// @title           iot-pillot API
// @version         0.1.0
// @description     IoT 全栈开发工作室的招新管理后台 API。
// @description     统一响应体是 {code, message, data}；除公开端点（注册/登录/发码/刷新/登出）外都需要 Authorization: Bearer <access_token>。
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     登录得到的 access_token。Swagger UI 的 Authorize 里只填 token 本身，前缀由 UI 补。
//
// 刻意不写 @host：写死 localhost:8080 会让文档在 VPS 上指向本机。
// 留空时 Swagger UI 用当前访问域名（本地与线上都对）。
func main() {
	srv, teardown, err := InitializeApp()
	defer teardown()
	if err != nil {
		log.Fatalf("initialize app: %v", err)
	}
	if err := srv.Run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
