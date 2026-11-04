/**
 * 链接 / 图片地址协议白名单（ADR 0005 决策 5、决策 3 纵深防护）。
 *
 * 链接只允许：http: / https: / 以 "/" 开头的站内相对路径。
 * 图片同样只允许上述范围（外链图是 http(s)），其余协议不渲染、走占位或纯文本。
 *
 * 集中在一处，避免 renderMarkdown 与 MarkdownImage 各写一份导致漂移——
 * 这是本模块「唯一真源」定位的底线。
 */
export function isAllowedUrl(url: string): boolean {
  if (url.startsWith("/")) return true;
  try {
    const u = new URL(url);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}
