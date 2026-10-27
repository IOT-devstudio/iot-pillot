/**
 * 报名入口四态判定的守卫测试（issue #152）。
 *
 * 锁住三件事：只有「明确缺字段」才拦报名；资料未知时 fail-open；
 * 完整与否只看班级 + 学号，不因为填了姓名就算完整。
 */
import { describe, expect, it } from "vitest";

import type { MyProfile } from "@/api/profile";
import { profileEntryState } from "./profileEntry";

function profile(overrides: Partial<MyProfile> = {}): MyProfile {
  return {
    user_id: 7,
    username: "member",
    role: "member",
    name: "张三",
    detail: { class: "物联网230301", student_id: 202311451, qq: "10001" },
    ...overrides,
  };
}

describe("profileEntryState", () => {
  it("offers 查看资料 and lets applications through when both required fields are filled and well-formed", () => {
    const entry = profileEntryState(profile());
    expect(entry).toMatchObject({
      state: "ready",
      missing: [],
      label: "查看资料",
      blocksApplication: false,
    });
  });

  it("blocks applications and names only the missing field", () => {
    expect(
      profileEntryState(profile({ detail: { class: "物联网230301", student_id: 0 } })),
    ).toMatchObject({ state: "needs-completion", missing: ["学号"], label: "完善资料", blocksApplication: true });
    expect(
      profileEntryState(profile({ detail: { class: " ", student_id: 202311451 } })),
    ).toMatchObject({ state: "needs-completion", missing: ["班级"] });
    expect(profileEntryState(profile({ detail: {} }))).toMatchObject({
      state: "needs-completion",
      missing: ["班级", "学号"],
      blocksApplication: true,
    });
  });

  it("blocks applications when values exist but the format is invalid", () => {
    // 0012 口径：班级必须是「汉字 + 6 位数字」、学号 9 位数字。
    // 填了但格式不符同样会被定时冻结，所以入口也要先拦住去改。
    expect(
      profileEntryState(profile({ detail: { class: "物联网 2301", student_id: 202311451 } })),
    ).toMatchObject({ state: "needs-completion", missing: ["班级"], blocksApplication: true });
    expect(
      profileEntryState(profile({ detail: { class: "物联网230301", student_id: 2023114514 } })),
    ).toMatchObject({ state: "needs-completion", missing: ["学号"], blocksApplication: true });
  });

  it("does not treat a filled name as a complete profile", () => {
    // 验收标准明确要求「不仅以姓名判断」：姓名有值但班级/学号为空仍然要拦。
    const entry = profileEntryState(profile({ name: "张三", detail: {} }));
    expect(entry.state).toBe("needs-completion");
    expect(entry.blocksApplication).toBe(true);
  });

  it("fails open with neutral wording when the profile is unknown", () => {
    // 加载中 / 读取失败都会让共享资料回到 null，此时不能拦报名。
    expect(profileEntryState(null)).toMatchObject({
      state: "unknown",
      missing: [],
      label: "个人资料",
      blocksApplication: false,
    });
  });

  it("hides the entry for founding admins even when their detail is empty", () => {
    expect(
      profileEntryState(profile({ is_founding: true, detail: {} })),
    ).toMatchObject({ state: "hidden", label: "", blocksApplication: false });
  });

  it("treats a missing detail object as incomplete rather than crashing", () => {
    // 创始管理员以外的账号理论上都有 detail，但接口缺字段时不能读崩。
    expect(profileEntryState(profile({ detail: undefined }))).toMatchObject({
      state: "needs-completion",
      missing: ["班级", "学号"],
    });
  });
});
