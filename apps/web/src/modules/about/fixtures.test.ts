/**
 * fixture 自身的守卫测试。
 *
 * 测的不是业务逻辑（这些 fixture 本来就没有逻辑），而是**数据与对外口径的
 * 一致性**——与 home / opener 模块的同名测试同一路数：将来有人复制粘贴加
 * 一条方向、忘了改 id，或者把未经确认的姓名、年份塞回页面，这里会先在 CI
 * 里拦住，而不是等页面渲染出去（issue #181 取代 #94 的「全部待补充」口径）。
 */
import { describe, expect, it } from "vitest";

import {
  ACHIEVEMENT_SCOPE,
  STUDIO_ACHIEVEMENTS,
  STUDIO_FOCUS,
  STUDIO_INTRO,
  STUDIO_ROLES,
  STUDIO_WORKS,
} from "./fixtures";

/** 复制粘贴新增条目时最容易忘改的就是 id */
function expectUniqueIds(items: { id: string }[]): void {
  const ids = items.map((item) => item.id);

  expect(new Set(ids).size).toBe(ids.length);
}

describe("about fixtures", () => {
  it("states the sourced studio facts and no placeholder values (issue #181)", () => {
    expect(STUDIO_INTRO.name).toContain("IOT");
    expect(STUDIO_INTRO.description).toContain("2018");

    const labels = STUDIO_INTRO.stats.map((stat) => stat.label);
    expect(labels).toEqual(["成立", "地点", "累计实战项目"]);

    for (const stat of STUDIO_INTRO.stats) {
      expect(stat.value.trim()).not.toBe("");
      // 「待补充」是 #94 的占位口径，正式资料到位后不应回归
      expect(stat.value).not.toContain("待补充");
    }

    // 50 余个是累计项目，不是在做项目；统计范围必须与数字同屏
    expect(STUDIO_INTRO.statsNote).toContain(ACHIEVEMENT_SCOPE);
  });

  it("keeps the intro copy non-empty", () => {
    expect(STUDIO_INTRO.tagline.trim()).not.toBe("");
  });

  it("keeps focus ids unique, each focus non-empty, and each topic non-empty", () => {
    expectUniqueIds(STUDIO_FOCUS);
    expect(STUDIO_FOCUS.map((focus) => focus.title)).toEqual(["前端开发", "后端开发", "Agent 开发"]);

    for (const focus of STUDIO_FOCUS) {
      expect(focus.summary.trim()).not.toBe("");
      expect(focus.topics.length).toBeGreaterThan(0);

      for (const topic of focus.topics) {
        expect(topic.trim()).not.toBe("");
      }
    }
  });

  it("does not dress repository tech or slide ecosystems up as the studio stack", () => {
    // #181：标签只能是学习内容，不能把 Vue 3 / Go / LLM API 这类仓库技术写成正式技术栈
    const topics = STUDIO_FOCUS.flatMap((focus) => focus.topics);

    for (const forbidden of ["Vue 3", "Vite", "Go", "Gin", "PostgreSQL", "LLM API", "Tool Calling"]) {
      expect(topics).not.toContain(forbidden);
    }
  });

  it("marks the Agent topic as a 2025 talk subject, not a long-term direction", () => {
    const agent = STUDIO_FOCUS.find((focus) => focus.id === "focus-agent");

    expect(agent?.source).toContain("2025");
    // 来源说明必须点名宣讲，避免被读成工作室的长期定位
    expect(agent?.summary).toContain("宣讲");
  });

  it("pins achievement numbers to their 2022-cohort scope", () => {
    expect(ACHIEVEMENT_SCOPE).toBe("截至 2022 级");
    expectUniqueIds(STUDIO_ACHIEVEMENTS);

    const byId = new Map(STUDIO_ACHIEVEMENTS.map((item) => [item.id, item.value]));
    expect(byId.get("award-national")).toBe("8 项");
    expect(byId.get("award-provincial")).toBe("25 项");
    expect(byId.get("intellectual-property")).toBe("5 项");
    expect(byId.get("cumulative-projects")).toBe("50 余个");
  });

  it("keeps work ids unique and every project carrying its source time", () => {
    expectUniqueIds(STUDIO_WORKS);
    expect(STUDIO_WORKS).toHaveLength(4);

    for (const work of STUDIO_WORKS) {
      expect(work.title.trim()).not.toBe("");
      expect(work.summary).toBe("");
      // 来源时间是资料来源时间，不是项目实施年份：字段就叫 source，不叫 year
      expect(work.source.trim()).not.toBe("");
      expect(work).not.toHaveProperty("year");
    }
  });

  it("describes project roles and confirmed current leads", () => {
    expectUniqueIds(STUDIO_ROLES);
    expect(STUDIO_ROLES.map((role) => role.title)).toEqual(["总负责人", "前端负责人", "后端负责人"]);

    for (const role of STUDIO_ROLES) {
      expect(role.summary.trim()).not.toBe("");
      expect(role.duties.length).toBeGreaterThan(0);
      expect(role.members.length).toBeGreaterThan(0);
    }
  });
});
