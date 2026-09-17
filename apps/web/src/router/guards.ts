/**
 * 路由守卫：管理端路由的准入判断。
 *
 * 为什么单独成文件：这段逻辑决定了"谁能进管理端"，是本项目第一条真正的授权
 * 分支，必须能被测到。所以把判定抽成纯函数，依赖（读令牌、问角色）通过参数注入，
 * 测试里不需要真的起 router，也不需要网络。
 *
 * 注意分工：这里只是**前端可见性**控制。真正的强制在服务端的
 * RequireRole 中间件（apps/api/internal/middleware/auth.go），
 * 绕过前端直接请求管理端接口依然会拿到 403。
 */
import { AuthRequestError, fetchCurrentUser } from "@/api/auth";
import { clearAuthSession, readAuthSession } from "@/auth/session";

/** 需要重定向时的目标位置 */
export interface AdminRedirect {
  name: string;
  query?: Record<string, string>;
}

export interface AdminGateDeps {
  /** 读取本地会话里的 access token；无会话返回 null */
  readToken: () => string | null;
  /** 向服务端查询当前令牌对应的角色 */
  fetchRole: (accessToken: string) => Promise<string>;
  /** 令牌确定失效时清理本地会话 */
  clearSession: () => void;
}

const defaultDeps: AdminGateDeps = {
  readToken: () => readAuthSession()?.access_token ?? null,
  fetchRole: async (accessToken: string) =>
    (await fetchCurrentUser(accessToken)).role,
  clearSession: () => clearAuthSession(),
};

/** 管理端准入失败时的落点。M3 会有专门的 403 页，现在先挡回控制台 */
const DENIED_ROUTE = "dashboard";
const LOGIN_ROUTE = "login";

/**
 * 判断能否进入目标路由。
 *
 * @returns null 表示放行；否则返回要重定向到的路由位置。
 *
 * 判定规则：
 *   - 不需要 admin 的路由：直接放行，**不查服务端**（避免每次导航都多一个请求）
 *   - 没有本地会话：去登录页，并记下原目标便于登录后跳回
 *   - 服务端说令牌失效（401/403）：清掉本地会话再去登录页
 *   - 其他失败（网络异常等）：**不清会话**，只按"无法确认"处理，去登录页
 *     —— 网络抖动不该把用户的会话删掉
 *   - 角色不是 admin：挡回控制台
 */
export async function resolveAdminAccess(
  requiresAdmin: boolean,
  fullPath: string,
  deps: AdminGateDeps = defaultDeps,
): Promise<AdminRedirect | null> {
  if (!requiresAdmin) {
    return null;
  }

  const token = deps.readToken();
  if (token === null) {
    return { name: LOGIN_ROUTE, query: { redirect: fullPath } };
  }

  let role: string;
  try {
    role = await deps.fetchRole(token);
  } catch (error: unknown) {
    if (error instanceof AuthRequestError && isTokenRejected(error.status)) {
      // 令牌确实无效：清掉会话，否则会带着一个坏令牌反复重试
      deps.clearSession();
    }
    return { name: LOGIN_ROUTE, query: { redirect: fullPath } };
  }

  if (role !== "admin") {
    return { name: DENIED_ROUTE };
  }

  return null;
}

/** 401 未认证 / 403 无权限都说明当前令牌不该继续用来访问管理端 */
function isTokenRejected(status: number): boolean {
  return status === 401 || status === 403;
}
