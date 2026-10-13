/**
 * 返回链接的角色分流逻辑（issue #94）。
 *
 * 从 BackToDashboard.vue 抽成纯函数：组件里只剩接线，目标与文案的
 * 选择规则在这里被单独测到，不必起渲染器。
 *
 * 口径与 router/guards.ts 的 landingFor 一致：
 *   - 管理员 → 控制台 /dashboard
 *   - 普通成员 / 角色未知 → 个人首页 /user/home
 */
import { landingFor } from "@/router/guards";

/** 返回目标：显式传了 to 就用 to，否则按角色落点 */
export function backLinkTarget(
  to: string | undefined,
  role: string | null,
): string {
  return to ?? landingFor(role ?? "member");
}

/** 文案：显式传了 label 就用 label，否则跟随目标 */
export function backLinkLabel(
  to: string | undefined,
  label: string | undefined,
  role: string | null,
): string {
  if (label !== undefined) {
    return label;
  }
  return backLinkTarget(to, role) === "/dashboard"
    ? "← 返回控制台"
    : "← 返回个人首页";
}
