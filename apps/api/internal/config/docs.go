package config

// DocsConfig 接口文档（Swagger UI + OpenAPI spec）的开关。
//
// 为什么不直接复用 SERVICE.Mode 判断：文档会把整个接口面暴露出来。
// 将来要单独给联调环境开着、或生产临时关掉，都不该被迫改运行模式。
type DocsConfig struct {
	// Enabled 是否注册 /docs（Swagger UI）与 /openapi.json（spec）。
	// 默认值跟随 mode：debug 开、release 关。
	// 显式写 docs.enabled 或设 IOT_PILOT_DOCS_ENABLED 时以显式值为准。
	Enabled bool
}
