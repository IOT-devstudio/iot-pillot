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

export async function submitAuth(
  mode: AuthMode,
  form: AuthForm,
  deps: AuthSubmitDeps,
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
  await deps.navigate("/dashboard");

  return { ok: true, session };
}
