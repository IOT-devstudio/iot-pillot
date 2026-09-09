# 贡献指南

本项目的所有贡献者（包括 AI 辅助开发）必须遵守本规范。

---

## 铁律：不能在 `main` 上直接开发

所有改动**必须**走分支 + Pull Request 流程：

1. 从最新的 `main` **迁出自己的分支**
2. 在自己的分支上提交、推送
3. **通过 Pull Request** 合并回 `main`
4. PR 合并后由 GitHub 自动 push（或 maintainer 手动 push）

`main` 分支受 GitHub branch protection 保护——直接 push 会被拒绝。

---

## 分支命名规范

格式：`<type>/<短描述>`，全小写、用连字符分隔、描述简洁。

| 前缀 | 用途 | 示例 |
|---|---|---|
| `feat/` | 新功能 | `feat/form-share-link` |
| `fix/` | 缺陷修复 | `fix/email-template-render` |
| `docs/` | 文档变更 | `docs/update-readme` |
| `refactor/` | 重构（无功能变更） | `refactor/extract-mail-module` |
| `test/` | 测试 | `test/auth-guards` |
| `chore/` | 杂项（依赖、CI、配置） | `chore/bump-gorm` |
| `perf/` | 性能优化 | `perf/mail-batch-send` |
| `hotfix/` | 紧急修复（从 main 切，合并后立即发版） | `hotfix/smtp-auth-fail` |

**禁止**：

- 在 `main` 上直接 commit 或 push
- 使用个人姓名 / 拼音作为分支前缀（如 `lx/xxx`）
- 描述过长（控制在 3-5 个单词内）
- 描述含空格或下划线（用连字符）

---

## Commit 规范（Conventional Commits）

格式：

```
<type>(<scope>): <subject>

<body>

<footer>
```

- **type**：与分支前缀保持一致
- **scope**：可选，影响范围。常用：`form` / `template` / `auth` / `mail` / `recruitment` / `ci` / `deps`
- **subject**：祈使句、小写、≤ 50 字符、不加句号
- **body**：可选，解释**为什么**而不是**做了什么**——做了什么看 diff
- **footer**：可选，关联 issue、breaking change、Co-authored-by

### 示例

**简单 commit：**

```
feat(form): support anonymous submission
```

**带 body：**

```
fix(template): handle missing variable substitution

当模板引用未定义的变量时，原实现导致邮件发送失败。
改为占位符留空并记录 warning。
```

**带 footer：**

```
feat(mail): add configurable SMTP retry policy

Closes #42

Co-Authored-By: 张三 <zhangsan@example.com>
```

**Breaking change**（在 type 后加 `!`，并在 footer 写 `BREAKING CHANGE: <说明>`）：

```
feat(api)!: switch auth from session to JWT

BREAKING CHANGE: existing session cookies invalidated; users must re-login.
```

---

## Pull Request 流程

1. **推送分支**：`git push -u origin <branch>`
2. **开 PR**：在 GitHub 上创建 PR，base = `main`，按模板填写
3. **CI 通过**：等待 status check 全绿（CI 配置后启用）
4. **Review 通过**：**至少 1 人 approve**
5. **解决所有 conversation**：所有 review comment 必须 resolve
6. **合并**：使用 **Squash and merge**——保证 `main` 历史每条 PR 一个 commit、整洁

### PR 模板必填项

- 变更类型（feat / fix / docs / ...）
- 变更说明（做什么、为什么）
- 关联 Issue（`Closes #xx` 或 `Refs #xx`）
- 测试情况（新增/更新了哪些测试，本地是否跑通）
- 检查清单（commit 规范、分支命名、文档是否需更新）

---

## 代码审查期望

- **review 是为接手的人服务**，不只是为原作者
- 看 diff 时假设是 5 年后从未看过代码的人在读
- 关注点：
  - 可读性、命名清晰度
  - 错误处理是否完整
  - 边界条件、并发安全
  - 测试覆盖
- **不在 review 里夹带私货**：格式、风格让 lint 工具管，不在 review 里争论
- review 意见分级：`nit`（可忽略）/ `suggestion`（建议改）/ `request changes`（必须改）

---

## 后续会话约定

本项目由 Claude 辅助开发。在新会话开始时，Claude 会自动加载 `CLAUDE.md` 了解项目背景与决策记录。

**遇到不确定的决策时**：

1. 先检查 `CLAUDE.md` 的"已确认"/"已建议"/"待确认"是否已涵盖
2. 若未涵盖，**先问再改**——尤其是技术选型、架构变更、依赖新增
3. 不要在不确定的情况下直接落地代码

---

## 速查卡

```bash
# 1. 切新分支
git checkout main && git pull
git checkout -b feat/my-change

# 2. 改动后提交（Conventional Commits 格式）
git add .
git commit -m "feat(form): add share link generation"

# 3. 推送并开 PR
git push -u origin feat/my-change
gh pr create --base main --title "feat(form): add share link generation" --body "..."

# 4. 等待 review + CI，merge 后由 GitHub 自动 push 到 main
```
