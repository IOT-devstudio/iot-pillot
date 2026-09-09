# 贡献指南

本项目的所有贡献者（包括 AI 辅助开发）必须遵守本规范。

---

## 合并流程：机器人自动 squash merge

`main` 分支受保护，**禁止人工点 Merge 按钮**。由 GitHub Actions
[`.github/workflows/auto-merge.yml`](./.github/workflows/auto-merge.yml) 自动处理。

### 触发条件

PR 同时满足以下条件时，机器人会自动 squash merge 到 `main`：

1. **作者是组织成员**（`IOT-devstudio`）
2. **无合并冲突**（GitHub 检测 mergeable = true）
3. **非 Draft 状态**（Draft PR 不会被自动合并）
4. **所有 status check 通过**（CI 上线后生效；当前未启用）

不满足条件时，workflow 只 log 原因、不报错。PR 会停留在 Open 状态等人工处理。

### 阻止自动合并

如果某次改动**不希望**自动合并（例如实验性改动、需要外部 reviewer）：

- 创建为 **Draft PR**（机器人跳过）
- 或加 `do-not-auto-merge` label（future：未实现，目前以 Draft 为唯一信号）

### 流程速查

```bash
# 1. 切新分支
git checkout main && git pull
git checkout -b feat/my-change

# 2. 改动后提交（Conventional Commits 格式）
git add .
git commit -m "feat(form): add share link generation"

# 3. 推送并开 PR（非 Draft）
git push -u origin feat/my-change
gh pr create --base main --title "feat(form): add share link generation" --body "..."

# 4. 机器人检查条件：成员 + 无冲突 → 自动 squash merge
```

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
3. **机器人自动合并**：见上方"合并流程"章节
4. **人工干预场景**（机器人条件不满足时）：
   - PR 非成员提交：邀请作者加入组织，或 maintainer 手动 merge
   - PR 有冲突：rebase 或 merge main 到分支后 push

### PR 模板必填项

- 变更类型（feat / fix / docs / ...）
- 变更说明（做什么、为什么）
- 关联 Issue（`Closes #xx` 或 `Refs #xx`）
- 测试情况（新增/更新了哪些测试，本地是否跑通）
- 检查清单（commit 规范、分支命名、文档是否需更新）

---

## 代码审查期望

- 成员身份 ≠ review 通过；成员本身仍然需要写好测试与描述
- 关注点：
  - 可读性、命名清晰度
  - 错误处理是否完整
  - 边界条件、并发安全
  - 测试覆盖
- **不在 review 里夹带私货**：格式、风格让 lint 工具管
- review 意见分级：`nit`（可忽略）/ `suggestion`（建议改）/ `request changes`（必须改）

> 注：当机器人自动合并生效时，"review 通过"维度由作者自觉 + CI 兜底。
> 后续若需强制 review，可在 repo settings 把 `required_approving_review_count` 调回 1。

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

# 3. 推送并开 PR（非 Draft 即可，机器人会自动合并）
git push -u origin feat/my-change
gh pr create --base main --title "feat(form): add share link generation" --body "..."

# 4. 机器人处理：成员 + 无冲突 → squash merge
```
