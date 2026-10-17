# 关于我们：侧边栏 Demo

保留侧栏布局，主题与当前项目用户侧 Swiss Signal 保持一致。长页面方案及其截图已移除。

## 预览

直接打开 sidebar.html。CSS、JavaScript、SVG 和现有工作室徽标均内嵌，无外部依赖或业务请求。

- 桌面：左侧四栏目，右侧内容展示；支持悬停、点击、方向键和 Home/End。
- 手机：栏目收为顶部导航，点击切换内容。
- 截图：sidebar-desktop.png（1440px）与 sidebar-mobile.png（390px）。

## 主题来源

以 apps/web/src/modules/swiss-signal/signal.css 为准：品牌蓝 #1739f5、荧光黄 #e9ff6c、纸白 #f3f4ed、墨色 #141a32，字体为 Bahnschrift / Microsoft YaHei / PingFang SC，辅助文字使用 Cascadia Code / Consolas。

顶栏复用 apps/web/public/studio-mark.png；首屏使用与项目首页一致的蓝色网格背景，侧栏以品牌蓝标记选中态、荧光黄标记悬停态。保留原 Demo 的内容层级与留白。

## 内容与范围

工作室介绍及三个长期技术方向来自 apps/web/src/modules/about/fixtures.ts。成果与成员保持待补充，没有增加未经确认的年份、人数、项目或人名。本次仅更新 Demo，未接入正式 /user/about 页面。

## 本地验证

Chromium 检查桌面 1440px 与手机 320/375/390/430px 的排版和横向溢出，并验证四栏目点击、悬停、键盘与模拟触屏切换，以及减少动态效果设置。未在真实 iOS / 微信浏览器验收。
