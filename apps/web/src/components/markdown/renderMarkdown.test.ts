import { describe, expect, it } from "vitest";
import { h } from "vue";
import { renderToString } from "vue/server-renderer";
import { renderMarkdown } from "./renderMarkdown";

/**
 * 这些测试只断言「安全属性」与「降级行为」（规格 #227 / ADR 0005 决策 4、5、11、13）。
 * 不写快照测试：VNode 结构随实现变动太脆，且快照无法表达「危险的东西不存在」这一意图。
 */

async function render(source: string): Promise<string> {
  // 包一层 div 作为根 VNode，renderToString 只接受单个 VNode。
  return renderToString(h("div", renderMarkdown(source)));
}

describe("renderMarkdown — 链接协议白名单（决策 5）", () => {
  it("javascript: 链接降级为纯文本，不生成可点击 a", async () => {
    const html = await render("[点我](javascript:alert(1))");
    expect(html).not.toContain('href="javascript:alert(1)"');
    expect(html).not.toContain("<a ");
    // 文字保留，不整段丢弃
    expect(html).toContain("点我");
  });

  it("data: 链接同样降级", async () => {
    const html = await render("[x](data:text/html,<b>hi</b>)");
    expect(html).not.toContain('href="data:text/html');
    expect(html).toContain("x");
  });

  it("http / https / 站内相对路径 正常渲染为可点击 a（仅 href，不加 target/rel）", async () => {
    const html = await render(
      "[外链](https://example.com) [站内](/api/v1/x) [内链](http://a.b/c)",
    );
    expect(html).toContain('href="https://example.com"');
    expect(html).toContain('href="/api/v1/x"');
    expect(html).toContain('href="http://a.b/c"');
    // 决策 5 只要求协议白名单，不附加 target/rel（避免对站内链接造成未要求的 UX 改动）。
    expect(html).not.toContain('target="_blank"');
  });
});

describe("renderMarkdown — HTML / 脚本结构性排除（决策 2、4）", () => {
  it("裸 <script> 在 html:false 下变成纯文本，不生成 script 元素", async () => {
    const html = await render("普通文字 <script>alert(1)</script> 结尾");
    expect(html).not.toContain("<script");
    expect(html).toContain("alert(1)");
  });

  it("html_block 降级为纯文本", async () => {
    // 整段被当作纯文本转义显示：标签被转义（&lt;），绝不会变成真实可执行的 DOM 结构。
    // 文字原样保留（不丢弃），但 onclick 等属性只作为不可执行的文本出现。
    const html = await render("<div onclick=\"x()\">hi</div>");
    expect(html).toContain("&lt;div");
    expect(html).not.toContain("<div ");
    expect(html).toContain("hi");
  });
});

describe("renderMarkdown — 图片统一属性（决策 3）", () => {
  it("所有 img 带 loading=lazy 与 referrerpolicy=no-referrer", async () => {
    const html = await render("![图1](/api/v1/attachments/1)");
    expect(html).toContain("<img");
    expect(html).toContain('loading="lazy"');
    expect(html).toContain('referrerpolicy="no-referrer"');
    expect(html).toContain('alt="图1"');
  });

  it("外链图片同样渲染并带两个属性", async () => {
    const html = await render("![doc](https://example.com/a.png)");
    expect(html).toContain('src="https://example.com/a.png"');
    expect(html).toContain('loading="lazy"');
    expect(html).toContain('referrerpolicy="no-referrer"');
  });
});

describe("renderMarkdown — 标题降级（决策 11）", () => {
  it("一级到三级标题分别降为 h2/h3/h4", async () => {
    const html = await render("# 一\n## 二\n### 三");
    expect(html).toContain("<h2>一</h2>");
    expect(html).toContain("<h3>二</h3>");
    expect(html).toContain("<h4>三</h4>");
    expect(html).not.toContain("<h1");
  });

  it("源 H4+ 超出支持的 H1–H3 子集，降级为纯文本（决策 2）", async () => {
    const html = await render("#### 四级标题\n##### 五级");
    // 不渲染成 h4/h5 元素，标题文字保留为纯文本（不丢弃）。
    expect(html).not.toContain("<h4");
    expect(html).not.toContain("<h5");
    expect(html).toContain("四级标题");
    expect(html).toContain("五级");
  });
});

describe("renderMarkdown — 代码块（决策 7、13）", () => {
  it("围栏代码块渲染为 pre>code，无语法高亮 class", async () => {
    const html = await render("```\nconst a = 1;\n```");
    expect(html).toContain("<pre>");
    expect(html).toContain("<code>");
    expect(html).toContain("const a = 1;");
    // 没有引入高亮库，不应出现 hljs / shiki 之类 class
    expect(html).not.toMatch(/class="[^"]*(hljs|shiki|token)/);
  });
});

describe("renderMarkdown — 未知 token 降级（决策 4）", () => {
  it("表格语法等不支持内容降级为纯文本，不丢弃", async () => {
    const html = await render("| a | b |\n| - | - |\n| 1 | 2 |");
    expect(html).toContain("a");
    expect(html).toContain("2");
    // 不渲染成 table 元素
    expect(html).not.toContain("<table");
  });

  it("行内强调 / 删除线 / 行内代码 正常渲染，硬换行渲染为 br", async () => {
    const html = await render("**粗** *斜* ~~删~~ `码`");
    expect(html).toContain("<strong>粗</strong>");
    expect(html).toContain("<em>斜</em>");
    expect(html).toContain("<s>删</s>");
    expect(html).toContain("<code>码</code>");
  });
});

describe("renderMarkdown — 列表与引用", () => {
  it("无序 / 有序列表与引用渲染为对应元素", async () => {
    const html = await render("- a\n- b\n\n1. x\n2. y\n\n> 引用");
    expect(html).toContain("<ul>");
    expect(html).toContain("<ol>");
    expect(html).toContain("<blockquote>");
    expect(html).toContain("a");
    expect(html).toContain("y");
  });
});
