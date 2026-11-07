import { describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";
import { renderToString } from "vue/server-renderer";

import { findAll, makeNode, settle, testRenderer } from "@/test/test-renderer";

/**
 * 站内附件的取数桩（#242）。
 *
 * 组件把 /api/v1/attachments/:id fetch 成 blob URL —— 这条路径要能测到
 * 「取回成功后渲染出 <img>」，所以必须能控制它 resolve / reject。
 * isAttachmentUrl 等纯函数保持真身，本用例要验证的正是它们的配合。
 */
const attachmentApi = vi.hoisted(() => ({ loadAttachmentObjectUrl: vi.fn() }));

vi.mock("@/api/attachment", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/api/attachment")>()),
  loadAttachmentObjectUrl: attachmentApi.loadAttachmentObjectUrl,
}));

import MarkdownImage from "./MarkdownImage.vue";

/**
 * 断言图片组件的安全属性与降级行为（ADR 0005 决策 3 / 12），
 * 以及 #242 修掉的「所有图片都显示图片不可用」。
 *
 * 两种渲染方式各有分工：
 *   - renderToString：首屏形态（骨架 / <img> / 占位块）的最短路径；
 *   - 自定义 renderer：需要等异步 fetch resolve 后再看树的用例。
 */

async function render(src: string, alt: string): Promise<string> {
  return renderToString(h(MarkdownImage, { src, alt }));
}

async function mount(
  src: string,
  alt: string,
): Promise<{ root: ReturnType<typeof makeNode>; unmount: () => void }> {
  const root = makeNode("root");
  const app = testRenderer.createApp(
    defineComponent({ setup: () => () => h(MarkdownImage, { src, alt }) }),
  );
  app.mount(root);
  return { root, unmount: () => app.unmount() };
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

describe("MarkdownImage — 站内附件取回后真正渲染 <img>（#242）", () => {
  it("fetch 成功后 <img> 的 src 是组件自己生成的 blob URL", async () => {
    // 回归点：`new URL("blob:…").protocol === "blob:"`，此前用正文协议白名单
    // 校验 resolvedSrc 会让 showImage 恒为 false，任何图片都落占位块。
    attachmentApi.loadAttachmentObjectUrl.mockReset();
    attachmentApi.loadAttachmentObjectUrl.mockResolvedValue(
      "blob:http://localhost/5f0e1c9a",
    );

    const view = await mount("/api/v1/attachments/7", "图1");
    try {
      // 首屏是骨架，不是占位块。
      expect(findAll(view.root, (n) => n.type === "img")).toHaveLength(0);
      expect(
        findAll(view.root, (n) => n.type === "span" && n.props.role === "img"),
      ).toHaveLength(1);

      await settle();

      const images = findAll(view.root, (n) => n.type === "img");
      expect(images).toHaveLength(1);
      expect(images[0]?.props.src).toBe("blob:http://localhost/5f0e1c9a");
      expect(images[0]?.props.alt).toBe("图1");
      expect(images[0]?.props.loading).toBe("lazy");
      expect(images[0]?.props.referrerpolicy).toBe("no-referrer");
      // 渲染出真图之后不该再有占位文案。
      expect(
        findAll(view.root, (n) => n.type === "span" && n.text.includes("图片不可用")),
      ).toHaveLength(0);
      expect(attachmentApi.loadAttachmentObjectUrl).toHaveBeenCalledWith(7);
    } finally {
      view.unmount();
    }
  });

  it("取数失败仍然落回占位块（401 / 404 / 网络错误同一种结果）", async () => {
    attachmentApi.loadAttachmentObjectUrl.mockReset();
    attachmentApi.loadAttachmentObjectUrl.mockRejectedValue(
      new Error("附件加载失败（404）"),
    );

    const view = await mount("/api/v1/attachments/9", "图2");
    try {
      await settle();
      expect(findAll(view.root, (n) => n.type === "img")).toHaveLength(0);
      const fallback = findAll(
        view.root,
        (n) => n.type === "span" && n.text.includes("图片不可用"),
      );
      expect(fallback[0]?.text).toBe("图片不可用：图2");
    } finally {
      view.unmount();
    }
  });
});
