import { createTextVNode, h, type VNode } from "vue";
import MarkdownIt from "markdown-it";
import type { Token } from "markdown-it";
import MarkdownImage from "./MarkdownImage.vue";
import { isAllowedUrl } from "./protocolAllowlist";

/**
 * Markdown 正文渲染器（ADR 0005）。
 *
 * 唯一真源就是这个模块：不接收任何 options、不暴露配置项（决策 10），
 * 渲染规则写死在这里，避免三处调用方行为漂移。
 *
 * 核心约束：`markdown-it` 只做解析（md.parse），绝不调用 md.render()（决策 2）。
 * 全程不产生 HTML 字符串、不用 v-html、不引入 DOMPurify——
 * 攻击面从「被过滤掉的危险」变成「根本不存在」。
 */
const md = new MarkdownIt();

/**
 * 取 token 属性并归一为 string。
 * @types/markdown-it 的 attrGet 返回 `string | number`，但我们的取值只会是字符串，
 * 统一转 string 避免下游类型抱怨。
 */
function getAttr(token: Token, name: string): string {
  const v = token.attrGet(name);
  return v == null ? "" : String(v);
}

/**
 * 标题降级映射（决策 11）：源 Markdown 的 H1–H3 分别降到 h2/h3/h4，
 * 避免与详情页 <h1> 冲突。源 H4+ 超出 ADR 0005 决策 2 支持的 H1–H3 子集，
 * 不出现在映射里，会被当作不支持结构降级为纯文本（见 heading_open/close）。
 */
const HEADING_TAGS: Record<string, string> = {
  h1: "h2",
  h2: "h3",
  h3: "h4",
};

type Child = VNode | string;

/** 把一段 VNode 子树摊平成纯文本（用于不支持结构的降级，决策 2/4）。 */
function toText(nodes: Child[]): string {
  let out = "";
  for (const n of nodes) {
    if (typeof n === "string") {
      out += n;
    } else if (typeof n.children === "string") {
      out += n.children;
    } else if (Array.isArray(n.children)) {
      out += toText(n.children as Child[]);
    }
  }
  return out;
}

/** 行内 token（text / strong / em / 链接 / 图片 ...）→ VNode 列表。 */
function renderInline(tokens: Token[]): VNode[] {
  const root: Child[] = [];
  // 栈式配对：open 压栈，close 出栈并把收集到的子节点包成对应元素。
  const stack: { tag: string; href?: string; nodes: Child[] }[] = [
    { tag: "#root", nodes: root },
  ];

  for (const t of tokens) {
    const top = stack[stack.length - 1];
    switch (t.type) {
      case "text":
        top.nodes.push(createTextVNode(t.content));
        break;
      case "code_inline":
        top.nodes.push(h("code", t.content));
        break;
      case "softbreak":
        top.nodes.push(createTextVNode(" "));
        break;
      case "hardbreak":
        top.nodes.push(h("br"));
        break;
      case "strong_open":
        stack.push({ tag: "strong", nodes: [] });
        break;
      case "strong_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("strong", node.nodes));
        break;
      }
      case "em_open":
        stack.push({ tag: "em", nodes: [] });
        break;
      case "em_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("em", node.nodes));
        break;
      }
      case "s_open":
        stack.push({ tag: "s", nodes: [] });
        break;
      case "s_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("s", node.nodes));
        break;
      }
      case "link_open": {
        const href = getAttr(t, "href");
        if (isAllowedUrl(href)) {
          stack.push({ tag: "a", href, nodes: [] });
        } else {
          // 不合规协议：保留文字、去掉可点击性（决策 5），不整段丢弃。
          stack.push({ tag: "#text-only", nodes: [] });
        }
        break;
      }
      case "link_close": {
        const node = stack.pop()!;
        if (node.tag === "a") {
          // 决策 5：仅渲染白名单通过的 href，不附加 target/rel。
          // （加 _blank 会对站内相对链接造成未要求的 UX 改动；无 _blank 便无 tabnabbing 风险。）
          stack[stack.length - 1].nodes.push(h("a", { href: node.href }, node.nodes));
        } else {
          // 降级：把原本链接里的文字直接摊平到外层。
          stack[stack.length - 1].nodes.push(...node.nodes);
        }
        break;
      }
      case "image": {
        // 注意：markdown-it 把图片 alt 存在 token.content 里，而不是 attrs。
        // （链接的 href 在 attrs，图片的 alt 在 content——这是库的不一致设计，已确认。）
        const src = getAttr(t, "src");
        const alt = t.content ?? "";
        top.nodes.push(h(MarkdownImage, { src, alt }));
        break;
      }
      default:
        // 未知 inline token：降级为纯文本（决策 4），不丢弃。
        if (t.content) top.nodes.push(createTextVNode(t.content));
        break;
    }
  }

  return root.filter((n): n is VNode => typeof n !== "string") as VNode[];
}

/** 块级 token（段落 / 标题 / 列表 / 引用 / 代码块 ...）→ VNode 列表。 */
function renderBlock(tokens: Token[]): VNode[] {
  const root: Child[] = [];
  const stack: { tag: string; nodes: Child[] }[] = [{ tag: "#root", nodes: root }];

  for (const t of tokens) {
    const top = stack[stack.length - 1];
    switch (t.type) {
      case "paragraph_open":
        stack.push({ tag: "p", nodes: [] });
        break;
      case "paragraph_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("p", node.nodes));
        break;
      }
      case "heading_open": {
        const mapped = HEADING_TAGS[t.tag];
        if (mapped) {
          stack.push({ tag: mapped, nodes: [] });
        } else {
          // 源 Markdown H4+ 超出支持的 H1–H3 子集 → 降级为纯文本（决策 2），不丢弃。
          stack.push({ tag: "#plain", nodes: [] });
        }
        break;
      }
      case "heading_close": {
        const node = stack.pop()!;
        if (node.tag === "#plain") {
          // 把标题内的文字摊平为纯文本（决策 2），不丢弃。
          stack[stack.length - 1].nodes.push(createTextVNode(toText(node.nodes)));
        } else {
          stack[stack.length - 1].nodes.push(h(node.tag, node.nodes));
        }
        break;
      }
      case "bullet_list_open":
        stack.push({ tag: "ul", nodes: [] });
        break;
      case "bullet_list_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("ul", node.nodes));
        break;
      }
      case "ordered_list_open":
        stack.push({ tag: "ol", nodes: [] });
        break;
      case "ordered_list_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("ol", node.nodes));
        break;
      }
      case "list_item_open":
        stack.push({ tag: "li", nodes: [] });
        break;
      case "list_item_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("li", node.nodes));
        break;
      }
      case "blockquote_open":
        stack.push({ tag: "blockquote", nodes: [] });
        break;
      case "blockquote_close": {
        const node = stack.pop()!;
        stack[stack.length - 1].nodes.push(h("blockquote", node.nodes));
        break;
      }
      case "fence": {
        // 围栏代码块：等宽 + 浅底，无语法高亮（决策 7），横向滚动由 CSS 负责（决策 13）。
        const code = h("code", t.content.replace(/\n$/, ""));
        top.nodes.push(h("pre", code));
        break;
      }
      case "hr":
        top.nodes.push(h("hr"));
        break;
      case "inline":
        top.nodes.push(...renderInline(t.children ?? []));
        break;
      case "html_block":
        // HTML 块结构性排除：降级为纯文本，不执行也不丢弃（决策 4）。
        if (t.content) top.nodes.push(createTextVNode(t.content));
        break;
      default:
        // 未知 block token（表格 / 脚注 / 嵌套列表等）：降级为纯文本（决策 4），不丢弃。
        if (t.content) top.nodes.push(createTextVNode(t.content));
        break;
    }
  }

  return root.filter((n): n is VNode => typeof n !== "string") as VNode[];
}

/**
 * 把 Markdown 源串渲染成 Vue VNode 数组。
 *
 * 调用方（MarkdownContent.vue）负责把这些 VNode 挂到 DOM；本函数纯函数、
 * 无副作用，便于单测用 renderToString 断言安全属性。
 */
export function renderMarkdown(source: string): VNode[] {
  const tokens = md.parse(source ?? "", {});
  return renderBlock(tokens);
}
