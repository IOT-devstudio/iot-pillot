# 登录注册页设计

## 目标

在 `apps/web` 中把现有 `/login` 占位页替换为可用的登录/注册页面，对接后端已经注册的认证接口。页面采用单页双状态：左侧展示工作室品牌与图案占位，右侧展示完全独立的认证表单。

本次确认的视觉方向为“研究手册”：米白纸张底色、蓝图网格、蓝色主色与少量红色批注感装饰。验证码发送接口暂未在后端路由中注册，因此注册表单保留验证码输入框与按钮，但按钮只展示暂未开放提示，不发起请求。

## 范围

包含：

- `/login` 认证页面；
- 登录与注册模式切换；
- 登录、注册字段校验；
- 对接 `POST /api/v1/login` 与 `POST /api/v1/register`；
- 成功保存 Token 并跳转 `/dashboard`；
- 后端业务错误、网络错误与提交 loading 状态；
- 响应式布局和基础键盘/屏幕阅读器可用性。

不包含：

- 后端验证码发送路由的补充；
- 忘记密码、第三方登录、手机注册；
- `/dashboard` 的真实业务页面；
- 全局鉴权守卫、refresh token 自动续期和退出登录页面；
- 后端或共享 DTO 修改。

## 页面结构

认证页面由两个平行区域组成：

```text
AuthView (/login)
├── BrandPanel (约 52%)
│   ├── 工作室标记占位
│   ├── 蓝图网格 / 几何线稿 / 红色批注装饰
│   └── 工作室宣传短句
└── AuthPanel (约 48%)
    ├── 登录 / 注册切换
    ├── LoginForm 或 RegisterForm
    ├── 提交按钮
    └── 验证码暂未开放提示（仅注册）
```

左侧在窄屏（宽度小于 820px）下隐藏，右侧认证区占满页面并保持可滚动，避免注册字段被截断。页面不使用 Element Plus `el-container` 的后台布局样式，认证区使用自定义 CSS 保持独立视觉。

## 组件与文件边界

建议新增以下文件：

- `apps/web/src/views/AuthView.vue`：页面布局、模式切换、表单状态、校验和提交流程；
- `apps/web/src/api/auth.ts`：认证接口请求函数与响应类型；
- `apps/web/src/auth/session.ts`：Token 保存与读取的最小封装；
- `apps/web/src/views/auth.test.ts` 或同目录测试文件：纯表单行为和请求数据测试。

现有 `apps/web/src/router/routes.ts` 将 `/login` 的组件从 `PlaceholderView` 改为 `AuthView`，其它占位路由不变。`route-specs.ts` 可继续保留 `/login` 的标题声明，但实际组件由路由显式指定，避免认证页依赖占位组件。

## 表单与接口

### 登录

字段：

- `username`：必填；
- `password`：必填，最少 6 位。

提交到：

```text
POST /api/v1/login
Content-Type: application/json
```

```json
{
  "username": "...",
  "password": "..."
}
```

### 注册

字段：

- `name`：必填，3–20 个字符；
- `email`：必填，合法邮箱，最多 50 个字符；
- `password`：必填，6–20 个字符；
- `confirmPassword`：仅前端字段，必须与 `password` 一致；
- `code`：必填，6 位验证码；
提交到：

```text
POST /api/v1/register
Content-Type: application/json
```

```json
{
  "name": "...",
  "password": "...",
  "email": "...",
  "code": "..."
}
```

验证码按钮文案为“获取验证码 · 暂未开放”，始终保持 disabled，不能误调用不存在的 `/api/v1/send-verify-code`。

### 成功响应与会话

登录、注册和刷新接口使用统一响应体。认证页只消费登录/注册响应中的：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "...",
    "refresh_token": "...",
    "user_id": 1
  }
}
```

成功后将 `access_token`、`refresh_token` 和 `user_id` 以对象形式保存到 `localStorage` 的 `iot-pillot.auth` 键中，并通过 Vue Router 跳转 `/dashboard`。不在本次页面中实现自动 refresh 或路由守卫，避免把尚未规划的全局鉴权行为混入认证页。

## 错误处理

- 前端校验失败：在字段下方展示中文校验信息，不发请求；
- HTTP 400/401 且响应可解析：展示后端 `message`；
- 响应格式异常或网络失败：展示“服务暂时不可用，请稍后重试”；
- 提交期间禁用当前提交按钮并显示 loading；
- 切换登录/注册时清理当前模式的字段错误和提交错误，但保留用户已经输入的可复用账号信息；
- 注册验证码未送达时，表单仍可提交给后端，由后端返回验证码错误，前端照常展示后端消息。

## 视觉与交互细节

- 主背景为温暖米白，表单面板为近白色，使用细边界分隔左右区域；
- 主色使用深蓝，提交按钮使用蓝色，红色只用于批注线和错误状态；
- 表单卡片不使用厚重阴影，输入框采用纸张/档案标签的细线风格；
- 首屏进入时品牌区与表单区分段淡入，切换模式使用短距离横向过渡；
- 所有装饰图案使用 CSS 或内联 SVG，不引入外部图片依赖；
- 保持清晰焦点态、可见 label、按钮语义和 `autocomplete` 属性。

## 测试设计

先写失败测试，再实现页面行为：

- `/login` 路由使用 `AuthView`，不再使用占位组件；
- 登录模式渲染用户名和密码字段；
- 注册模式渲染姓名、邮箱、密码、确认密码和验证码字段；
- 空字段、邮箱格式、密码长度、验证码长度和确认密码不一致时阻止提交；
- 登录提交产生正确的 `/api/v1/login` 请求体；
- 注册提交产生正确的 `/api/v1/register` 请求体，且不调用验证码发送接口；
- 成功响应保存会话信息并跳转 `/dashboard`；
- 后端错误消息与网络错误消息可见；
- 提交期间按钮处于 loading 状态并避免重复提交。

验证命令：

```bash
pnpm --filter @iot-pillot/web test
pnpm --filter @iot-pillot/web typecheck
pnpm --filter @iot-pillot/web build
```
