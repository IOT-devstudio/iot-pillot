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
  AUTH_CODE_SESSION_INVALIDATED,
  AuthRequestError,
} from "@/api/auth";
import { fetchCurrentUser, type UserRole } from "@/api/admin";
import { clearAuthSession, readAuthSession } from "@/auth/session";

/**
 * 需要重定向时的目标位置。
 *
 * 只用固定路径定位：登录入口是二维认证页所在的 "/"，角色落点也是固定路径。
 */
export interface RouteRedirect {
  path: string;
  query?: Record<string, string>;
}

export interface RouteGateDeps {
  /** 读取本地会话里的 access token；无会话返回 null */
  readToken: () => string | null;
  /** 向服务端查询当前令牌对应的身份（角色 + 创始标记，同一份 /me 响应） */
  fetchIdentity: (accessToken: string) => Promise<RouteIdentity>;
  /** 令牌确定失效时清理本地会话 */
  clearSession: () => void;
}

/** 守卫判定需要的最小身份切片，来自 GET /me */
export interface RouteIdentity {
  role: string;
  /** 创始管理员（issue #120）；后端字段缺省时按 false 处理 */
  isFounding: boolean;
}

const defaultDeps: RouteGateDeps = {
  readToken: () => readAuthSession()?.access_token ?? null,
  fetchIdentity: async () => {
    const user = await fetchCurrentUser();
    return { role: user.role, isFounding: user.is_founding === true };
  },
  clearSession: () => clearAuthSession(),
};

/**
 * 登录入口。
 *
 * 根路径和 /login 复用二维认证组件；守卫继续使用既有根路径与 redirect。
 */
const LOGIN_ROUTE = "/";

/** 去登录入口并记住原目标，便于登录后跳回；reason 会转成登录页的提示条 */
function toLogin(
  fullPath: string,
  reason?: "expired" | "invalidated",
): RouteRedirect {
  const query: Record<string, string> = { redirect: fullPath };
  if (reason !== undefined) {
    query.reason = reason;
  }
  return { path: LOGIN_ROUTE, query };
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
 * @param denyFounding 创始管理员（ADMIN_USERS 种入）禁止进入（issue #138）。
 *   命中时重定向到用户首页并带 ?notice= 提示；只在角色放行后判定，
 *   公开路由不受影响。后端 is_founding 缺省时（#120 未合并）恒为 false，静默放行。
 * @returns null 表示放行；否则返回要重定向到的路由位置。
 *
 * 判定规则：
 *   - 公开路由（allowRoles 为空）：直接放行，**不查服务端**
 *     （避免每次导航都多一个 /me 请求）
 *   - 没有本地会话：去登录页，并记下原目标便于登录后跳回
 *   - 角色在清单内：放行
 *   - 角色不在清单内：跳该角色自己的落点
 *   - 角色放行但 denyFounding 命中创始管理员：回用户首页 + notice
 *   - 服务端说令牌失效（401/403）：清掉本地会话再去登录页
 *   - 其他失败（网络异常等）：**不清会话**，只按"无法确认"处理，去登录页
 *     —— 网络抖动不该把用户的会话删掉
 */
export async function resolveRouteAccess(
  allowRoles: readonly string[],
  fullPath: string,
  deps: RouteGateDeps = defaultDeps,
  denyFounding = false,
): Promise<RouteRedirect | null> {
  if (allowRoles.length === 0) {
    return null;
  }

  const token = deps.readToken();
  if (token === null) {
    return toLogin(fullPath);
  }

  let identity: RouteIdentity;
  try {
    identity = await deps.fetchIdentity(token);
  } catch (error: unknown) {
    if (error instanceof AuthRequestError && isTokenRejected(error.status)) {
      // 令牌确实无效：清掉会话，否则会带着一个坏令牌反复重试。
      // 401 带上失效原因（issue #84 契约：4011=被顶替/改密/登出），
      // 登录页按 ?reason= 提示；403 是权限问题不是会话结束，不带。
      deps.clearSession();
      if (error.status === 401) {
        return toLogin(
          fullPath,
          error.businessCode === AUTH_CODE_SESSION_INVALIDATED
            ? "invalidated"
            : "expired",
        );
      }
    }
    return toLogin(fullPath);
  }

  const { role, isFounding } = identity;
  if (!allowRoles.includes(role)) {
    return { path: landingFor(role) };
  }

  // 创始管理员没有个人设置页（issue #138，维护者拍板）：
  // 该账号无绑定邮箱、不走自助改密，个人设置表单对其语义不成立。
  // 重定向回用户首页并带 notice，页面给出解释而不是无声弹回。
  if (denyFounding && isFounding) {
    return { path: USER_HOME, query: { notice: FOUNDING_SETTINGS_BLOCKED } };
  }

  return null;
}

/** 创始管理员被拦出个人设置页时的 notice 标识；首页按此出提示文案 */
export const FOUNDING_SETTINGS_BLOCKED = "founding-settings";

/** 401 未认证 / 403 无权限都说明当前令牌不该继续用来访问受保护资源 */
function isTokenRejected(status: number): boolean {
  return status === 401 || status === 403;
}
