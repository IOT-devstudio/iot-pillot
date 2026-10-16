# 关于我们侧边栏

现有工作室简介、方向/定位、成果、成员四个展示组件保持内容和液态玻璃流光样式，页面改为左侧栏目导航、右侧内容面板。

鼠标悬停栏目时切换内容；移到右侧面板后保持选中。触摸设备点击切换。键盘 Tab 可进入选中栏目，方向键循环选择，Home/End 到首尾，后续 Tab 可进入关联内容区域。栏目和面板通过 tab/tabpanel 语义关联。

保留全局 glass-beam-flow 动画及其 prefers-reduced-motion 降级；内容面板仅增加相邻展开切换，减少动态效果时禁用过渡。没有新增依赖和业务请求，也没有新增未经确认的工作室事实。

浏览器回归：启动 Vite，设置 PLAYWRIGHT_MODULE 指向已安装 Playwright 后运行 `node apps/web/tests/about-sidebar.browser.cjs`。支持 TEST_BASE_URL、SCREENSHOT_DIR。使用模拟身份并拦截所有业务请求，验证 1440px/390px 的悬停、触摸、键盘、内容访问、动画和水平溢出。

需求提及五张卡片，但当前主线只有上述四个组件；第五块内容需维护者确认，未虚构新增内容。
