# Task 2 Report: 认证 API 请求封装

## 改动文件

- `apps/web/src/api/auth.ts`
- `apps/web/src/api/auth.test.ts`
- `.superpowers/sdd/2026-09-16-auth-page/task-2-report.md`

## RED

命令：

```bash
pnpm --filter @iot-pillot/web test -- src/api/auth.test.ts
```

结果：FAIL。`apps/web/src/api/auth.test.ts` 无法加载 `./auth`，符合 brief 预期，因为 `apps/web/src/api/auth.ts` 尚不存在。

关键输出：

```text
FAIL  src/api/auth.test.ts [ src/api/auth.test.ts ]
Error: Failed to load url ./auth ... Does the file exist?
Test Files  1 failed (1)
```

## GREEN

命令：

```bash
pnpm --filter @iot-pillot/web test -- src/api/auth.test.ts
```

结果：PASS。focused test 通过。

关键输出：

```text
✓ src/api/auth.test.ts (5 tests) 7ms
Test Files  1 passed (1)
Tests  5 passed (5)
```

## 实现说明

- 新增 `AuthSession`、`BackendResponse<T>`、`AuthRequestError`。
- 新增私有 `postAuth<T>`，统一设置 JSON 请求头并 `JSON.stringify` payload。
- `login` 请求 `/api/v1/login`。
- `register` 请求 `/api/v1/register`。
- 后端非 2xx、业务 `code !== 0` 或成功响应缺少 `data` 时抛出 `AuthRequestError`。
- 后端错误优先保留 `message`。
- `fetch` reject 或 JSON 解析失败统一转换为 `服务暂时不可用，请稍后重试`。
- 未实现验证码发送请求。

## 顾虑

- 仅按 brief 运行 focused test，未运行全量 web 测试或 build。
- `git status` 输出有用户目录全局 ignore 权限警告：`unable to access 'C:\Users\17818/.config/git/ignore': Permission denied`，不影响本次 focused test。
