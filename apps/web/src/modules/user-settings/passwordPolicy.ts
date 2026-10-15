/**
 * 修改密码弹窗的展示层策略（issue #85 重构：液态玻璃弹窗 + 邮箱验证）。
 *
 * 提交流程已改为复用「忘记密码」的后端契约（send-reset-code +
 * reset-password）：验证码由后端真实校验，不需要旧密码字段。
 * 这里只留两块纯函数：
 *   - passwordStrength：强度分档（长度 + 字符种类，纯前端计算）
 *   - mapResetError：后端整块 message 分流回具体字段
 */

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
export interface MappedResetError {
  field: "code" | "email";
  message: string;
}

export function mapResetError(message: string): MappedResetError | null {
  // VerifyCode 的失败文案（错误 / 过期 / 次数超限）都带「验证码」
  if (message.includes("验证码")) {
    return { field: "code", message };
  }
  // 绑定邮箱本该存在；出现「未注册」说明 /me 与账号数据不一致，落到邮箱行
  if (message.includes("尚未注册")) {
    return { field: "email", message };
  }
  return null;
}
