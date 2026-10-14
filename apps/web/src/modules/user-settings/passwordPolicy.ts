/**
 * 修改密码的展示层策略（issue #85 A 档打磨，对照大厂改密交互）。
 *
 * 三块纯函数，单独可测：
 *   - passwordRules：输入时的实时规则清单（不是提交校验，负责「边输边打勾」）
 *   - passwordStrength：强度分档（长度 + 字符种类，纯前端计算）
 *   - mapChangePasswordError：后端整块 message 分流回具体字段
 *
 * 提交时的硬校验仍在 auth/form.ts 的 validateChangePasswordForm（含后端
 * 6-20 位契约），两边规则文案保持一致，避免清单打勾了却提交被拒。
 */

export type RuleState = "pending" | "ok" | "fail";

export interface PasswordRule {
  id: "length" | "distinct" | "confirm";
  label: string;
  state: RuleState;
}

/**
 * 实时规则清单。
 *
 * pending = 还没输入、无从判断；fail = 明确违反；ok = 满足。
 * 旧密码是「必填 + 服务端验证」，不进清单——它不是新密码的规则。
 */
export function passwordRules(
  oldPassword: string,
  newPassword: string,
  confirmPassword: string,
): PasswordRule[] {
  const lengthState: RuleState =
    newPassword.length === 0
      ? "pending"
      : newPassword.length >= 6 && newPassword.length <= 20
        ? "ok"
        : "fail";

  const distinctState: RuleState =
    oldPassword.length === 0 || newPassword.length === 0
      ? "pending"
      : oldPassword === newPassword
        ? "fail"
        : "ok";

  return [
    {
      id: "length",
      label: "6–20 个字符",
      state: lengthState,
    },
    {
      id: "distinct",
      label: "与当前密码不同",
      state: distinctState,
    },
    {
      id: "confirm",
      label: "两次输入一致",
      state:
        confirmPassword.length === 0
          ? "pending"
          : confirmPassword === newPassword
            ? "ok"
            : "fail",
    },
  ];
}

export type StrengthLevel = 1 | 2 | 3;

export interface PasswordStrength {
  level: StrengthLevel;
  label: "弱" | "中" | "强";
}

/**
 * 强度分档：种类 = 小写 / 大写 / 数字 / 符号中出现的类别数。
 *
 *   弱：不足 8 位，或只有一类字符
 *   中：8 位以上且两类
 *   强：8 位以上且三类以上
 *
 * 与后端契约无关的展示提示，不参与提交校验。
 */
export function passwordStrength(password: string): PasswordStrength | null {
  if (password.length === 0) return null;

  const classes =
    Number(/[a-z]/.test(password)) +
    Number(/[A-Z]/.test(password)) +
    Number(/[0-9]/.test(password)) +
    Number(/[^a-zA-Z0-9]/.test(password));

  if (password.length >= 8 && classes >= 3) {
    return { level: 3, label: "强" };
  }
  if (password.length >= 8 && classes >= 2) {
    return { level: 2, label: "中" };
  }
  return { level: 1, label: "弱" };
}

/** 后端文案 → 字段级错误。返回 null 表示留在整块错误区（如网络异常） */
export interface MappedPasswordError {
  field: "oldPassword" | "newPassword";
  message: string;
}

const OLD_PASSWORD_MESSAGES = new Map<string, string>([
  // auth_usec.ErrInvalidCredentials：旧密码校验失败
  ["用户名或密码错误", "当前密码不正确"],
  // auth_usec.ErrAccountLocked：改密也会记录失败次数，锁定提示同样指向旧密码
  [
    "登录失败次数过多，请 15 分钟后再试或重置密码",
    "当前密码多次不正确，账号已临时锁定，请 15 分钟后再试或使用忘记密码",
  ],
]);

const NEW_PASSWORD_MESSAGE = "新密码不能与当前密码相同";

export function mapChangePasswordError(
  message: string,
): MappedPasswordError | null {
  const oldPasswordMessage = OLD_PASSWORD_MESSAGES.get(message);
  if (oldPasswordMessage !== undefined) {
    return { field: "oldPassword", message: oldPasswordMessage };
  }
  if (message === NEW_PASSWORD_MESSAGE) {
    return { field: "newPassword", message };
  }
  return null;
}
