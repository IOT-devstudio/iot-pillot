import { describe, expect, it } from "vitest";

import homeSource from "./UserHomeView.vue?raw";

/**
 * 个人首页页内导航的入口覆盖。
 *
 * 为什么要有这条测试：#224 把学习路线的用户端页面和路由都做完了，
 * 却漏了「首页导航加一项」这一步（设计文档 2026-11-02 明确要求）。
 * 功能没坏，但除了手输 URL 没人到得了，等于对普通成员隐形。
 *
 * 为什么断言模板源码而不是挂载组件：首页要挂真实 Element Plus 弹窗、
 * 信号转场和三块独立数据，为了一条「导航里有没有这个链接」把所有依赖都
 * 桩掉，成本远高于它锁住的风险。这里要回答的问题很窄——导航里有没有
 * 指向该路径的 <a>，以及点击是否走统一的登录门 + 转场。
 */

/** 页内导航必须覆盖的入口：都在设计文档里点名要求过。 */
const REQUIRED_ENTRIES = [
  { href: "/user/invitations", label: "邀请与结果", handler: "goInvitations" },
  { href: "/user/qa", label: "招新问答", handler: "goQa" },
  { href: "/user/learning-paths", label: "学习路线", handler: "goLearningPaths" },
] as const;

describe("个人首页页内导航", () => {
  it("renders a page-in navigation landmark", () => {
    expect(homeSource).toContain('aria-label="页面内导航"');
  });

  it.each(REQUIRED_ENTRIES)("links $label to $href", ({ href, label, handler }) => {
    // 链接本身：href 指向真实路由，文案让人认得出这是什么。
    expect(homeSource).toContain(`href="${href}"`);
    expect(homeSource).toContain(label);

    // 点击不能是裸跳转：必须走统一的处理函数（登录门 + 擦除转场），
    // 否则未登录用户会经历一次硬跳再被守卫弹回。
    expect(homeSource).toContain(`@click="${handler}"`);
    expect(homeSource).toContain(`function ${handler}(`);
  });

  it("gates the learning-path entry behind sign-in like the other user pages", () => {
    // 与 /user/qa、/user/invitations 同口径：路由是 ANY_SIGNED_IN，
    // 所以点击时先提示登录，别让人到空白页才发现要登录。
    const handler = homeSource.slice(
      homeSource.indexOf("function goLearningPaths("),
      homeSource.indexOf("function goLearningPaths(") + 400,
    );
    expect(handler).toContain("requireSignIn");
    expect(handler).toContain('router.push("/user/learning-paths")');
    expect(handler).toContain("transition");
  });

  it("keeps 学习路线 right after 招新问答 in document order", () => {
    const qaAt = homeSource.indexOf('href="/user/qa"');
    const learningAt = homeSource.indexOf('href="/user/learning-paths"');
    expect(qaAt).toBeGreaterThan(-1);
    expect(learningAt).toBeGreaterThan(qaAt);
  });
});
