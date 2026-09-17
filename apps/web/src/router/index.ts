import { createRouter, createWebHistory } from "vue-router";
import { resolveAdminAccess } from "./guards";
import { routes } from "./routes";

const router = createRouter({
  history: createWebHistory(),
  routes,
});

/**
 * 全局守卫：只有标记了 meta.requiresAdmin 的路由才会去服务端确认角色。
 *
 * 判定逻辑在 router/guards.ts（纯函数、有单测）；这里只做接线。
 * 注意返回值：Vue Router 的 NavigationGuardReturn 不接受 null，
 * 放行必须显式返回 true，所以把 resolveAdminAccess 的 null 映射成 true。
 */
router.beforeEach(async (to) => {
  const redirect = await resolveAdminAccess(
    Boolean(to.meta.requiresAdmin),
    to.fullPath,
  );
  return redirect ?? true;
});

export default router;
