/**
 * 修改密码展示策略的测试（issue #85 A 档）。
 *
 * 锁住三件事：规则清单的三态（pending/ok/fail）判定、强度分档边界、
 * 后端文案到字段的分流——分流错了会把「当前密码不正确」显示成整块
 * 网络错误，用户不知道该改哪个框。
 */
import { describe, expect, it } from "vitest";

import {
  mapChangePasswordError,
  passwordRules,
  passwordStrength,
} from "./passwordPolicy";

describe("passwordRules", () => {
  it("stays pending while nothing is typed", () => {
    const states = Object.fromEntries(
      passwordRules("", "", "").map((rule) => [rule.id, rule.state]),
    );
    expect(states).toEqual({
      length: "pending",
      distinct: "pending",
      confirm: "pending",
    });
  });

  it("checks length bounds (6-20 follows the backend contract)", () => {
    expect(passwordRules("", "12345", "")[0]?.state).toBe("fail");
    expect(passwordRules("", "123456", "")[0]?.state).toBe("ok");
    expect(passwordRules("", "1".repeat(21), "")[0]?.state).toBe("fail");
  });

  it("flags a new password equal to the current one", () => {
    expect(passwordRules("same12", "same12", "")[1]?.state).toBe("fail");
    expect(passwordRules("old-pw", "new-pw", "")[1]?.state).toBe("ok");
    // 旧密码还没填时无从比较
    expect(passwordRules("", "whatever", "")[1]?.state).toBe("pending");
  });

  it("matches confirmation only when both sides are present", () => {
    expect(passwordRules("", "secret1", "")[2]?.state).toBe("pending");
    expect(passwordRules("", "secret1", "secret1")[2]?.state).toBe("ok");
    expect(passwordRules("", "secret1", "secret2")[2]?.state).toBe("fail");
  });
});

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

describe("mapChangePasswordError", () => {
  it("maps wrong-current-password messages onto the old password field", () => {
    expect(mapChangePasswordError("用户名或密码错误")).toEqual({
      field: "oldPassword",
      message: "当前密码不正确",
    });
    expect(
      mapChangePasswordError("登录失败次数过多，请 15 分钟后再试或重置密码")
        ?.field,
    ).toBe("oldPassword");
  });

  it("maps password-reused message onto the new password field", () => {
    expect(mapChangePasswordError("新密码不能与当前密码相同")).toEqual({
      field: "newPassword",
      message: "新密码不能与当前密码相同",
    });
  });

  it("leaves unknown messages in the card-level error area", () => {
    expect(mapChangePasswordError("服务暂时不可用，请稍后重试")).toBeNull();
    expect(mapChangePasswordError("请求过于频繁，请稍后再试")).toBeNull();
  });
});
