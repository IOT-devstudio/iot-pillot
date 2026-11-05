import { describe, expect, it } from "vitest";

import type { RoadmapStage } from "@iot-pillot/shared-types";

import {
  MAX_RESOURCES_PER_STAGE,
  MAX_STAGES,
  isAllowedResourceUrl,
  moveResource,
  moveStage,
  newResource,
  newStage,
  validateRoadmapDraft,
} from "./model";

function stage(title: string, id = ""): RoadmapStage {
  return { id, title, summary: "", resources: [] };
}

describe("newStage / newResource", () => {
  it("新阶段的 id 留空，等服务端生成", () => {
    // ADR 0006 决策 3：前端不生成阶段 id。
    // 自造的 id 会被服务端当成伪造而改发新 id，等于白白丢掉「原阶段保留 id」的收益。
    expect(newStage().id).toBe("");
    expect(newStage().resources).toEqual([]);
  });

  it("新资源是空标题与空链接", () => {
    expect(newResource()).toEqual({ title: "", url: "" });
  });
});

describe("moveStage", () => {
  const stages = [stage("一"), stage("二"), stage("三")];

  it("上移与下移交换相邻两项", () => {
    expect(moveStage(stages, 1, -1).map((s) => s.title)).toEqual(["二", "一", "三"]);
    expect(moveStage(stages, 1, 1).map((s) => s.title)).toEqual(["一", "三", "二"]);
  });

  it("在边界处原样返回，不抛错", () => {
    // 按钮在边界会禁用，但键盘或竞态仍可能触达；静默无操作比抛错合适。
    expect(moveStage(stages, 0, -1).map((s) => s.title)).toEqual(["一", "二", "三"]);
    expect(moveStage(stages, 2, 1).map((s) => s.title)).toEqual(["一", "二", "三"]);
  });

  it("不改动入参（渲染依赖原数组不变）", () => {
    const before = stages.map((s) => s.title);
    moveStage(stages, 0, 1);
    expect(stages.map((s) => s.title)).toEqual(before);
  });

  it("重排后阶段 id 跟着走，而不是跟着位置", () => {
    // 这正是「用户进度按 id 记」的价值：把第 2 阶段移到第 1 位，
    // 原来那个阶段的 id 仍然指向它，用户的进度不会静默指到别的阶段。
    const withIds = [stage("一", "s_aaaa1111"), stage("二", "s_bbbb2222")];
    const moved = moveStage(withIds, 1, -1);
    expect(moved.map((s) => s.id)).toEqual(["s_bbbb2222", "s_aaaa1111"]);
    expect(moved[0]!.title).toBe("二");
  });
});

describe("moveResource", () => {
  it("上移与下移交换相邻两项", () => {
    const res = [
      { title: "a", url: "https://a.com" },
      { title: "b", url: "https://b.com" },
    ];
    expect(moveResource(res, 1, -1).map((r) => r.title)).toEqual(["b", "a"]);
    expect(moveResource(res, 0, 1).map((r) => r.title)).toEqual(["b", "a"]);
    expect(moveResource(res, 0, -1).map((r) => r.title)).toEqual(["a", "b"]);
  });
});

describe("isAllowedResourceUrl", () => {
  it("允许 http / https 与站内相对路径", () => {
    expect(isAllowedResourceUrl("https://example.com/a")).toBe(true);
    expect(isAllowedResourceUrl("http://example.com")).toBe(true);
    expect(isAllowedResourceUrl("/user/qa")).toBe(true);
  });

  it("拒绝危险协议与协议相对 URL", () => {
    expect(isAllowedResourceUrl("javascript:alert(1)")).toBe(false);
    expect(isAllowedResourceUrl("data:text/html,<script>")).toBe(false);
    expect(isAllowedResourceUrl("file:///etc/passwd")).toBe(false);
    // "//evil.com" 会跳到外部站点，与「站内相对路径」不是一回事。
    expect(isAllowedResourceUrl("//evil.com/x")).toBe(false);
    expect(isAllowedResourceUrl("")).toBe(false);
    expect(isAllowedResourceUrl("not a url")).toBe(false);
  });
});

describe("validateRoadmapDraft", () => {
  const ok = { title: "前端路线", directionCode: "front-end", stages: [stage("入门")] };

  it("合法草稿通过", () => {
    expect(validateRoadmapDraft(ok)).toBeNull();
  });

  it("标题与方向必填", () => {
    expect(validateRoadmapDraft({ ...ok, title: "  " })?.message).toContain("标题不能为空");
    expect(validateRoadmapDraft({ ...ok, directionCode: "" })?.message).toContain("请选择方向");
  });

  it("阶段上限是 20（边界值通过、21 拒绝）", () => {
    const at = Array.from({ length: MAX_STAGES }, (_, i) => stage(`阶段${i}`));
    expect(validateRoadmapDraft({ ...ok, stages: at })).toBeNull();
    const over = [...at, stage("多一个")];
    expect(validateRoadmapDraft({ ...ok, stages: over })?.message).toContain("最多 20 个");
  });

  it("每个阶段资源上限是 20", () => {
    const res = (n: number) =>
      Array.from({ length: n }, () => ({ title: "r", url: "https://example.com" }));
    const at = [{ ...stage("入门"), resources: res(MAX_RESOURCES_PER_STAGE) }];
    expect(validateRoadmapDraft({ ...ok, stages: at })).toBeNull();

    const over = [{ ...stage("入门"), resources: res(MAX_RESOURCES_PER_STAGE + 1) }];
    expect(validateRoadmapDraft({ ...ok, stages: over })?.message).toContain("最多 20 个资源");
  });

  it("报错用「第 N 个阶段」定位，而不是 id", () => {
    // id 对管理员是不可见的技术细节，用序号才能让人找到对应输入框。
    const stages = [stage("一"), stage("  "), stage("三")];
    const issue = validateRoadmapDraft({ ...ok, stages });
    expect(issue?.where).toBe("第 2 个阶段");
    expect(issue?.message).toContain("第 2 个阶段缺少标题");
  });

  it("非法链接被拦下并说明原因", () => {
    const stages = [
      { ...stage("入门"), resources: [{ title: "坏链接", url: "javascript:alert(1)" }] },
    ];
    const issue = validateRoadmapDraft({ ...ok, stages });
    expect(issue?.message).toContain("不合规");
    expect(issue?.where).toContain("第 1 个资源");
  });

  it("资源缺标题也会被拦下", () => {
    const stages = [{ ...stage("入门"), resources: [{ title: " ", url: "https://a.com" }] }];
    expect(validateRoadmapDraft({ ...ok, stages })?.message).toContain("缺少标题");
  });
});
