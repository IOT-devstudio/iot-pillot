# Issue #81：瑞士信号 2D 迁移审查

## 范围与固定基准

- Issue：https://github.com/IOT-devstudio/iot-pillot/issues/81。
- 实施基线：远程 main `36af3f978171feb28ef65aedffbcd6deb654821a`；独立分支 `feat/issue-81-swiss-signal`。
- 唯一视觉参考：`7444bf9391c684e0dc781b2fbe2adf0b641293dc` 中的 `apps/web/public/swiss-signal.html` 与配套文件。完整保存于实施分支，仍可独立打开，不依赖原来的本地标签。
- 正式 `/` 和 `/login` 复用 AuthView；`/user/home` 为同风格首页；`/user/settings` 复用既有本人资料编辑与真实 GET/PUT 接口，并统一视觉。
- 未扩展后端字段、招新接口、权限或管理后台，也没有认证绕过、假注册或 Three.js 实验。

## 实现与业务差异

`modules/swiss-signal` 持有局部样式、共享外壳、2D 动效和原生详情弹窗。CSS 选择器统一限定在 `.swiss-signal`，动画名称加前缀，后台继续使用原有样式。外壳持续覆盖认证到普通用户页面的转场；页面切换与卸载清理观察器、动画、计时器、帧任务和事件监听。

保留原型配色、字体、网格、信号星、扫描与圆环、循环字带、遮罩标题、错落入场、磁吸/回弹、滑动切换、导航选中背景、横向方向与章节视差、数字递增、线性进度和弹窗进退。触摸设备禁用磁吸；暂停、减少动态效果、页面隐藏以及动画完成期限均可让业务进入最终状态。

原型的预览首页按钮、任意邮箱认证、假资料保存、找回密码占位入口均未迁移为业务功能。正式登录按既有 username/password 契约；注册仍含姓名、邮箱、6–20 位密码、确认密码和 6 位邮箱验证码。验证码发送/倒计时、错误反馈、重复提交保护、会话保存、站内 redirect 与角色守卫沿用既有实现。真实验证码采用紧凑一行，替代原型的演示同意勾选行，保持初始认证页高度与原型一致；验证通知和字段错误按实际反馈展开。

首页直接读取 `fetchMyProfile()`，不再由旧 `profileFixture` 为用户虚构姓名或资料。接口缺项显示“未填写/未提供”。方向与申请数量来自既有 fixture，状态、详情和本次报名体验均明确标注演示；选择意向没有网络写入或持久化。为保持原型章节顺序，资料概览放入统计条第三栏；申请章节保持一张进度 sheet，原有三条记录均可通过“查看全部 3 条演示记录”或可点击的状态标签打开弹窗选择，没有删除记录；窄屏仍可通过状态标签访问。本人资料链接使用 `/user/settings`，可编辑字段仍仅班级、学号、QQ 和方向。

## 登录挂起的诊断与修复

用户在 `localhost:5173` 报告一直“正在校验”。同一时刻旧 5173 服务的健康检查/GET me 有响应，而登录 POST 分别在 6 秒和 10 秒检查中超时；新标准 Vite 服务 5175 的相同无效测试身份请求能正常收到 401。

查明 5173 的进程是 `node --input-type=module` 自定义预览；5174 属于其他仓库，未干预。只重启 5173 为本仓库标准 Vite 后，登录 POST 正常收到服务器的 `401 / 用户名或密码错误`。未使用用户的账号密码，也未执行真实注册、邮件发送或资料写入。

另为 `requestBackend` 增加 15 秒 AbortController 超时，覆盖请求与 JSON 读取，保留调用者取消信号，结束时释放计时器/监听器。超时提示“请求超时，请检查网络后重试”，提交 finally 解锁按钮。针对挂起请求的测试先复现失败，再实现通过。

## 验证与审查入口

可复现的隔离验收脚本：`apps/web/tests/swiss-signal.browser.cjs`（交互/录屏）、`apps/web/tests/swiss-signal.visual.cjs`（与固定 HTML 同视口的几何、计算样式和截图对照）。需要另行安装 Playwright 与 Edge，并先启动本地 Vite；本项目未新增产品依赖。

```powershell
pnpm --filter @iot-pillot/web dev --host 127.0.0.1 --port 5173 --strictPort
# 另一终端：使 Node 能解析已安装的 playwright（或设置 NODE_PATH）。
node apps/web/tests/swiss-signal.browser.cjs
node apps/web/tests/swiss-signal.visual.cjs
pnpm --filter @iot-pillot/web test -- --maxWorkers=2 --minWorkers=1
pnpm --filter @iot-pillot/web build
pnpm -r typecheck
```

`SIGNAL_ORIGIN` 可覆盖服务地址，`SIGNAL_OUTPUT` 可覆盖输出目录，`BROWSER_CHANNEL` 默认 `msedge`。验收脚本所有业务 API 都通过 Playwright 拦截，不会向已部署后端创建账号、发邮件或保存资料。截图中的身份数据是接口测试 fixture，不能当作真实账号联调结果。

最终验证：30 个 Vitest 文件、278 项测试全部通过；构建、全工作区类型检查通过。Windows 曾发生默认高并发 worker 的 VirtualAlloc 失败，最终使用 1–2 个 worker 运行相同完整测试集。生产构建仍有既有 500KB chunk 提示，未扩展本任务做打包重构。

最终隔离浏览器验收 31 项检查、0 个未捕获错误；视觉脚本完成 2048/1440/768/390/320 宽度 × 登录/注册/首页共 15 组对照，主要区块几何差异不超过 1 像素。

浏览器检查包含真实验证码请求形状/冷却、重复提交保护、safe redirect、admin/member 落点、GET/PUT me 白名单、会话失效、空资料/错误重试、弹窗 Escape/外部关闭/焦点返回、系统减少动态效果、途中暂停、页面隐藏事件模拟、SPA 离开再进入、原生手机触摸滑动、15秒挂起超时，以及斜向光晕实际运动/暂停恢复。拦截响应测试不代替有效账号的真实联调。

输出位于忽略目录 `outputs/issue-81/`：

- `login-desktop.png`、`login-mobile.png`、`register-1440.png`、`register-390.png`。
- `reference-login-desktop.png` 是固定 HTML 原型的同尺寸参考。
- `comparison-{login,register,home}-{reference,current}-{宽度}.png` 是减少动态效果下的同视口静态对照；`style-comparison.json` 记录主要章节几何与计算样式。
- `home-1440.png`、`home-390.png`、`settings-desktop.png`、`settings-mobile.png`。
- `direction-dialog-viewport.png`、`direction-dialog-mobile.png`、`application-dialog.png`。
- `reduced-motion-home.png`、减少动态效果下的弹窗截图。
- `review-desktop.webm`、`review-mobile.webm` 是审查短录屏；`verification-full.webm` 是完整验收过程。
- `browser-report.json` 为逐项浏览器结果，包含明确的模拟响应和隐藏事件模拟口径。

真实服务验证限于健康检查、未授权资料读取和不存在的测试身份登录拒绝；有效账号的真实登录、真实邮箱验证码及真实资料保存未由代理执行。前端成功认证、角色落点、退出、资料读写、失效刷新等通过既有单元测试和拦截响应的浏览器流程验证。

## 独立代码审查

没有 Critical 或 Important 问题。两个 Minor 已修正：资料表单单独展示真实的只读姓名，不把 username 当作姓名；旧 fallback 链接说明当前二维认证，不再声称浏览器当前 WebGL 失败。另补手机申请记录选择入口以及设置页面四宽检查。有效账号的真实成功登录/验证码/资料写入未代理执行，保留明确验证边界。

## 提交与待用户审查

用户已授权提交并创建 PR。远程交付采用 Draft PR，保留审查机会；本仓库非 Draft PR 会立即触发自动合并。合并、部署和 Issue 关闭状态以远程结果为准。

## 随 PR 保存的审查证据

完整、可重复生成的验收输出仍在忽略目录 `outputs/issue-81/`。以下精选证据随仓库保存，便于远程审查；录屏与首页截图使用隔离模拟响应，不包含真实账号资料。

- 同视口登录页：[HTML 基准](issue-81/assets/comparison-login-reference-1440.png)、[Vue 实现](issue-81/assets/comparison-login-current-1440.png)。
- [手机登录页](issue-81/assets/login-mobile.png)、[桌面首页](issue-81/assets/home-1440.png)、[手机首页](issue-81/assets/home-390.png)。
- [桌面短录屏](issue-81/assets/review-desktop.webm)、[手机短录屏](issue-81/assets/review-mobile.webm)。

交付前再次执行 278 项单元测试、全工作区 typecheck 与前端 build，全部通过；构建保留现有大于 500 kB 的分包提醒。