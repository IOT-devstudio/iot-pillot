export interface LoginForm {
  username: string;
  password: string;
}

export interface RegisterForm {
  name: string;
  email: string;
  password: string;
  confirmPassword: string;
  code: string;
}

export interface RegisterPayload {
  name: string;
  password: string;
  email: string;
  code: string;
}

export interface LoginPayload {
  username: string;
  password: string;
}

export type FieldErrors<T> = Partial<Record<keyof T, string>>;

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const SIX_DIGIT_CODE_PATTERN = /^\d{6}$/;

export function validateLoginForm(form: LoginForm): FieldErrors<LoginForm> {
  const errors: FieldErrors<LoginForm> = {};
  const username = form.username.trim();
  const password = form.password.trim();

  if (!username) {
    errors.username = "请输入用户名";
  }

  if (!password) {
    errors.password = "请输入密码";
  } else if (form.password.length < 6) {
    errors.password = "密码至少 6 位";
  } else if (form.password.length > 20) {
    errors.password = "密码最多 20 位";
  }

  return errors;
}

export function validateRegisterForm(
  form: RegisterForm,
): FieldErrors<RegisterForm> {
  const errors: FieldErrors<RegisterForm> = {};
  const name = form.name.trim();
  const email = form.email.trim();
  const password = form.password.trim();
  const confirmPassword = form.confirmPassword.trim();
  const code = form.code.trim();

  if (!name) {
    errors.name = "请输入姓名";
  } else if (name.length < 3 || name.length > 20) {
    errors.name = "姓名长度为 3-20 个字符";
  }

  if (!email) {
    errors.email = "请输入邮箱";
  } else if (email.length > 50 || !EMAIL_PATTERN.test(email)) {
    errors.email = "请输入正确的邮箱地址";
  }

  if (!password) {
    errors.password = "请输入密码";
  } else if (form.password.length < 6) {
    errors.password = "密码至少 6 位";
  } else if (form.password.length > 20) {
    errors.password = "密码最多 20 位";
  }

  if (!confirmPassword) {
    errors.confirmPassword = "请再次输入密码";
  } else if (form.confirmPassword !== form.password) {
    errors.confirmPassword = "两次输入的密码不一致";
  }

  if (!SIX_DIGIT_CODE_PATTERN.test(code)) {
    errors.code = "请输入 6 位验证码";
  }

  return errors;
}

export function toLoginPayload(form: LoginForm): LoginPayload {
  return { username: form.username.trim(), password: form.password };
}

export function toRegisterPayload(form: RegisterForm): RegisterPayload {
  return {
    name: form.name.trim(),
    email: form.email.trim(),
    password: form.password,
    code: form.code,
  };
}
