# 游客模式（Issue #156）

前置后端 #155 已由 PR #170 合入，方向列表公开。只开放前端
`/user/home` 和 `/user/about`；个人设置、邀请、管理页、表单填写继续要求登录。
首页沿用原有版式，游客只读取方向列表，个人数据区保留为空态。

登录页原「创建工作室账号」按钮替换为「先逛逛，了解工作室」入口，
直接进入 `/user/home`；注册仍可通过顶部标签和底部文字入口进入。

报名、我的进度、邀请和个人设置入口共用 `useSignInGate` 与
`SignInPrompt`；确认登录后回跳地址保留当前路径、查询参数及 hash。
顶栏游客入口独立于已登录用户菜单。

管理员从访客首页或关于页登录后进入 `/dashboard`，身份取自本次登录的
`/me` 资料检查。普通用户保留原页、查询参数和 hash；分享表单与个人设置等
具体业务目标继续按原链接返回，并由路由守卫检查权限。

公开页面读取个人数据使用 `requestForPublicPage`：401/403 时清理会话并留在当前页面。
网络或服务异常保留会话并允许重试。其他受保护页面继续使用原有的令牌刷新与跳转策略。
失效会话之前发出的异步响应不能重新填入个人区；其他标签页登录/退出也会同步会话信号。

## 本地验证

从仓库根目录执行：

```powershell
pnpm -r typecheck
pnpm --filter @iot-pillot/web test
pnpm --filter @iot-pillot/web build
pnpm --filter @iot-pillot/web dev --host 127.0.0.1 --port 5186
```

另一终端运行浏览器回归（需要本地 Playwright 和 Edge）：

```powershell
# 若 playwright 不在模块查找路径，设置 PLAYWRIGHT_MODULE 为其绝对路径。
$env:TEST_BASE_URL = 'http://127.0.0.1:5186'
node apps/web/tests/guest-mode.browser.cjs
node apps/web/tests/login-redirect.browser.cjs
```

测试会拦截全部业务 API 请求，使用夹具，不连接真实业务数据。
覆盖桌面 1440px 和 H5 320/375/390/430px、游客零个人请求、
统一提示的取消/Escape/确认、登录回跳、受保护路由、所有首页接口的
401/403 降级、网络异常保留会话，以及 admin/member 报名回归。
这是 Web/H5 检查，不代表微信或原生端验证，也不代表生产已部署。

## 审批检查

1. 未登录打开首页与关于页，查看方向开放状态、个人区空态和顶栏登录入口。
2. 点击方向卡报名、我的进度、邀请与结果、完善资料，确认先出现统一提示；
   关闭后继续停留，确认后进入登录页，登录成功返回原页。
3. 已登录时核对资料、报名记录与报名/面试双路径。

本次交付在本地任务分支，等待用户审批；不自动推送、发布 PR 或更新 Issue。
