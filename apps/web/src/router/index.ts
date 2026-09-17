import { createRouter, createWebHistory, type RouteMeta } from "vue-router";
import { resolveRouteAccess } from "./guards";
import { routes } from "./routes";

const router = createRouter({
  history: createWebHistory(),
  routes,
});

/** 路由 meta 里声明的允许角色清单；未声明即公开路由 */
function allowRolesOf(meta: RouteMeta): readonly string[] {
  const { allowRoles } = meta;
  return Array.isArray(allowRoles) ? (allowRoles as string[]) : [];
}

/**
 * 全局守卫：按 meta.allowRoles 判断能否进入目标路由。
 *
 * 判定逻辑在 router/guards.ts（纯函数、有单测）；这里只做接线。
 * 注意返回值：Vue Router 的 NavigationGuardReturn 不接受 null，
 * 放行必须显式返回 true，所以把 resolveRouteAccess 的 null 映射成 true。
 */
router.beforeEach(async (to) => {
  const redirect = await resolveRouteAccess(allowRolesOf(to.meta), to.fullPath);
  return redirect ?? true;
});

export default router;
