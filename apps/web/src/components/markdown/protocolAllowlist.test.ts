import { describe, expect, it } from "vitest";
import { isAllowedUrl } from "./protocolAllowlist";

/**
 * 正文协议白名单（ADR 0005 决策 5）的唯一真源。
 *
 * #242 这次踩到的坑就在这附近：站内附件取回后是 blob URL，`protocol` 是 `blob:`，
 * 被这里拒绝——但**正确的修法不是往白名单里加 blob:**。本文件把这个判断锁住：
 * 白名单的职责是「正文里允许出现哪些协议」，一旦放行 `blob:`，
 * `![](blob:…)` 就成了合法正文语法，而那正是决策 5 想排除的东西。
 * 组件对自造的 blob URL 用局部规则放行，见 MarkdownImage.vue。
 */
describe("isAllowedUrl — 正文协议白名单（决策 5）", () => {
  it("放行 http / https", () => {
    expect(isAllowedUrl("http://example.com/a.png")).toBe(true);
    expect(isAllowedUrl("https://example.com/a.png")).toBe(true);
  });

  it("放行以 / 开头的站内相对路径（含附件地址）", () => {
    expect(isAllowedUrl("/api/v1/attachments/1")).toBe(true);
    expect(isAllowedUrl("/user/home")).toBe(true);
  });

  it("拒绝 javascript: / data: / file:", () => {
    expect(isAllowedUrl("javascript:alert(1)")).toBe(false);
    expect(isAllowedUrl("data:image/png;base64,AAAA")).toBe(false);
    expect(isAllowedUrl("file:///etc/passwd")).toBe(false);
  });

  it("仍然拒绝 blob:（#242 回归锁：不许顺手加进白名单）", () => {
    // 组件自造的 blob URL 由 MarkdownImage 的局部规则放行；
    // 白名单一旦松口，`![](blob:…)` 就会变成合法正文语法。
    expect(isAllowedUrl("blob:http://localhost/5f0e1c9a")).toBe(false);
    expect(isAllowedUrl("blob:null/5f0e1c9a")).toBe(false);
  });

  it("拒绝解析不出协议的字符串", () => {
    expect(isAllowedUrl("")).toBe(false);
    expect(isAllowedUrl("example.com/a.png")).toBe(false);
  });
});
