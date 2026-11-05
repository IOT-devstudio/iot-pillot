/**
 * 编辑器插图的状态机（#220，ADR 0004 决策 4）。
 *
 * 抽成纯逻辑而不是写进 .vue：
 *   - 「一次一张串行上传」「单张失败可重试」「删缩略图即从正文移除语法」
 *     这三件事都是可被判定的状态转移，用单测锁住比组件测试便宜一个量级；
 *   - 正文（Markdown 文本）是绑定的唯一真源（ADR 0004 决策 3），
 *     所以条目状态必须能由「正文里有没有这段语法」反推，而不是独立维护一份清单。
 */

import {
  ATTACHMENT_MAX_PER_DOCUMENT,
  attachmentUrl,
} from "@/api/attachment";

/** 一张图在编辑过程中的状态。 */
export type ImageItemStatus = "uploading" | "done" | "failed";

/** 缩略图条带里的一项。 */
export interface ImageItem {
  /** 本地唯一键：同一文件可能被选两次，不能用文件名或 id 当 key。 */
  key: string;
  /** 上传成功后拿到的附件 id；上传中/失败为 null。 */
  attachmentId: number | null;
  /** 本地文件名，仅用于失败重试与提示，不发给服务端。 */
  fileName: string;
  status: ImageItemStatus;
  /** 失败原因（展示给用户）。 */
  error: string;
  /** 本地预览地址（blob URL）：上传中也能看见自己选了哪张。 */
  previewUrl: string;
}

/** 生成语法用的 alt：文件名去掉扩展名，空则用「图片」。 */
export function altFromFileName(fileName: string): string {
  const base = fileName.replace(/\.[^.]+$/, "").trim();
  return base === "" ? "图片" : base;
}

/** 一项的 Markdown 图片语法。上传中的条目 id 未知，返回 null。 */
export function markdownSyntaxFor(item: ImageItem): string | null {
  if (item.attachmentId === null) return null;
  return `![${altFromFileName(item.fileName)}](${attachmentUrl(item.attachmentId)})`;
}

/**
 * 还剩多少个空位。
 *
 * 上限按**条带里的条目总数**算（含失败项），不是只算成功的：
 * 否则连传 9 张失败后还能再选 9 张，用户面对的是一屏红叉。
 */
export function remainingSlots(items: readonly ImageItem[]): number {
  return Math.max(0, ATTACHMENT_MAX_PER_DOCUMENT - items.length);
}

/** 超限提示文案。null 表示没超。 */
export function overLimitMessage(
  requested: number,
  items: readonly ImageItem[],
): string | null {
  const left = remainingSlots(items);
  if (requested <= left) return null;
  if (left === 0) {
    return `一次最多插 ${ATTACHMENT_MAX_PER_DOCUMENT} 张图，请先移除一些再选`;
  }
  return `一次最多插 ${ATTACHMENT_MAX_PER_DOCUMENT} 张图，本次只接收前 ${left} 张`;
}

/**
 * 提交前的闸门：还有失败中的图就拦住。
 *
 * 返回 null 表示可以提交；否则返回给用户看的原因。
 * 上传中**不拦**（那是等一会儿的事，用户看到骨架自然知道），
 * 但失败必须拦：正文里没有该图的语法，放行等于静默丢图。
 */
export function blockReasonOnSubmit(items: readonly ImageItem[]): string | null {
  const failed = items.filter((i) => i.status === "failed").length;
  if (failed > 0) {
    return `还有 ${failed} 张图片上传失败，请重试或移除后再提交`;
  }
  const uploading = items.filter((i) => i.status === "uploading").length;
  if (uploading > 0) {
    return `还有 ${uploading} 张图片正在上传，请稍候`;
  }
  return null;
}

/**
 * 从正文移除某张图的语法。
 *
 * 只删这一条精确语法（含可能存在的换行），不碰其它内容。
 * 找不到时原样返回——条目可能在上传中（正文里本来还没有语法）。
 */
export function removeSyntax(body: string, item: ImageItem): string {
  const syntax = markdownSyntaxFor(item);
  if (syntax === null) return body;

  // 语法可能独占一行，连同相邻的换行一起删，避免留下空行。
  // 优先删「前面的换行 + 语法」（中间行与末行都能干净收尾）；
  // 首行没有前置换行可删，此时改删「语法 + 后面的换行」。
  const withLeadingLine = `\n${syntax}`;
  if (body.includes(withLeadingLine)) return body.replace(withLeadingLine, "");

  const withTrailingLine = `${syntax}\n`;
  if (body.includes(withTrailingLine)) return body.replace(withTrailingLine, "");

  return body.replace(syntax, "");
}

/**
 * 把一条语法插到光标处。
 *
 * 前后各补一个换行：图片语法夹在段落文字中间时，Markdown 会把它并进同一段，
 * 缩略图与正文位置对不上。插入后返回新正文与新的光标位置（落在图片之后、空行之前），
 * 让用户可以接着往下写。
 */
export function insertSyntaxAtCursor(
  body: string,
  cursor: number,
  syntax: string,
): { body: string; cursor: number } {
  const safeCursor = Math.min(Math.max(cursor, 0), body.length);
  const before = body.slice(0, safeCursor);
  const after = body.slice(safeCursor);

  const needsLeadingBreak = before.length > 0 && !before.endsWith("\n");

  const inserted = `${needsLeadingBreak ? "\n" : ""}${syntax}\n`;
  const nextBody = `${before}${inserted}${after}`;
  // 光标落在插入内容之后（也就是那个换行之后），用户可以接着往下写。
  return { body: nextBody, cursor: before.length + inserted.length };
}

/**
 * 依据正文校准条目：正文里已经没有该图语法的「已完成」条目应当被移除。
 *
 * 这是「正文是唯一真源」的落地点：用户手动在文本域里删掉 `![x](/api/v1/attachments/1)`
 * 时，缩略图条带也要跟着消失，否则条带与正文各说各话。
 * 上传中/失败的条目没有语法可比，一律保留。
 */
export function reconcileWithBody(
  items: readonly ImageItem[],
  body: string,
): ImageItem[] {
  return items.filter((item) => {
    if (item.status !== "done") return true;
    const syntax = markdownSyntaxFor(item);
    if (syntax === null) return true;
    return body.includes(syntax);
  });
}
