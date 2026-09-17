/**
 * 路由守卫：按角色清单判断能否进入目标路由。
 *
 * 为什么单独成文件：这段逻辑决定了"谁能进哪个页面"，是本项目第一条真正的授权
 * 分支，必须能被测到。所以把判定抽成纯函数，依赖（读令牌、问角色）通过参数注入，
 * 测试里不需要真的起 router，也不需要网络。
 *
 * 注意分工：这里只是**前端可见性**控制。真正的强制在服务端的
 * RequireRole 中间件（apps/api/internal/middleware/auth.go），
 * 绕过前端直接请求管理端接口依然会拿到 403。
 */
import {
  AuthRequestError,
  type UserRole,
} from "@/api/auth";
import { clearAuthSession, readAuthSession } from "@/auth/session";
import { fetchCurrentUserWithRefresh } from "@/auth/current-user";

/**
 * 需要重定向时的目标位置。
 *
 * 只用固定路径定位：登录入口是 3D 开屏所在的 "/"，角色落点也是固定路径。
 */
export interface RouteRedirect {
  path: string;
  query?: Record<string, string>;
}

export interface RouteGateDeps {
  /** 读取本地会话里的 access token；无会话返回 null */
  readToken: () => string | null;
  /** 向服务端查询当前令牌对应的角色 */
  fetchRole: (accessToken: string) => Promise<string>;
  /** 令牌确定失效时清理本地会话 */
  clearSession: () => void;
}

const defaultDeps: RouteGateDeps = {
  readToken: () => readAuthSession()?.access_token ?? null,
  fetchRole: async (accessToken: string) =>
    (await fetchCurrentUserWithRefresh(accessToken)).role,
  clearSession: () => clearAuthSession(),
};

/**
 * 登录入口。
 *
 * 指向 3D 开屏（根路径）而不是二维的 /login：后者只在 WebGL 渲染失败时
 * 由开屏自己转过去，正常路径永远到不了，这样「只有一个登录界面」才成立。
 */
const LOGIN_ROUTE = "/";

/** 去登录入口并记住原目标，便于登录后跳回 */
function toLogin(fullPath: string): RouteRedirect {
  return { path: LOGIN_ROUTE, query: { redirect: fullPath } };
}

/**
 * 各角色的登录落点。
 *
 * 越权时跳访问者**自己的**落点，而不是固定的拒绝页：普通用户被挡出成员页后
 * 会落在自己的首页，不会出现「被跳到自己也进不去的地方」这种死循环。
 *
 * 未知角色一律按普通用户处理——服务端将来新增角色时，默认给最小可见范围，
 * 而不是让用户卡在登录页。
 */
export function landingFor(role: UserRole | string): string {
  return role === "admin" ? "/dashboard" : USER_HOME;
}

/** 用户侧首页，也是非成员角色的统一落点 */
const USER_HOME = "/user/home";

/**
 * 判断能否进入目标路由。
 *
 * @param allowRoles 允许访问的角色清单；**空数组表示公开路由**
 * @returns null 表示放行；否则返回要重定向到的路由位置。
 *
 * 判定规则：
 *   - 公开路由（allowRoles 为空）：直接放行，**不查服务端**
 *     （避免每次导航都多一个 /me 请求）
 *   - 没有本地会话：去登录页，并记下原目标便于登录后跳回
 *   - 角色在清单内：放行
 *   - 角色不在清单内：跳该角色自己的落点
 *   - 服务端说令牌失效（401/403）：清掉本地会话再去登录页
 *   - 其他失败（网络异常等）：**不清会话**，只按"无法确认"处理，去登录页
 *     —— 网络抖动不该把用户的会话删掉
 */
export async function resolveRouteAccess(
  allowRoles: readonly string[],
  fullPath: string,
  deps: RouteGateDeps = defaultDeps,
): Promise<RouteRedirect | null> {
  if (allowRoles.length === 0) {
    return null;
  }

  const token = deps.readToken();
  if (token === null) {
    return toLogin(fullPath);
  }

  let role: string;
  try {
    role = await deps.fetchRole(token);
  } catch (error: unknown) {
    if (error instanceof AuthRequestError && isTokenRejected(error.status)) {
      // 令牌确实无效：清掉会话，否则会带着一个坏令牌反复重试
      deps.clearSession();
    }
    return toLogin(fullPath);
  }

  if (!allowRoles.includes(role)) {
    return { path: landingFor(role) };
  }

  return null;
}

/** 401 未认证 / 403 无权限都说明当前令牌不该继续用来访问受保护资源 */
function isTokenRejected(status: number): boolean {
  return status === 401 || status === 403;
}
