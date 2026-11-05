import { describe, expect, it } from "vitest";
import { h } from "vue";
import { renderToString } from "vue/server-renderer";
import MarkdownImage from "./MarkdownImage.vue";

/**
 * 只断言图片组件的安全属性与降级行为（ADR 0005 决策 3 / 12）。
 * 加载失败（onError）分支是浏览器事件驱动的纯逻辑（failed=true → 占位），
 * 这里用协议门占位覆盖「占位块文案」路径；onError 交互由组件逻辑保证。
 */

async function render(src: string, alt: string): Promise<string> {
  return renderToString(h(MarkdownImage, { src, alt }));
}

describe("MarkdownImage — 统一属性与占位降级（决策 3 / 12）", () => {
  it("外链图片渲染 <img> 且带 loading=lazy 与 referrerpolicy=no-referrer", async () => {
    // 外链不需要我们的凭据，直接交给 <img>；站内附件走 blob（见下一个用例）。
    const html = await render("https://example.com/a.png", "图1");
    expect(html).toContain("<img");
    expect(html).toContain('loading="lazy"');
    expect(html).toContain('referrerpolicy="no-referrer"');
    expect(html).toContain('alt="图1"');
  });

  it("站内附件首屏渲染骨架而非裸 <img>", async () => {
    // 读取端点挂 authRequired，<img src> 带不上 token，必须先 fetch 成 blob URL。
    // 因此首屏是骨架而不是直接指向 /api/v1/attachments/1 的 <img>。
    const html = await render("/api/v1/attachments/1", "图1");
    expect(html).not.toContain('src="/api/v1/attachments/1"');
    expect(html).toContain("md-img-skeleton");
  });

  it("危险协议（javascript: / data:）不渲染 <img>，改为占位块", async () => {
    const js = await render("javascript:alert(1)", "恶意图");
    expect(js).not.toContain("<img");
    expect(js).toContain("图片不可用");

    const data = await render("data:text/html,<b>x</b>", "数据图");
    expect(data).not.toContain("<img");
    expect(data).toContain("图片不可用");
  });

  it("空 alt 的装饰性图片：占位块不带补充说明", async () => {
    // 用危险协议触发占位（决策 12：占位块出现时，空 alt 视为装饰性，文案不带 「：」）。
    const html = await render("javascript:x", "");
    expect(html).toContain("图片不可用");
    expect(html).not.toContain("图片不可用：");
  });

  it("带 alt 的占位块显示「图片不可用：{alt}」", async () => {
    const html = await render("javascript:x", "凭证图");
    expect(html).toContain("图片不可用：凭证图");
  });
});
