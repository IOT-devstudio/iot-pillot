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

/** 找回密码表单（issue #85 / FE-01）：邮箱 + 验证码 + 新密码 */
export interface ResetForm {
  email: string;
  password: string;
  confirmPassword: string;
  code: string;
}

export interface ResetPayload {
  email: string;
  code: string;
  new_password: string;
}

/** 修改密码表单（issue #85 / FE-01）：登录态下换密码 */
export interface ChangePasswordForm {
  oldPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export interface ChangePasswordPayload {
  old_password: string;
  new_password: string;
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
  } else if (name.length < 2 || name.length > 20) {
    errors.name = "姓名长度为 2-20 个字符";
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

/** 密码字段的通用校验：与注册/登录同一套 6–20 位口径 */
function passwordError(password: string): string | undefined {
  if (!password) {
    return "请输入密码";
  }
  if (password.length < 6) {
    return "密码至少 6 位";
  }
  if (password.length > 20) {
    return "密码最多 20 位";
  }
  return undefined;
}

export function validateResetForm(form: ResetForm): FieldErrors<ResetForm> {
  const errors: FieldErrors<ResetForm> = {};
  const email = form.email.trim();

  if (!email) {
    errors.email = "请输入邮箱";
  } else if (email.length > 50 || !EMAIL_PATTERN.test(email)) {
    errors.email = "请输入正确的邮箱地址";
  }

  const passwordErrorText = passwordError(form.password);
  if (passwordErrorText) {
    errors.password = passwordErrorText;
  }

  if (!form.confirmPassword) {
    errors.confirmPassword = "请再次输入密码";
  } else if (form.confirmPassword !== form.password) {
    errors.confirmPassword = "两次输入的密码不一致";
  }

  if (!SIX_DIGIT_CODE_PATTERN.test(form.code.trim())) {
    errors.code = "请输入 6 位验证码";
  }

  return errors;
}

export function toResetPayload(form: ResetForm): ResetPayload {
  return {
    email: form.email.trim(),
    code: form.code.trim(),
    new_password: form.password,
  };
}

export function validateChangePasswordForm(
  form: ChangePasswordForm,
): FieldErrors<ChangePasswordForm> {
  const errors: FieldErrors<ChangePasswordForm> = {};

  if (!form.oldPassword) {
    errors.oldPassword = "请输入当前密码";
  }

  const newPasswordError = passwordError(form.newPassword);
  if (newPasswordError) {
    errors.newPassword = newPasswordError.replace("密码", "新密码");
  } else if (form.newPassword === form.oldPassword) {
    errors.newPassword = "新密码不能与当前密码相同";
  }

  if (!form.confirmPassword) {
    errors.confirmPassword = "请再次输入新密码";
  } else if (form.confirmPassword !== form.newPassword) {
    errors.confirmPassword = "两次输入的密码不一致";
  }

  return errors;
}

export function toChangePasswordPayload(
  form: ChangePasswordForm,
): ChangePasswordPayload {
  return {
    old_password: form.oldPassword,
    new_password: form.newPassword,
  };
}
