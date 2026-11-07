import { beforeEach, describe, expect, it, vi } from "vitest";
import { computed } from "vue";

/**
 * `useForumScope` 的路径派生（#242）。
 *
 * 这里是本次修复最容易被「顺手改回去」的一处：#221 的版本把路径前缀写死成
 * 「域 → 前缀」映射，于是新加的管理端入口（/recruitment/qa）会被送回 /user/qa，
 * 管理员一按「返回列表」就跳出管理台外壳。测试直接喂 meta，不挂真 router——
 * 组件只读 meta，这才是它的契约面。
 */

const routeState = vi.hoisted(() => ({
  meta: {} as Record<string, unknown>,
  path: "/",
}));

vi.mock("vue-router", () => ({
  useRoute: () => routeState,
}));

import { useForumScope } from "../useForumScope";

/** 按给定 meta 求值一组派生路径。 */
function derive(meta: Record<string, unknown>) {
  routeState.meta = meta;
  const forum = useForumScope();
  return {
    scope: forum.scope.value,
    isStudio: forum.isStudio.value,
    isAdminEntry: forum.isAdminEntry.value,
    listPath: forum.listPath.value,
    newPath: forum.newPath.value,
    detailPath: forum.detailPath(42),
    editPath: forum.editPath(42),
    label: forum.label.value,
    description: forum.description.value,
    canCompose: forum.canCompose.value,
  };
}

beforeEach(() => {
  routeState.meta = {};
  routeState.path = "/";
});

describe("useForumScope — 用户端招新问答", () => {
  const meta = {
    forumScope: "recruitment",
    forumBase: "/user/qa",
    allowRoles: ["admin", "member"],
  };

  it("路径全部落在 /user/qa 前缀下", () => {
    const got = derive(meta);
    expect(got.listPath).toBe("/user/qa");
    expect(got.newPath).toBe("/user/qa/new");
    expect(got.detailPath).toBe("/user/qa/42");
    expect(got.editPath).toBe("/user/qa/42/edit");
  });

  it("提供「提问」入口，文案是「提问与查阅」口径", () => {
    const got = derive(meta);
    expect(got.canCompose).toBe(true);
    expect(got.isAdminEntry).toBe(false);
    expect(got.label).toBe("招新问答");
    expect(got.description).toContain("常见问题");
  });
});

describe("useForumScope — 管理端招新问答（#242）", () => {
  const meta = {
    forumScope: "recruitment",
    forumBase: "/recruitment/qa",
    adminShell: true,
    allowRoles: ["admin"],
  };

  it("返回列表 / 编辑后跳转都留在 /recruitment/qa，不跳回用户端", () => {
    // 回归点：按「域 → 前缀」硬编码时这里会得到 /user/qa。
    const got = derive(meta);
    expect(got.listPath).toBe("/recruitment/qa");
    expect(got.detailPath).toBe("/recruitment/qa/42");
    expect(got.editPath).toBe("/recruitment/qa/42/edit");
    expect(got.listPath).not.toContain("/user/");
  });

  it("内容域仍是 recruitment（接口前缀不变，只是入口不同）", () => {
    const got = derive(meta);
    expect(got.scope).toBe("recruitment");
    expect(got.isStudio).toBe(false);
  });

  it("不提供「提问」入口：该页职责是回答问题", () => {
    const got = derive(meta);
    expect(got.canCompose).toBe(false);
    expect(got.isAdminEntry).toBe(true);
    // 文案点出「发新帖请到用户端」，避免管理员以为功能缺失
    expect(got.description).toContain("发新帖请到用户端");
  });
});

describe("useForumScope — 内部论坛", () => {
  const meta = {
    forumScope: "studio",
    forumBase: "/studio/forum",
    adminShell: true,
    allowRoles: ["admin"],
  };

  it("路径落在 /studio/forum，且仍然可以发内部帖", () => {
    const got = derive(meta);
    expect(got.listPath).toBe("/studio/forum");
    expect(got.detailPath).toBe("/studio/forum/42");
    expect(got.newPath).toBe("/studio/forum/new");
    // 内部论坛就是给工作室成员发帖用的，管理台里也不能收起「提问」
    expect(got.canCompose).toBe(true);
    expect(got.isStudio).toBe(true);
  });
});

describe("useForumScope — meta 缺省时的回落", () => {
  it("未标 forumBase 时回落到该域的历史前缀（漏标不改变原有行为）", () => {
    expect(derive({ forumScope: "recruitment" }).listPath).toBe("/user/qa");
    expect(derive({ forumScope: "studio" }).listPath).toBe("/studio/forum");
  });

  it("未标 forumScope 时按对外招新问答处理，不误暴露内部内容", () => {
    const got = derive({});
    expect(got.scope).toBe("recruitment");
    expect(got.isStudio).toBe(false);
  });
});
