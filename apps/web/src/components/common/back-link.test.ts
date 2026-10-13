/**
 * 返回链接分流规则的测试（issue #94）。
 *
 * 锁住三件事：管理员回控制台、普通成员与角色未知回个人首页、
 * 显式 to / label 优先于角色推断。
 */
import { describe, expect, it } from "vitest";

import { backLinkLabel, backLinkTarget } from "./back-link";

describe("backLinkTarget", () => {
  it("sends admins to the dashboard", () => {
    expect(backLinkTarget(undefined, "admin")).toBe("/dashboard");
  });

  it("sends members to their personal home", () => {
    expect(backLinkTarget(undefined, "member")).toBe("/user/home");
  });

  it("falls back to the personal home while the role is unknown", () => {
    // 加载中、查询失败都算未知；/user/home 两个角色都能进，不会把人引进死路
    expect(backLinkTarget(undefined, null)).toBe("/user/home");
  });

  it("prefers an explicit target over the role", () => {
    expect(backLinkTarget("/recruitment/prospects", "admin")).toBe(
      "/recruitment/prospects",
    );
    expect(backLinkTarget("/user/home", "admin")).toBe("/user/home");
  });
});

describe("backLinkLabel", () => {
  it("labels the dashboard target as 返回控制台", () => {
    expect(backLinkLabel(undefined, undefined, "admin")).toBe("← 返回控制台");
  });

  it("labels the personal-home target as 返回个人首页", () => {
    expect(backLinkLabel(undefined, undefined, "member")).toBe(
      "← 返回个人首页",
    );
  });

  it("follows an explicit target when choosing the default label", () => {
    expect(backLinkLabel("/user/home", undefined, "admin")).toBe(
      "← 返回个人首页",
    );
  });

  it("prefers an explicit label over everything", () => {
    expect(backLinkLabel("/recruitment/prospects", "← 返回意向成员列表", "admin")).toBe(
      "← 返回意向成员列表",
    );
  });
});
