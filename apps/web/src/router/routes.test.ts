import { describe, expect, it, vi } from "vitest";
// 学习路线用户端视图（#224）同样引入 Element Plus，一并 mock。
vi.mock("@/modules/learning-roadmap/views/LearningPathsView.vue", () => ({
  default: { name: "LearningPathsView" },
}));
vi.mock("@/modules/learning-roadmap/views/LearningPathDetailView.vue", () => ({
  default: { name: "LearningPathDetailView" },
}));
// 学习路线视图（#223）引入 Element Plus，其 theme-chalk CSS 不能在 node 环境解析；
// 路由表测试只关心 path/meta/component，mock 掉组件即可。
vi.mock("@/modules/learning-roadmap/views/LearningRoadmapView.vue", () => ({
  default: { name: "LearningRoadmapView" },
}));
vi.mock("@/modules/recruitment/views/InvitationsView.vue", () => ({
  default: { name: "InvitationsView" },
}));
vi.mock("@/modules/recruitment/views/CandidatesView.vue", () => ({
  default: { name: "CandidatesView" },
}));
vi.mock("@/modules/recruitment/views/CandidateDetailView.vue", () => ({
  default: { name: "CandidateDetailView" },
}));
vi.mock("@/modules/recruitment/views/MyInvitationsView.vue", () => ({
  default: { name: "MyInvitationsView" },
}));

vi.mock("@/modules/dashboard/views/DashboardView.vue", () => ({
  default: { name: "DashboardView" },
}));
vi.mock("@/modules/dashboard/views/AdminMembersView.vue", () => ({
  default: { name: "AdminMembersView" },
}));
vi.mock("@/views/AuthView.vue", () => ({ default: { name: "AuthView" } }));
// opener 视图会在模块作用域引入 three / gsap 并操作 WebGL，测试里必须打桩
vi.mock("@/modules/opener/views/StudioOpener.vue", () => ({
  default: { name: "StudioOpener" },
}));
vi.mock("@/modules/opener-editor/views/BuildingsEditor.vue", () => ({
  default: { name: "BuildingsEditor" },
}));
vi.mock("@/modules/home/views/UserHomeView.vue", () => ({
  default: { name: "UserHomeView" },
}));
vi.mock("@/modules/about/views/AboutView.vue", () => ({
  default: { name: "AboutView" },
}));
vi.mock("@/modules/recruitment/views/ProspectsView.vue", () => ({
  default: { name: "ProspectsView" },
}));
vi.mock("@/modules/recruitment/views/ApplicationsView.vue", () => ({
  default: { name: "ApplicationsView" },
}));
vi.mock("@/modules/recruitment/views/ProspectDetailView.vue", () => ({
  default: { name: "ProspectDetailView" },
}));
// mail 视图引入 Element Plus 弹窗与 composable，测试里只验路由元信息
vi.mock("@/modules/mail/views/MailCenterView.vue", () => ({
  default: { name: "MailCenterView" },
}));
// 个人设置视图在模块作用域 import Element Plus 组件，测试里只验路由元信息
vi.mock("@/modules/user-settings/views/UserSettingsView.vue", () => ({
  default: { name: "UserSettingsView" },
}));
// 工作室成员名录（#195）只验路由元信息，组件不必真挂载
vi.mock("@/modules/studio-members/views/StudioMembersView.vue", () => ({
  default: { name: "StudioMembersView" },
}));
// 工作室知识库（#196）同样只验路由元信息
vi.mock("@/modules/studio-knowledge/views/StudioKnowledgeView.vue", () => ({
  default: { name: "StudioKnowledgeView" },
}));
// 工作室项目管理（#198）同样只验路由元信息
vi.mock("@/modules/studio-projects/views/StudioProjectsView.vue", () => ({
  default: { name: "StudioProjectsView" },
}));
// 招新问答（#219）同样只验路由元信息，组件不必真挂载
vi.mock("@/modules/forum/views/QaListView.vue", () => ({
  default: { name: "QaListView" },
}));
vi.mock("@/modules/forum/views/QaDetailView.vue", () => ({
  default: { name: "QaDetailView" },
}));
// 发帖/编辑页（#220）引入 Element Plus 输入与下拉，测试里只验路由元信息
vi.mock("@/modules/forum/views/QaComposeView.vue", () => ({
  default: { name: "QaComposeView" },
}));

import { routes } from "./routes";

describe("application routes", () => {
  it("内部论坛（#221）复用招新问答组件，靠 meta 切内容域", () => {
    const studioForum = routes.filter((r) => r.path.startsWith("/studio/forum"));

    expect(studioForum.map((r) => r.path)).toEqual([
      "/studio/forum",
      "/studio/forum/new",
      "/studio/forum/:id/edit",
      "/studio/forum/:id",
    ]);

    for (const r of studioForum) {
      // 内部论坛只对工作室成员开放：漏标 allowRoles 就等于对普通成员敞开
      expect(r.meta?.allowRoles).toEqual(["admin"]);
      // 与其它 /studio/* 页一致，套管理台布局
      expect(r.meta?.adminShell).toBe(true);
      expect(r.meta?.forumScope).toBe("studio");
      // AC 明确要求「复用组件，不复制一份实现」
      expect(["QaListView", "QaDetailView", "QaComposeView"]).toContain(
        (r.component as { name?: string })?.name,
      );
    }
  });

  it("学习路线（#223）只对管理员开放并套管理台外壳", () => {
    const roadmap = routes.find((r) => r.path === "/studio/roadmaps");

    expect(roadmap).toBeDefined();
    // 路线维护是运营动作：漏标 allowRoles 就等于对普通成员敞开。
    expect(roadmap?.meta?.allowRoles).toEqual(["admin"]);
    expect(roadmap?.meta?.adminShell).toBe(true);
    expect((roadmap?.component as { name?: string })?.name).toBe("LearningRoadmapView");
  });

  it("招新问答各页标了 recruitment 内容域", () => {
    const qaPages = routes.filter((r) => r.path.startsWith("/user/qa"));

    expect(qaPages.length).toBeGreaterThan(0);
    for (const r of qaPages) {
      expect(r.meta?.forumScope).toBe("recruitment");
      // 招新问答面向全体登录用户，不是 admin 专属
      expect(r.meta?.allowRoles).toEqual(["admin", "member"]);
      // #242：纳入瑞士信号壳，与 /user/home 观感一致（此前是裸渲染的后台风格）
      expect(r.meta?.signalShell).toBe(true);
      // 路径前缀显式声明，管理端入口用同一套组件、另一组前缀
      expect(r.meta?.forumBase).toBe("/user/qa");
    }
  });

  it("管理端招新问答（#242）只回答问题且留在管理台外壳里", () => {
    const adminQa = routes.filter((r) => r.path.startsWith("/recruitment/qa"));

    expect(adminQa.map((r) => r.path)).toEqual([
      "/recruitment/qa",
      "/recruitment/qa/:id/edit",
      "/recruitment/qa/:id",
    ]);

    for (const r of adminQa) {
      // 漏标 allowRoles 就等于对普通用户敞开
      expect(r.meta?.allowRoles).toEqual(["admin"]);
      // 与其它 /recruitment/* 页一致，套管理台布局
      expect(r.meta?.adminShell).toBe(true);
      // 与用户端同一个内容域（接口前缀相同），只是入口与外壳不同
      expect(r.meta?.forumScope).toBe("recruitment");
      // 关键：前缀显式声明为管理端位置，否则「返回列表 / 编辑后跳转」会
      // 把管理员送回 /user/qa，直接跳出管理台外壳。
      expect(r.meta?.forumBase).toBe("/recruitment/qa");
      // 不许再挂 signalShell，否则会同时命中两个外壳
      expect(r.meta?.signalShell).toBeUndefined();
      // AC 明确要求复用组件，不复制一份实现
      expect(["QaListView", "QaDetailView", "QaComposeView"]).toContain(
        (r.component as { name?: string })?.name,
      );
    }

    // 管理端不提供发帖入口：没有 /recruitment/qa/new
    expect(adminQa.some((r) => r.path === "/recruitment/qa/new")).toBe(false);
  });

  it("内部论坛同样声明了显式前缀（#242 后不再靠硬编码推导）", () => {
    const studioForum = routes.filter((r) => r.path.startsWith("/studio/forum"));

    for (const r of studioForum) {
      expect(r.meta?.forumBase).toBe("/studio/forum");
    }
  });

  it("mounts the same 2D auth view on both public login paths", () => {
    const rootRoutes = routes.filter((route) => route.path === "/");

    expect(rootRoutes).toHaveLength(1);
    expect(rootRoutes[0]?.name).toBe("opener");
    expect(rootRoutes[0]?.component).toMatchObject({ name: "AuthView" });
    expect(rootRoutes[0]?.component).toBe(
      routes.find((route) => route.path === "/login")?.component,
    );
    // 开屏对所有人开放，不能挂上任何角色限制
    expect(rootRoutes[0]?.meta?.allowRoles).toEqual([]);
  });

  it("marks the buildings editor as member-only", () => {
    const adminRoutes = routes.filter(
      (route) => route.path === "/admin/buildings",
    );

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.component).toMatchObject({
      name: "BuildingsEditor",
    });
    // 丢了 allowRoles 就等于门禁静默失效（guard 只认这个 meta），必须锁住
    expect(adminRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("mounts the admin dashboard separately from placeholder pages", () => {
    const dashboardRoutes = routes.filter(
      (route) => route.path === "/dashboard",
    );

    expect(dashboardRoutes).toHaveLength(1);
    expect(dashboardRoutes[0]?.component).toMatchObject({
      name: "DashboardView",
    });
    // 管理台含后端健康状态与用户名单，必须是成员专属
    expect(dashboardRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
  });

  it("mounts the admin members page in the settings center (issue #59)", () => {
    const adminRoutes = routes.filter(
      (route) => route.path === "/settings/users",
    );

    expect(adminRoutes).toHaveLength(1);
    expect(adminRoutes[0]?.name).toBe("settings-users");
    expect(adminRoutes[0]?.component).toMatchObject({
      name: "AdminMembersView",
    });
    // 用户与权限页能改角色，漏标 allowRoles 就等于对普通用户敞开
    expect(adminRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
    // 设置中心子页同样挂统一管理外壳
    expect(adminRoutes[0]?.meta?.adminShell).toBe(true);
  });

  it("redirects the settings index and the retired admins path (issue #59)", () => {
    // /settings 索引落到第一个真实区块，避免空落点
    const settingsIndex = routes.filter((route) => route.path === "/settings");
    expect(settingsIndex).toHaveLength(1);
    expect(settingsIndex[0]?.redirect).toBe("/settings/users");

    // 旧路径 /dashboard/admins 保留重定向，收藏夹与旧链接不能变死链
    const retiredAdmins = routes.filter(
      (route) => route.path === "/dashboard/admins",
    );
    expect(retiredAdmins).toHaveLength(1);
    expect(retiredAdmins[0]?.redirect).toBe("/settings/users");
  });

  it("keeps the fallback login page public and retires the old home path", () => {
    const loginRoutes = routes.filter((route) => route.path === "/login");
    expect(loginRoutes).toHaveLength(1);
    expect(loginRoutes[0]?.component).toMatchObject({ name: "AuthView" });
    // 后备登录页必须公开：3D 渲染失败转投过来时，守卫不能把它再挡走
    expect(loginRoutes[0]?.meta?.allowRoles).toEqual([]);

    // 内容首页已并入管理台，旧路径必须重定向过去，否则收藏夹和旧链接会变成死链
    const retiredHome = routes.filter((route) => route.path === "/home");
    expect(retiredHome).toHaveLength(1);
    expect(retiredHome[0]?.redirect).toBe("/dashboard");
  });

  it("publishes home and about while protecting personal user pages", () => {
    const userPages = routes.filter((route) => route.path.startsWith("/user/"));

    expect(userPages.map(({ path, name }) => ({ path, name }))).toEqual([
      { path: "/user/home", name: "user-home" },
      { path: "/user/about", name: "about" },
      { path: "/user/settings", name: "user-settings" },
      { path: "/user/invitations", name: "user-invitations" },
      // 学习路线（#224）在论坛路由之前注册，因此排在 qa 前面。
      { path: "/user/learning-paths", name: "learning-paths" },
      { path: "/user/learning-paths/:id", name: "learning-path-detail" },
      { path: "/user/qa", name: "qa-list" },
      // 发帖与编辑（#220）：/user/qa/new 必须排在 /user/qa/:id 之前，
      // 否则 "new" 会被当成 id 匹配进详情页。
      { path: "/user/qa/new", name: "qa-new" },
      { path: "/user/qa/:id/edit", name: "qa-edit" },
      { path: "/user/qa/:id", name: "qa-detail" },
    ]);
    // 只有首页和介绍公开；个人设置与邀请仍要求登录。
    for (const page of userPages) {
      expect(page.meta?.allowRoles).toEqual(
        ["/user/home", "/user/about"].includes(page.path) ? [] : ["admin", "member"],
      );
    }
  });

  it("学习路线用户端（#224）两个路由都要求登录且不套管理台外壳", () => {
    const paths = routes.filter((r) => r.path.startsWith("/user/learning-paths"));

    expect(paths.map((r) => r.path)).toEqual([
      "/user/learning-paths",
      "/user/learning-paths/:id",
    ]);
    for (const r of paths) {
      // 登录即可（含 admin），漏标 allowRoles 就等于对游客敞开。
      expect(r.meta?.allowRoles).toEqual(["admin", "member"]);
      // 用户端走用户外壳，不是管理台。
      expect(r.meta?.adminShell).toBeUndefined();
    }
  });

  it("mounts the studio member roster as an admin-only page (issue #195)", () => {
    const roster = routes.filter((route) => route.path === "/studio/members");

    expect(roster).toHaveLength(1);
    expect(roster[0]?.name).toBe("studio-members");
    expect(roster[0]?.component).toMatchObject({ name: "StudioMembersView" });
    // 工作室运营首期仅 admin（ADR 0001）；漏标 allowRoles 就等于对普通用户敞开。
    expect(roster[0]?.meta?.allowRoles).toEqual(["admin"]);
    // 成员名录在统一管理外壳里，不另起顶栏。
    expect(roster[0]?.meta?.adminShell).toBe(true);
  });

  it("mounts the studio knowledge base as an admin-only page (issue #196)", () => {
    const knowledge = routes.filter(
      (route) => route.path === "/studio/knowledge",
    );

    expect(knowledge).toHaveLength(1);
    expect(knowledge[0]?.name).toBe("studio-knowledge");
    expect(knowledge[0]?.component).toMatchObject({
      name: "StudioKnowledgeView",
    });
    // 知识库首期仅 admin（ADR 0001 §3）；permission 字段可写但不挂公开读接口。
    expect(knowledge[0]?.meta?.allowRoles).toEqual(["admin"]);
    expect(knowledge[0]?.meta?.adminShell).toBe(true);
  });

  it("mounts the studio project list as an admin-only page (issue #198)", () => {
    const projects = routes.filter((route) => route.path === "/studio/projects");

    expect(projects).toHaveLength(1);
    expect(projects[0]?.name).toBe("studio-projects");
    expect(projects[0]?.component).toMatchObject({ name: "StudioProjectsView" });
    // 项目管理含参与名单，漏标 allowRoles 就等于对普通用户敞开。
    expect(projects[0]?.meta?.allowRoles).toEqual(["admin"]);
    expect(projects[0]?.meta?.adminShell).toBe(true);
  });

  it("mounts the mail center as a member-only real page", () => {
    const mailRoutes = routes.filter((route) => route.path === "/templates");

    expect(mailRoutes).toHaveLength(1);
    expect(mailRoutes[0]?.name).toBe("templates");
    expect(mailRoutes[0]?.component).toMatchObject({ name: "MailCenterView" });
    // 邮件含招新对象联系方式，漏标 allowRoles 就等于对普通用户敞开
    expect(mailRoutes[0]?.meta?.allowRoles).toEqual(["admin"]);
    // 邮件中心也属于统一管理外壳，避免重新引入独立顶栏。
    expect(mailRoutes[0]?.meta?.adminShell).toBe(true);
  });

  it("leaves no placeholder pages behind (issue #59 retired the placeholder system)", () => {
    // /dashboard 与 /settings 都有了真实页面（或重定向），
    // PlaceholderView 已删除；任何回归成占位页的路由都会在这里暴露。
    const placeholderRoutes = routes.filter(
      (route) =>
        typeof route.component === "object" &&
        route.component !== null &&
        "name" in route.component &&
        (route.component as { name?: string }).name === "PlaceholderView",
    );
    expect(placeholderRoutes).toEqual([]);
  });

  it("keeps every real admin page inside the member-only shell", () => {
    // 管理页统一带 adminShell + ["admin"]；漏标任一项都会让
    // 页面脱出管理外壳或对普通用户敞开（守卫只认 allowRoles）。
    // /admin/buildings 是例外：全屏布局编辑器，刻意不挂管理外壳
    // （它有自己的成员专属断言用例），只锁 allowRoles。
    const adminPages = routes.filter(
      (route) =>
        route.component !== undefined &&
        typeof route.path === "string" &&
        (route.path.startsWith("/dashboard") ||
          route.path.startsWith("/settings") ||
          route.path.startsWith("/recruitment") ||
          route.path.startsWith("/forms") ||
          route.path.startsWith("/studio") ||
          route.path === "/templates" ||
          route.path === "/admin/buildings"),
    );
    expect(adminPages.length).toBeGreaterThan(0);
    for (const page of adminPages) {
      expect(page.meta?.allowRoles).toEqual(["admin"]);
      if (page.path !== "/admin/buildings") {
        expect(page.meta?.adminShell).toBe(true);
      }
    }
  });

  it("mounts real forms pages with admin permissions and authenticated share access", () => {
    for (const path of ["/forms", "/forms/new", "/forms/:id/edit", "/forms/:id/submissions"]) {
      const matches = routes.filter(route => route.path === path);
      expect(matches).toHaveLength(1);
      expect(matches[0]?.meta?.allowRoles).toEqual(["admin"]);
      expect(matches[0]?.meta?.adminShell).toBe(true);
      expect(matches[0]?.component).not.toMatchObject({ name: "PlaceholderView" });
    }
    const share = routes.find(route => route.path === "/f/:shareToken");
    expect(share?.meta?.allowRoles).toEqual(["admin", "member"]);
    expect(share?.meta?.signalShell).toBe(true);
  });

  it("registers the recruitment flow routes", () => {
    const recruitmentPaths = routes
      .filter((route) => route.path.startsWith("/recruitment"))
      .map((route) => route.path);

    expect(recruitmentPaths).toEqual([
      "/recruitment",
      "/recruitment/prospects",
      "/recruitment/prospects/:id",
      "/recruitment/applications",
      "/recruitment/invitations",
      "/recruitment/candidates",
      "/recruitment/candidates/:id",
      // 招新问答管理端入口（#242）：加在候选人之后，不打断既有顺序。
      "/recruitment/qa",
      "/recruitment/qa/:id/edit",
      "/recruitment/qa/:id",
    ]);
  });

  it("locks the recruitment pages behind the member role", () => {
    const recruitmentPages = routes.filter(
      (route) => route.path.startsWith("/recruitment") && route.component,
    );

    // 4 个招新流程页 + 3 个问答管理端页（#242）
    expect(recruitmentPages).toHaveLength(7);
    for (const page of recruitmentPages) {
      // 漏标 allowRoles 的页面会被守卫当成公开路由，这里锁死
      expect(page.meta?.allowRoles).toEqual(["admin"]);
    }
  });
});
