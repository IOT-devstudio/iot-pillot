/**
 * opener-editor 模块的路由。
 *
 * meta.requiresAdmin 会被 router/guards.ts 读取：进入前先向服务端确认角色。
 * 注意这只是前端可见性控制，绕过它直接调用管理端接口依然会拿到 403
 * （服务端 RequireRole 中间件）。
 */
import type { RouteRecordRaw } from "vue-router";

import BuildingsEditor from "./views/BuildingsEditor.vue";

export const openerEditorRoutes: RouteRecordRaw[] = [
  {
    path: "/admin/buildings",
    name: "admin-buildings",
    component: BuildingsEditor,
    meta: { title: "楼栋布局编辑器", requiresAdmin: true },
  },
];
