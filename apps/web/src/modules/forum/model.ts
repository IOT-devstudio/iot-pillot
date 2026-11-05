/**
 * 招新问答的纯展示逻辑（#219）。
 *
 * 抽成纯函数而不是写在 .vue 里：时间格式、官方回答标签与摘要口径要能被单测锁住，
 * 而组件测试需要挂载 Element Plus，成本远高于测一个函数。
 */

import { markdownExcerpt } from "@/components/markdown/markdownExcerpt";

/** ISO 时间 → YYYY-MM-DD HH:mm；解析失败时退回原字符串前 16 位。 */
export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso.slice(0, 16);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** ISO 时间 → YYYY-MM-DD；解析失败时退回原字符串前 10 位。 */
export function formatDay(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso.slice(0, 10);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** 官方回答标签：管理员回复置顶并标「官方」。 */
export function replyRoleLabel(isOfficial: boolean): string {
  return isOfficial ? "官方回答" : "社区回复";
}

/**
 * 帖子正文摘要（列表预览）。先剥 Markdown 标记再截断（ADR 0005 决策 8：
 * 列表不直接渲染 20 条正文 VNode，只给一行纯文本预览）。
 */
export function postExcerpt(body: string, max = 120): string {
  return markdownExcerpt(body, max);
}
