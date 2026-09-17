import { describe, expect, it } from "vitest";

import { formatISODate } from "./format";

describe("formatISODate", () => {
  it("takes the date part of an ISO-8601 timestamp", () => {
    expect(formatISODate("2026-09-02T20:14:00+08:00")).toBe("2026-09-02");
  });

  it("keeps the date as recorded instead of converting time zones", () => {
    // 这一条是重点：00:30+08:00 换算成 UTC 是前一天的 16:30。
    // 若用 new Date(iso).toLocaleDateString()，在 UTC 环境下会打印 09-01。
    // 报名时间要的是「记录当时当地的那一天」，所以必须仍是 09-02。
    expect(formatISODate("2026-09-02T00:30:00+08:00")).toBe("2026-09-02");
    expect(formatISODate("2026-09-02T23:30:00+08:00")).toBe("2026-09-02");
  });

  it("accepts a bare date without a time part", () => {
    expect(formatISODate("2026-09-02")).toBe("2026-09-02");
  });

  it("returns the raw value when it is not a recognisable ISO date", () => {
    // 这两条是防回归：坏数据要原样显示，不能变成 Invalid Date 或空白
    expect(formatISODate("")).toBe("");
    expect(formatISODate("待定")).toBe("待定");
    expect(formatISODate("2026/09/02")).toBe("2026/09/02");
  });
});
