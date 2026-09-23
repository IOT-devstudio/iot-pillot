import { describe, expect, it } from "vitest";

import { splitHighlight } from "./highlight";

describe("splitHighlight", () => {
  it("returns the whole text untouched when keyword is empty", () => {
    expect(splitHighlight("张三的面试邀请", "")).toEqual([
      { text: "张三的面试邀请", hit: false },
    ]);
    expect(splitHighlight("abc", "   ")).toEqual([{ text: "abc", hit: false }]);
  });

  it("splits hits and gaps in order", () => {
    expect(splitHighlight("欢迎加入，张三", "张三")).toEqual([
      { text: "欢迎加入，", hit: false },
      { text: "张三", hit: true },
    ]);
  });

  it("matches case-insensitively and keeps original casing", () => {
    expect(splitHighlight("Drayee@QQ.com", "drayee")).toEqual([
      { text: "Drayee", hit: true },
      { text: "@QQ.com", hit: false },
    ]);
  });

  it("marks every occurrence", () => {
    expect(splitHighlight("ab ab ab", "ab")).toEqual([
      { text: "ab", hit: true },
      { text: " ", hit: false },
      { text: "ab", hit: true },
      { text: " ", hit: false },
      { text: "ab", hit: true },
    ]);
  });

  it("returns a single non-hit segment when nothing matches", () => {
    expect(splitHighlight("面试邀请", "offer")).toEqual([
      { text: "面试邀请", hit: false },
    ]);
  });

  it("handles consecutive hits without empty gaps", () => {
    expect(splitHighlight("aaa", "aa")).toEqual([
      { text: "aa", hit: true },
      { text: "a", hit: false },
    ]);
  });
});
