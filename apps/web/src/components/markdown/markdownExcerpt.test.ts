import { describe, expect, it } from "vitest";
import { markdownExcerpt } from "./markdownExcerpt";

/**
 * 摘要只断言「标记被剥离、正文可读」（ADR 0005 决策 8）。
 * 长度截断由既有 bodyExcerpt 负责，这里不重复其逻辑。
 */
describe("markdownExcerpt — 剥离标记", () => {
  it("图片语法整体替换为 [图]", () => {
    const out = markdownExcerpt("看看 ![架构图](/api/v1/attachments/9) 再说");
    expect(out).toContain("[图]");
    expect(out).not.toContain("![架构图]");
    expect(out).not.toContain("/api/v1/attachments/9");
  });

  it("链接只留文字，去掉 URL", () => {
    const out = markdownExcerpt("详见 [文档](https://example.com/doc)");
    expect(out).toContain("文档");
    expect(out).not.toContain("https://example.com/doc");
  });

  it("去掉行首的标题 / 引用 / 列表 / 序号标记", () => {
    const src = "# 标题\n> 引用\n- 项目\n1. 第一点\n**粗** 文字";
    const out = markdownExcerpt(src);
    expect(out).not.toContain("# 标题");
    expect(out).not.toContain("> 引用");
    expect(out).not.toContain("- 项目");
    expect(out).not.toContain("1. 第一点");
    // 强调标记被去掉，但文字保留
    expect(out).toContain("粗");
    expect(out).toContain("文字");
    expect(out).not.toContain("**");
  });

  it("多行内容压成单行预览（交由 bodyExcerpt 压空白）", () => {
    const out = markdownExcerpt("第一行\n\n第二行   第三行");
    expect(out).not.toContain("\n");
  });

  it("空输入安全返回空串", () => {
    expect(markdownExcerpt("")).toBe("");
    expect(markdownExcerpt(undefined as unknown as string)).toBe("");
  });
});
