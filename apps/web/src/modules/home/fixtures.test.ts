/**
 * fixture 自身的守卫测试。
 *
 * 测的不是业务逻辑（fixture 本来就没有逻辑），而是**数据自身的一致性**——
 * 与 opener 模块 buildings.test.ts 的做法一致：将来有人随手加一条方向、
 * 复制粘贴忘了改 id，这里会在 CI 里先拦住，而不是等页面渲染出重复 key。
 */
import { describe, expect, it } from "vitest";

import {
  DEMO_CURRENT_USER,
  MY_APPLICATIONS,
  RECRUITMENT_DIRECTIONS,
  findDirection,
} from "./fixtures";

describe("home fixtures", () => {
  it("keeps direction ids unique", () => {
    const ids = RECRUITMENT_DIRECTIONS.map((direction) => direction.id);

    expect(new Set(ids).size).toBe(ids.length);
  });

  it("gives every direction a non-empty title and summary", () => {
    for (const direction of RECRUITMENT_DIRECTIONS) {
      expect(direction.title.trim()).not.toBe("");
      expect(direction.summary.trim()).not.toBe("");
    }
  });

  it("keeps at least one direction open so the page has something to click", () => {
    // 全关掉的话「报名」按钮整页都是 disabled，演示不出交互
    expect(RECRUITMENT_DIRECTIONS.some((direction) => direction.isOpen)).toBe(
      true,
    );
  });

  it("gives the demo user a username and a role the page can render", () => {
    expect(DEMO_CURRENT_USER.username.trim()).not.toBe("");
    expect(DEMO_CURRENT_USER.role).toBeTruthy();
  });
});

describe("home application fixtures", () => {
  it("keeps application ids unique", () => {
    const ids = MY_APPLICATIONS.map((application) => application.id);

    expect(new Set(ids).size).toBe(ids.length);
  });

  it("points every application at a direction that actually exists", () => {
    // 记录不冗余存标题，标题全靠 directionId 现查——写错一个 id 就会让
    // 列表里那一行静默降级成「（方向已下线）」，所以必须在这里拦住
    for (const application of MY_APPLICATIONS) {
      expect(findDirection(application.directionId)).toBeDefined();
    }
  });

  it("covers more than one status so the colour coding is actually visible", () => {
    const statuses = new Set(MY_APPLICATIONS.map((a) => a.status));

    expect(statuses.size).toBeGreaterThan(1);
  });

  it("returns undefined for an unknown direction id instead of throwing", () => {
    // 降级路径：组件据此显示「（方向已下线）」
    expect(findDirection("direction-does-not-exist")).toBeUndefined();
  });
});
