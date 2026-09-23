/**
 * 搜索命中的字符高亮（issue #72 搜索功能）。
 *
 * 输出「切段」而不是带 <span> 的 HTML 字符串：视图用 v-for 渲染段，
 * 文本永远只作为文本插值——keyword 来自用户输入，走 v-html 就是给自己
 * 开一个存储型 XSS 的口子。
 */

export interface HighlightSegment {
  text: string;
  /** 是否为 keyword 命中段（渲染为标蓝） */
  hit: boolean;
}

/**
 * 把 text 按 keyword 的不区分大小写出现位置切成命中/非命中段。
 * keyword 为空时整段原样返回；text 不含命中时同样只有一段非命中。
 */
export function splitHighlight(text: string, keyword: string): HighlightSegment[] {
  const needle = keyword.trim();
  if (!needle || !text) return [{ text, hit: false }];

  const segments: HighlightSegment[] = [];
  const lowerText = text.toLowerCase();
  const lowerNeedle = needle.toLowerCase();
  let cursor = 0;

  let index = lowerText.indexOf(lowerNeedle);
  while (index !== -1) {
    if (index > cursor) {
      segments.push({ text: text.slice(cursor, index), hit: false });
    }
    segments.push({ text: text.slice(index, index + needle.length), hit: true });
    cursor = index + needle.length;
    index = lowerText.indexOf(lowerNeedle, cursor);
  }
  if (cursor < text.length) {
    segments.push({ text: text.slice(cursor), hit: false });
  }
  return segments;
}
