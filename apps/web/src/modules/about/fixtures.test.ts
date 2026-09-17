/**
 * fixture 自身的守卫测试。
 *
 * 测的不是业务逻辑（这些 fixture 本来就没有逻辑），而是**数据自身的一致性**
 * ——与 home / opener 模块的同名测试同一路数：将来有人复制粘贴加一条方向、
 * 忘了改 id，这里会在 CI 里先拦住，而不是等页面渲染出重复 key。
 */
import { describe, expect, it } from "vitest";

import {
  STUDIO_FOCUS,
  STUDIO_INTRO,
  STUDIO_MEMBERS,
  STUDIO_WORKS,
} from "./fixtures";

/** 复制粘贴新增条目时最容易忘改的就是 id */
function expectUniqueIds(items: { id: string }[]): void {
  const ids = items.map((item) => item.id);

  expect(new Set(ids).size).toBe(ids.length);
}

describe("about fixtures", () => {
  it("keeps the intro copy non-empty", () => {
    expect(STUDIO_INTRO.name.trim()).not.toBe("");
    expect(STUDIO_INTRO.tagline.trim()).not.toBe("");
    expect(STUDIO_INTRO.description.trim()).not.toBe("");
  });

  it("gives every stat a label and a value to render", () => {
    expect(STUDIO_INTRO.stats.length).toBeGreaterThan(0);

    for (const stat of STUDIO_INTRO.stats) {
      expect(stat.label.trim()).not.toBe("");
      expect(stat.value.trim()).not.toBe("");
    }
  });

  it("keeps focus ids unique, each focus non-empty, and each stack non-empty", () => {
    expectUniqueIds(STUDIO_FOCUS);

    for (const focus of STUDIO_FOCUS) {
      expect(focus.title.trim()).not.toBe("");
      expect(focus.summary.trim()).not.toBe("");
      // 栈标签整行为空的话页面上会留一块空白
      expect(focus.stack.length).toBeGreaterThan(0);
    }
  });

  it("keeps work ids unique and each work non-empty", () => {
    expectUniqueIds(STUDIO_WORKS);

    for (const work of STUDIO_WORKS) {
      expect(work.title.trim()).not.toBe("");
      expect(work.summary.trim()).not.toBe("");
      expect(work.year.trim()).not.toBe("");
    }
  });

  it("keeps member ids unique and each member non-empty", () => {
    expectUniqueIds(STUDIO_MEMBERS);

    for (const member of STUDIO_MEMBERS) {
      expect(member.name.trim()).not.toBe("");
      expect(member.title.trim()).not.toBe("");
      expect(member.bio.trim()).not.toBe("");
    }
  });

  it("keeps the avatar initial derivable for every member", () => {
    // MemberList 用 name 的首字做文字头像，空名字会让它渲染出一个空圆圈
    for (const member of STUDIO_MEMBERS) {
      expect(member.name.slice(0, 1).trim()).not.toBe("");
    }
  });
});
