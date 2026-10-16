/**
 * 方向展示元数据的守卫测试（issue #89）。
 *
 * 锁住两件事：三个内置方向的键齐全（漏一个页面就降级成通用样式），
 * 以及未知 code 走通用降级而不是 undefined 崩模板。
 */
import { describe, expect, it } from "vitest";

import {
  DIRECTION_PRESENTATION,
  interviewBlockReason,
  presentationOf,
} from "./directionPresentation";

describe("directionPresentation", () => {
  it("covers the three seeded direction codes", () => {
    for (const code of ["front-end", "back-end", "agent"]) {
      const presentation = presentationOf(code);
      expect(presentation.headline.trim()).not.toBe("");
      expect(presentation.tag.trim()).not.toBe("");
      expect(presentation.symbol.trim()).not.toBe("");
      expect(presentation.summary.trim()).not.toBe("");
    }
    expect(Object.keys(DIRECTION_PRESENTATION).sort()).toEqual([
      "agent",
      "back-end",
      "front-end",
    ]);
  });

  it("falls back to generic presentation for unknown codes", () => {
    const fallback = presentationOf("embedded");
    expect(fallback.headline).toBe(presentationOf("nope").headline);
    expect(fallback.summary.trim()).not.toBe("");
  });
});

/**
 * 面试路径置灰判定（issue #136 §5 边界表）。
 * 按票逐场景锁行为：无表单 / 表单未开放 / 申请终态 / 可补交 / 字段缺省。
 */
describe("interviewBlockReason", () => {
  it("blocks when the direction has no form (or form is null)", () => {
    expect(interviewBlockReason({ form: null }, null)).toBe(
      "该方向暂未开放面试",
    );
    // #135 交付前 form 字段整体缺省：同样按「无表单」降级，不能崩
    expect(interviewBlockReason({}, null)).toBe("该方向暂未开放面试");
    expect(interviewBlockReason(null, null)).toBe("该方向暂未开放面试");
  });

  it("blocks when the form is closed", () => {
    expect(
      interviewBlockReason({ form: { is_open: false } }, null),
    ).toBe("面试表单未开放");
  });

  it("blocks when the application reached a terminal state", () => {
    const direction = { form: { is_open: true } };
    expect(interviewBlockReason(direction, { status: "accepted" })).toBe(
      "申请已结束",
    );
    expect(interviewBlockReason(direction, { status: "rejected" })).toBe(
      "申请已结束",
    );
  });

  it("allows the interview while the application is still in review", () => {
    // 先报名、后补交面试的主场景（§2.4）
    const direction = { form: { is_open: true } };
    expect(interviewBlockReason(direction, { status: "applied" })).toBeNull();
    expect(interviewBlockReason(direction, { status: "reviewing" })).toBeNull();
  });

  it("allows the interview with no application at all", () => {
    expect(
      interviewBlockReason({ form: { is_open: true } }, null),
    ).toBeNull();
  });

  it("prefers the no-form reason over terminal state", () => {
    // 优先级：无表单 > 表单未开放 > 申请已结束
    expect(
      interviewBlockReason({ form: null }, { status: "rejected" }),
    ).toBe("该方向暂未开放面试");
    expect(
      interviewBlockReason({ form: { is_open: false } }, { status: "rejected" }),
    ).toBe("面试表单未开放");
  });
});
