# 管理端全屏侧栏与控制台样式升级实施计划

> **For agentic workers:** Execute this plan inline in the current task, following the approved design spec. Keep each change focused and inspect the resulting diff before moving on.

**Goal:** 将管理端管理页改为全屏侧栏布局，并按 B 版方案重排控制台指标和最近动态。

**Architecture:** `App.vue` 按路由 `meta.adminShell` 保持一个共享的 `AdminLayout`，路由切换时折叠状态由布局内的 Vue `ref` 保留。侧栏由单一导航数据源生成，桌面可折叠、窄屏使用抽屉；已有页面负责原业务内容，控制台复用原数据状态与接口。

**Tech Stack:** Vue 3 `<script setup>`、Vue Router 4、Element Plus、原生 CSS。

**Spec:** `docs/superpowers/specs/2026-09-23-admin-console-redesign-design.md`

## Global Constraints

- 不添加新的生产依赖或 Pinia。
- 保持现有 URL、`meta.allowRoles`、业务 API、逐源 loading/error/retry 行为。
- 不更改 3D 登录、普通用户页面或楼栋可视化编辑器。
- 使用 `min-height: 100dvh`，保持小屏无横向溢出和可见键盘焦点。

---

### Task 1: 建立共享管理端布局和响应式侧栏

**Files:**
- Create: `apps/web/src/components/common/AdminLayout.vue`
- Modify: `apps/web/src/App.vue`
- Modify: `apps/web/src/modules/dashboard/routes.ts`
- Modify: `apps/web/src/modules/recruitment/routes.ts`
- Modify: `apps/web/src/router/routes.ts`

- [x] 在布局中以 `ref(false)` 持有 `collapsed` 和 `mobileOpen`，以 `useRoute()` 读取活动页面；管理路由切换时关掉移动抽屉，Escape 也关闭。
- [x] 构建两组现有导航链接：工作台放控制台、意向成员、表单、邮件模板；管理放用户与权限、系统设置。意向成员详情匹配 `/recruitment`，控制台精确匹配 `/dashboard`。
- [x] 实现 246px 展开栏、76px 图标栏、抽屉遮罩、移动端菜单入口、品牌标志、周期信息和全局退出按钮。图标使用同一笔画风格的内联 SVG，不加图标依赖。
- [x] 在 dashboard、recruitment 与占位后台路由上添加 `meta.adminShell: true`；楼栋编辑器维持独立全屏工具。
- [x] 在 `App.vue` 仅对 `adminShell` 路由渲染共享布局，并把页面路由内容放入默认插槽。
- [x] 布局样式定义桌面网格、折叠过渡、移动抽屉、焦点环、减少动画偏好和页面背景层次。

### Task 2: 让现有管理页使用共享内容区

**Files:**
- Modify: `apps/web/src/modules/dashboard/views/AdminMembersView.vue`
- Modify: `apps/web/src/modules/recruitment/views/ProspectsView.vue`
- Modify: `apps/web/src/modules/recruitment/views/ProspectDetailView.vue`
- Modify: `apps/web/src/views/PlaceholderView.vue`
- Modify: `apps/web/src/components/common/AppHeader.vue` (remove if no remaining consumers)

- [x] 删除各页重复的 `AppHeader`、重复退出入口和“返回控制台”导航按钮；管理操作与业务内容原位保留。
- [x] 清除页面自行居中的 1200px 限制，改为占满共享内容区；统一顶部工具行、页面标题间距、窄屏 padding。
- [x] 保持表格、过滤器、详情操作、邮件对话框与占位状态现有交互。
- [x] 删除无引用的旧顶栏组件；意向成员详情保留“返回意向成员列表”。

### Task 3: 重排控制台指标与最近动态

**Files:**
- Modify: `apps/web/src/modules/dashboard/views/DashboardView.vue`

- [x] 移除快捷入口数据与卡片区，顶部改为左侧注册用户主指标、右侧管理员和发信总数小指标加系统状态横条。
- [x] 扩展最近动态区标题；左右并排展示最近用户和最近发信，空状态、重试与加载骨架仍使用原状态对象。
- [x] 使用真实就绪结果表达状态，保留发信总数累计含义；用户入口继续去 `/dashboard/admins`。
- [x] 在 1100px 以下收为单列，在移动宽度缩紧内边距并容纳长时间戳；尊重减少动画偏好。

### Task 4: 集成检查与交付

**Files:**
- Inspect: `apps/web/src/App.vue`, admin routes, all admin views, and `git diff`

- [x] 以仓库 CI 使用的 pnpm 8 运行 web build；pnpm 8.15.9 按锁文件离线恢复依赖，没有改写 lockfile。
- [x] 查看桌面和移动布局的本地页面，检查导航状态、折叠、抽屉关闭、控制台真实状态分支和横向溢出。
- [x] 检查差异只含本任务源文件与计划文件；现有 `.playwright-mcp/`、`.superpowers/`、`artifacts/` 目录没有加入版本控制。
