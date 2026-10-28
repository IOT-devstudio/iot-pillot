package handler

import (
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
)

// TestNewRouterDoesNotPanic 构造一次完整路由表。
//
// gin 在注册阶段就会对「同方法下通配符与静态段冲突」直接 panic，而这类冲突
// 只有在真正构造路由时才会暴露——没有任何单测覆盖的话，一次路由新增就可能
// 让服务在启动时崩掉。这里用 nil 依赖构造：handler 只在处理请求时才解引用服务，
// 因此构造阶段不需要 DB / Redis / SMTP。
func TestNewRouterDoesNotPanic(t *testing.T) {
	cfg := &config.Config{
		SERVICE: &config.ServiceConfig{Port: 0, Mode: "test"},
		CORS:    &config.CORSConfig{},
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("路由注册 panic（多为通配符冲突）: %v", recovered)
		}
	}()

	router := NewRouter(
		cfg,
		nil, // health
		nil, // auth
		nil, // admin
		nil, // mail
		nil, // recruitment
		nil, // invitation
		nil, // form
		nil, // studio member（#195）
		nil, // studio knowledge（#196）
		nil, // studio project（#198）
		nil, // freeze scheduler：允许为空（Run 里有 nil 判断）
		nil, // token manager
		nil, // admin store
	)
	if router == nil || router.eng == nil {
		t.Fatal("路由未构造成功")
	}

	// 解冻路由（0012）必须真的挂上：gin 的 Routes() 是注册结果的唯一真相。
	var found bool
	for _, route := range router.eng.Routes() {
		if route.Method == "POST" && route.Path == "/api/v1/admin/users/:user_id/unfreeze" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("未注册 POST /api/v1/admin/users/:user_id/unfreeze")
	}

	// 工作室成员（#195）五条端点必须真的挂上，且都在 admin 组内。
	wantStudioRoutes := map[string]bool{
		"GET /api/v1/admin/studio/members":              false,
		"POST /api/v1/admin/studio/members":             false,
		"GET /api/v1/admin/studio/members/:id":          false,
		"PUT /api/v1/admin/studio/members/:id":          false,
		"POST /api/v1/admin/studio/members/:id/archive": false,
		// hired 转成员（#197）：静态段 from-candidate 与 :id 同层共存，
		// 两条都必须注册上，否则请求会打到错的 handler 上。
		"POST /api/v1/admin/studio/members/from-candidate": false,
	}
	for _, route := range router.eng.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantStudioRoutes[key]; ok {
			wantStudioRoutes[key] = true
		}
	}
	for key, registered := range wantStudioRoutes {
		if !registered {
			t.Errorf("未注册 %s", key)
		}
	}

	// 知识库（#196）七条端点必须真的挂上，且都在 admin 组内。
	//
	// 特别锁住 revisions 两条：gin 里 /knowledge/:id 与 /knowledge/:id/revisions
	// 同层共存是合法的，但顺序写错会让 :id 吞掉 revisions 段——
	// 那种错误只在请求时才暴露，这里提前拦住。
	wantKnowledgeRoutes := map[string]bool{
		"GET /api/v1/admin/studio/knowledge":                        false,
		"POST /api/v1/admin/studio/knowledge":                       false,
		"GET /api/v1/admin/studio/knowledge/:id":                    false,
		"PUT /api/v1/admin/studio/knowledge/:id":                    false,
		"DELETE /api/v1/admin/studio/knowledge/:id":                 false,
		"GET /api/v1/admin/studio/knowledge/:id/revisions":          false,
		"GET /api/v1/admin/studio/knowledge/:id/revisions/:version": false,
	}
	for _, route := range router.eng.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantKnowledgeRoutes[key]; ok {
			wantKnowledgeRoutes[key] = true
		}
	}
	for key, registered := range wantKnowledgeRoutes {
		if !registered {
			t.Errorf("未注册 %s", key)
		}
	}

	// 项目与参与（#198）九条端点必须真的挂上。
	//
	// 特别锁住 /projects/:id 与 /projects/:id/members/:member_id 这两组
	// 同层共存：顺序写错会让 :id 吞掉 members 段，那种错误只在请求时暴露。
	wantProjectRoutes := map[string]bool{
		"GET /api/v1/admin/studio/projects":                           false,
		"POST /api/v1/admin/studio/projects":                          false,
		"GET /api/v1/admin/studio/projects/:id":                       false,
		"PUT /api/v1/admin/studio/projects/:id":                       false,
		"DELETE /api/v1/admin/studio/projects/:id":                    false,
		"GET /api/v1/admin/studio/projects/:id/members":               false,
		"POST /api/v1/admin/studio/projects/:id/members":              false,
		"PUT /api/v1/admin/studio/projects/:id/members/:member_id":    false,
		"DELETE /api/v1/admin/studio/projects/:id/members/:member_id": false,
	}
	for _, route := range router.eng.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantProjectRoutes[key]; ok {
			wantProjectRoutes[key] = true
		}
	}
	for key, registered := range wantProjectRoutes {
		if !registered {
			t.Errorf("未注册 %s", key)
		}
	}
}
