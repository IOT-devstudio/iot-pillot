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

export type AuthMode = "login" | "register" | "reset";

/**
 * 会直接产生登录会话的两种模式。
 * reset（找回密码，issue #85）成功后不发令牌，走独立提交路径，
 * submitAuth 只接受这两种——把 reset 传进来会误走注册分支。
 */
export type CredentialMode = Extract<AuthMode, "login" | "register">;
export type AuthForm = LoginForm | RegisterForm;

export interface AuthSubmitDeps {
  login: (payload: LoginPayload) => Promise<AuthSession>;
  register: (payload: RegisterPayload) => Promise<AuthSession>;
  saveSession: (session: AuthSession) => void;
  checkProfile?: () => Promise<{ user_id: number; role: string } | void>;
  navigate: (path: string) => Promise<unknown> | unknown;
}

export type AuthSubmitResult =
  | { ok: true; session: AuthSession }
  | { ok: false; errors: Record<string, string> };

/**
 * 登录成功后的默认落点。
 *
 * 默认进入受保护的管理台，由守卫按角色分流。
 * 访客首页与 About 页是公开路由，守卫不会查询身份；管理员从这些入口
 * 登录时，复用登录后的资料检查结果将其送到管理台。具体业务链接仍保留。
 */
export const DEFAULT_LANDING = "/dashboard";

/**
 * 冻结账号专属落脚页（0012 自助解冻）。
 *
 * 与「资料不完整提示」不同：那个是横幅提示、可以忽略；
 * 这个是真的进不去别的页面，只能补资料。
 */
export const PROFILE_UNFREEZE_PATH = "/user/settings?reason=frozen";

export function defaultNavigateTarget(redirect?: string, role?: string): string {
  // 只接受站内绝对路径：redirect 可能来自 URL，放行 "//evil.com" 这类
  // 协议相对地址会让登录变成一个开放重定向漏洞。
  if (redirect && redirect.startsWith("/") && !redirect.startsWith("//")) {
    const pathname = redirect.split(/[?#]/, 1)[0].replace(/\/+$/, "");
    if (role === "admin" && ["/user/home", "/user/about"].includes(pathname)) {
      return DEFAULT_LANDING;
    }
    return redirect;
  }

  return DEFAULT_LANDING;
}

export async function submitAuth(
  mode: CredentialMode,
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

  // 受限会话（0012 自助解冻）：账号因资料不全被冻结，令牌只能用来补资料。
  // 必须直接进设置页，**忽略 redirect**——原来的目标页几乎一定访问不了，
  // 把人带过去只会看到一片 403。
  if (session.scope === "profile_only" || session.frozen) {
    await deps.navigate(PROFILE_UNFREEZE_PATH);
    return { ok: true, session };
  }

  const profile = await deps.checkProfile?.();
  const role = profile?.user_id === session.user_id ? profile?.role : undefined;
  await deps.navigate(defaultNavigateTarget(redirect, role));

  return { ok: true, session };
}
