/**
 * 展示用的 ISO-8601 日期格式化。
 *
 * 为什么不用 `new Date(iso).toLocaleDateString()`：
 *   1. 它依赖运行时所在的**时区**与 Node/浏览器的 **ICU 数据**，同一个 ISO 串
 *      在 CI 与本机会算出不同结果，断言会跨机器抖；
 *   2. 它做的是「换算到看的人所在时区」，而报名时间应该显示**记录当时当地
 *      的那一天**——`ISODateTime` 的约定就是一个 ISO-8601 字符串，直接取它的
 *      日期部分才是原样呈现。
 *
 * 所以这里只切字符串：'2026-09-02T20:14:00+08:00' → '2026-09-02'。
 * 认不出日期部分时**原样返回**——宁可显示原始串，也不要显示 'Invalid Date'。
 */
export function formatISODate(iso: string): string {
  const match = /^(\d{4}-\d{2}-\d{2})/.exec(iso);
  return match?.[1] ?? iso;
}
