/**
 * 修改密码弹窗展示策略的测试（issue #85 重构）。
 */
import { describe, expect, it } from "vitest";

import { mapResetError, passwordStrength } from "./passwordPolicy";

describe("passwordStrength", () => {
  it("returns null for an empty password", () => {
    expect(passwordStrength("")).toBeNull();
  });

  it("grades short or single-class passwords as weak", () => {
    expect(passwordStrength("abcdef")).toEqual({ level: 1, label: "弱" });
    expect(passwordStrength("abcdefgh")).toEqual({ level: 1, label: "弱" });
  });

  it("grades 8+ chars with two classes as medium", () => {
    expect(passwordStrength("passw0rd")).toEqual({ level: 2, label: "中" });
  });

  it("grades 8+ chars with three classes as strong", () => {
    expect(passwordStrength("Passw0rd!")).toEqual({ level: 3, label: "强" });
    expect(passwordStrength("Passw0rd")).toEqual({ level: 3, label: "强" });
    // 只有两类（小写+数字）是中
    expect(passwordStrength("passw0rd1")).toEqual({ level: 2, label: "中" });
  });
});

describe("mapResetError", () => {
  it("maps verification-code failures onto the code field", () => {
    expect(mapResetError("验证码错误或已过期")).toEqual({
      field: "code",
      message: "验证码错误或已过期",
    });
    expect(mapResetError("验证码已失效，请重新获取")?.field).toBe("code");
  });

  it("maps an unregistered email onto the email field", () => {
    expect(mapResetError("该邮箱尚未注册")?.field).toBe("email");
  });

  it("leaves unknown messages in the dialog-level error area", () => {
    expect(mapResetError("服务暂时不可用，请稍后重试")).toBeNull();
    expect(mapResetError("请求过于频繁，请稍后再试")).toBeNull();
  });
});
