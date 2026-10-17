# 关于我们侧栏页面

Issue #179 将 `/user/about` 的内容布局迁移到 `docs/demos/about/sidebar.html` 的侧栏方案。正式页面复用 SignalShell 顶栏、返回入口和页脚；页面内部使用 Demo 的编号导航、蓝色简介首屏、SVG 结构图、三段工作流程、技术主题列表和成果统计/岗位分工。

页面使用 `apps/web/src/modules/about/fixtures.ts` 的静态内容，数据口径见 issue #181：名称、成立时间、地点、成果数字、历史项目和岗位职责都来自工作室知识库与 2025 年宣讲资料，资料没给的事实一律不补。技术主题独立于当轮招新方向，本轮可报名的方向仍由招新页面决定；页面不读取任何接口，也没有身份副作用。

四个栏目的口径：

- **工作室简介**：名称 IOT 全栈工作室、成立 2018、地点第十教学楼 B413、累计实战项目 50 余个。数字位下方固定显示「项目数据截至 2022 级（历史统计），不代表当前规模」，不显示当前成员人数或「在做项目数」。
- **技术主题**：前端开发、后端开发、Agent 开发。标签是**学习内容**，不是工作室的正式技术栈；Agent 卡片带可见的「2025 年宣讲主题「后端 AI 开发」」来源说明，并写明它不代表已有 Agent 项目或本轮开放报名。
- **项目成果**：成果统计（国家级奖项 8 项、省部级奖项 25 项、授权专利及软件著作权 5 项、累计实战项目 50 余个）与「截至 2022 级」同屏；四个历史项目只呈现名称与中性的「实施年份与范围待核实」，来源时间用 `source` 字段单列，**不放进「年份」位**，避免被读成项目实施年份。
- **团队分工**：总负责人、前端负责人、后端负责人三个岗位职责卡片，替代原来的空成员列表。没有可公开名册，因此不生成姓名与头像；岗位职责与系统 `admin` / `member` 权限角色无关。

全局 `liquid-glass.css` 中的 `.glass-backdrop` 保持原有固定定位和蓝、黄径向渐变，始终位于侧栏与内容下方，且不拦截指针事件。内容面板采用 Demo 的半透明纸白底，不复制全局 CSS 选择器或 Demo 独立顶栏。

页面主体按 Demo 初始尺寸的 120% 展示，宽度随容器收缩，避免放大后产生横向溢出。桌面鼠标悬停预览栏目，点击后保持选中；移入内容区不会重置。键盘方向键循环切换，Home/End 跳到首尾。手机端四栏排成顶部横向导航，点击或触摸切换。每个 tab 关联自己的 tabpanel；栏目内容按 Demo 渐入，减少动态效果时动画与过渡关闭。

验证：启动 Vite，设置 `PLAYWRIGHT_MODULE` 指向已安装的 Playwright，然后运行 `node apps/web/tests/about-sidebar.browser.cjs`。可设置 `TEST_BASE_URL` 和 `SCREENSHOT_DIR`。脚本使用模拟身份和 `/me` 响应，检查 1440px 桌面与 320/375/390/430px 手机宽度的栏目交互、键盘、关联关系、光晕、减少动态效果和横向溢出，并断言四个栏目的口径（成果统计范围、项目来源时间、Agent 来源说明、页面上不再出现「待补充」）；另以无会话浏览器验证游客可访问简介。生产环境与真实 iOS/微信浏览器需另行验收。

页面对比截图：桌面 [改动前](issues/assets/about-sidebar-before-1440.png) / [改动后](issues/assets/about-sidebar-after-1440.png)，手机 [改动前](issues/assets/about-sidebar-before-390.png) / [改动后](issues/assets/about-sidebar-after-390.png)。#181 后的四个栏目：桌面 [简介](issues/assets/about-content-intro-1440.png)、[技术主题](issues/assets/about-content-focus-1440.png)、[项目成果](issues/assets/about-content-works-1440.png)、[团队分工](issues/assets/about-content-roles-1440.png)，手机 [项目成果](issues/assets/about-content-works-390.png)。
