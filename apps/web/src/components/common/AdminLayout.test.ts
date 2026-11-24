import { describe, expect, it } from "vitest";

import layoutSource from "@/components/common/AdminLayout.vue?raw";

/**
 * 管理端左侧导航栏的滚动条。
 *
 * 需求：管理员端左侧导航栏要能滚动——菜单项多于一屏时不能被裁掉。
 *
 * 为什么断言源码而不是挂载组件：AdminLayout 依赖 vue-router、UserMenu 与
 * 大量真实 DOM 交互，为一条「导航区是否可滚」把所有依赖桩掉成本过高；
 * 而这条需求本质就是几条 CSS 声明，直接读样式更准确、更稳。
 */

/** 从 scoped style 中取出 .admin-navigation 的规则体。 */
function ruleBody(selector: string): string {
  const match = new RegExp(`${selector}\\s*\\{([^}]*)\\}`).exec(layoutSource);
  if (!match) return "";
  return match[1]!;
}

describe("管理端左侧导航栏滚动", () => {
  it("导航区自身可纵向滚动", () => {
    const nav = ruleBody(".admin-navigation");
    expect(nav).toMatch(/overflow-y:\s*auto/);
  });

  it("导航区用 min-height: 0 允许在 flex 容器里收缩", () => {
    // 这是关键：flex 子项默认 min-height:auto（按内容撑开），
    // 不置 0 就永远不会收缩，overflow-y 也就永远不触发。
    const nav = ruleBody(".admin-navigation");
    expect(nav).toMatch(/min-height:\s*0/);
    expect(nav).toMatch(/flex:\s*1/);
  });

  it("侧栏仍是固定视口高度，滚动只发生在导航区", () => {
    const sidebar = ruleBody(".admin-sidebar");
    expect(sidebar).toMatch(/height:\s*100dvh/);
    // 整栏 overflow:hidden 保留：滚动交给内部导航区，头部与底部固定。
    expect(sidebar).toMatch(/overflow:\s*hidden/);
  });

  it("头部与底部不参与收缩，滚动时保持可见", () => {
    expect(ruleBody(".admin-sidebar__head")).toMatch(/flex:\s*0\s+0\s+auto/);
    expect(ruleBody(".admin-sidebar__footer")).toMatch(/flex:\s*0\s+0\s+auto/);
  });

  it("提供细滚动条样式，避免默认滚动条挤占窄侧栏", () => {
    expect(layoutSource).toContain(".admin-navigation::-webkit-scrollbar");
    expect(ruleBody(".admin-navigation")).toMatch(/scrollbar-gutter:\s*stable/);
  });
});
