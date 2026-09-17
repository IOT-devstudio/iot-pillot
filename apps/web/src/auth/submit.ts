import type { AuthSession } from "@/api/auth";
import {
  type LoginForm,
  type LoginPayload,
  type RegisterForm,
  type RegisterPayload,
  toLoginPayload,
  toRegisterPayload,
  validateLoginForm,
  validateRegisterForm,
} from "@/auth/form";

export type AuthMode = "login" | "register";
export type AuthForm = LoginForm | RegisterForm;

export interface AuthSubmitDeps {
  login: (payload: LoginPayload) => Promise<AuthSession>;
  register: (payload: RegisterPayload) => Promise<AuthSession>;
  saveSession: (session: AuthSession) => void;
  navigate: (path: string) => Promise<unknown> | unknown;
}

export type AuthSubmitResult =
  | { ok: true; session: AuthSession }
  | { ok: false; errors: Record<string, string> };

/**
 * 登录成功后的默认落点。
 *
 * 这里**只做最基础的跳转**，不按角色分流：目标页由路由守卫按 meta.allowRoles
 * 把关，普通用户走到这里会被守卫带到自己的页面。
 *
 * 为什么不在这里查角色再决定：
 *   1. 守卫每次导航都必须查角色，这里再查一次就是重复请求；
 *   2. 分流逻辑集中在一处（guards.ts），比散落在登录流程里更难写错。
 *
 * Vue Router 的异步守卫在目标组件渲染前完成，用户看不到中间跳转。
 */
export const DEFAULT_LANDING = "/dashboard";

export function defaultNavigateTarget(redirect?: string): string {
  // 只接受站内绝对路径：redirect 可能来自 URL，放行 "//evil.com" 这类
  // 协议相对地址会让登录变成一个开放重定向漏洞。
  if (redirect && redirect.startsWith("/") && !redirect.startsWith("//")) {
    return redirect;
  }

  return DEFAULT_LANDING;
}

export async function submitAuth(
  mode: AuthMode,
  form: AuthForm,
  deps: AuthSubmitDeps,
  redirect?: string,
): Promise<AuthSubmitResult> {
  const errors =
    mode === "login"
      ? validateLoginForm(form as LoginForm)
      : validateRegisterForm(form as RegisterForm);

  if (Object.keys(errors).length > 0) {
    return { ok: false, errors };
  }

  const session =
    mode === "login"
      ? await deps.login(toLoginPayload(form as LoginForm))
      : await deps.register(toRegisterPayload(form as RegisterForm));

  deps.saveSession(session);
  await deps.navigate(defaultNavigateTarget(redirect));

  return { ok: true, session };
}
