import { bodyExcerpt } from "@/modules/studio-knowledge/model";

/**
 * Markdown 正文摘要（ADR 0005 决策 8）。
 *
 * 列表页要截一段纯文本预览，但直接把 Markdown 原文交给既有 bodyExcerpt
 * 会把 **加粗** / ## 标题 / ![图](...) 原样显示出来。
 * 这里先剥掉 Markdown 标记，再交给 bodyExcerpt：
 *   - 图片语法整体替换成 [图]；
 *   - 链接只留文字（去掉 URL）；
 *   - 去掉行首的 # / > / - / * / 数字序号；
 *   - 去掉强调与行内代码的标记符号。
 *
 * 不引入渲染器：列表渲染 20 条正文的 VNode 树纯属浪费，摘要本来也不该有排版。
 */
const IMAGE_RE = /!\[[^\]]*\]\([^)]*\)/g;
const LINK_RE = /\[([^\]]+)\]\([^)]*\)/g;
// 行首层级标记：# ## > - * 以及 "1." 这类有序序号，逐个块首去掉。
const LEADING_MARKER_RE = /^[ \t]*([#>*\-]|\d+\.)[ \t]*/gm;
const EMPHASIS_RE = /(\*\*|__|\*|_|~~|`)/g;

export function markdownExcerpt(source: string, max = 120): string {
  const stripped = String(source ?? "")
    .replace(IMAGE_RE, "[图]")
    .replace(LINK_RE, "$1")
    .replace(LEADING_MARKER_RE, "")
    .replace(EMPHASIS_RE, "");

  return bodyExcerpt(stripped, max);
}
