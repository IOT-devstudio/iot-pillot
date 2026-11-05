import { describe, expect, it } from "vitest";
import {
  formatDateTime,
  formatDay,
  postExcerpt,
  replyRoleLabel,
} from "../model";

describe("forum model 展示逻辑（#219）", () => {
  it("formatDay 把 ISO 截成 YYYY-MM-DD", () => {
    expect(formatDay("2026-11-04T08:30:00Z")).toBe("2026-11-04");
  });

  it("formatDay 解析失败退回原串前 10 位", () => {
    expect(formatDay("not-a-date")).toBe("not-a-date");
  });

  it("formatDateTime 带时分", () => {
    // 不同时区结果不同，只验证形态与有效位数
    const out = formatDateTime("2026-11-04T08:30:00Z");
    expect(out).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
  });

  it("replyRoleLabel 区分官方/社区", () => {
    expect(replyRoleLabel(true)).toBe("官方回答");
    expect(replyRoleLabel(false)).toBe("社区回复");
  });

  it("postExcerpt 剥离 Markdown 标记", () => {
    const md = "# 标题 [链接](https://e.com) 与 ![图](/x.png) 以及 **加粗** 文字";
    const out = postExcerpt(md, 60);
    expect(out).not.toContain("**");
    expect(out).not.toContain("#");
    expect(out).toContain("[图]");
    expect(out).toContain("链接");
    expect(out).not.toContain("https://");
  });

  it("postExcerpt 超长截断并补省略号", () => {
    const out = postExcerpt("这是一段明显超过二十个字的招新问答正文内容用于验证截断逻辑", 20);
    expect(out.length).toBeLessThanOrEqual(21);
    expect(out).toContain("…");
  });

  it("postExcerpt 短文本不补省略号", () => {
    expect(postExcerpt("一句话", 120)).toBe("一句话");
  });
});
