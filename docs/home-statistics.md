# 首页统计与异步数据

首页开放方向数、本人报名数由 Vue 根据真实接口响应渲染。`useSignalMotion` 只控制容器的入场效果，不直接改写数字文本。

原故障：统计区进入视口时，计数动画捕获尚未加载完成的 0；接口更新为 3 个方向、1 条报名后，动画后续帧仍把 DOM 文本写回 00。下方报名详情正常，但顶部显示 00，容易被误认为报名记录丢失。

回归脚本：`apps/web/tests/home-statistics.browser.cjs`。先启动 Vite 或生产预览，然后设置 `PLAYWRIGHT_MODULE` 为已安装的 Playwright 模块路径，执行 `node apps/web/tests/home-statistics.browser.cjs`。可用 `TEST_BASE_URL` 指定预览地址。

脚本拦截全部业务 API，使用模拟会话与数据，不发送真实报名或邮件。等待统计区进入视口、动画开始后再返回接口数据，检查 1440px/390px、普通/减少动态效果、首次载入/刷新共八种组合，必须显示 03 个开放方向及 01 条报名。
