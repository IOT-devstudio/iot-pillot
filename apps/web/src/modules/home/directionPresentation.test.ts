/**
 * 方向展示元数据的守卫测试（issue #89）。
 *
 * 锁住两件事：三个内置方向的键齐全（漏一个页面就降级成通用样式），
 * 以及未知 code 走通用降级而不是 undefined 崩模板。
 */
import { describe, expect, it } from "vitest";

import { DIRECTION_PRESENTATION, presentationOf } from "./directionPresentation";

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
