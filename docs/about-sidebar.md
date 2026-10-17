# 关于我们侧栏页面

Issue #179 将 `/user/about` 的内容布局迁移到 `docs/demos/about/sidebar.html` 的侧栏方案。正式页面复用 SignalShell 顶栏、返回入口和页脚；页面内部使用 Demo 的编号导航、蓝色简介首屏、SVG 结构图、三段工作流程、技术方向列表和成果/成员空态。

页面继续使用 `apps/web/src/modules/about/fixtures.ts` 的已确认文案和数据。长期技术方向独立于当轮招新方向；成果与成员数组为空时展示待补充状态。没有新增业务请求或身份副作用。

全局 `liquid-glass.css` 中的 `.glass-backdrop` 保持原有固定定位和蓝、黄径向渐变，始终位于侧栏与内容下方，且不拦截指针事件。内容面板采用 Demo 的半透明纸白底，不复制全局 CSS 选择器或 Demo 独立顶栏。

桌面鼠标悬停预览栏目，点击后保持选中；移入内容区不会重置。键盘方向键循环切换，Home/End 跳到首尾。手机端四栏排成顶部横向导航，点击或触摸切换。每个 tab 关联自己的 tabpanel；减少动态效果时栏目过渡关闭。

验证：启动 Vite，设置 `PLAYWRIGHT_MODULE` 指向已安装的 Playwright，然后运行 `node apps/web/tests/about-sidebar.browser.cjs`。可设置 `TEST_BASE_URL` 和 `SCREENSHOT_DIR`。脚本使用模拟身份和 `/me` 响应，检查 1440px 桌面与 320/375/390/430px 手机宽度的栏目交互、键盘、关联关系、光晕、减少动态效果和横向溢出。生产环境与真实 iOS/微信浏览器需另行验收。
