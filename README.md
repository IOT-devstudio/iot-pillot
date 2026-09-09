# iot-pillot

IoT 全栈开发工作室的后台管理 web 程序。当前阶段：**招新管理**。

- 仓库：https://github.com/IOT-devstudio/iot-pillot
- 组织：[IOT-devstudio](https://github.com/IOT-devstudio)

---

## 项目简介

本系统支撑工作室招新全流程：

- 表单系统（匿名填写 + 分享链接）
- SMTP 邀请流程（prospect → candidate）
- 候选人处理（一键 offer / 感谢信）
- 邮件模板 CRUD
- 管理员 / 成员 RBAC
- 可替换的 SMTP 发件配置

---

## 技术栈

| 层       | 选型                                      |
| -------- | ----------------------------------------- |
| 前端     | Vue 3 + Element Plus + TypeScript         |
| 后端     | Go（Gin + GORM + go-mail + casbin，建议） |
| 数据库   | PostgreSQL                                |
| Monorepo | pnpm workspaces                           |
| CI/CD    | GitHub Actions                            |
| 镜像     | ghcr.io                                   |

完整技术栈决策与理由见 [CLAUDE.md](./CLAUDE.md)。

---

## 快速开始

```bash
# 克隆（推荐 SSH，避免 HTTPS 端口被封）
git clone git@github.com:IOT-devstudio/iot-pillot.git
cd iot-pillot
```

> 脚手架落地后补全 install / dev / build / test 命令。

---

## 贡献流程（速览）

详细规范见 [CONTRIBUTING.md](./CONTRIBUTING.md)。核心规则：

1. **禁止在 `main` 上直接开发**。所有改动走分支 + Pull Request。
2. **分支命名**：`feat/<desc>`、`fix/<desc>`、`docs/<desc>`、`refactor/<desc>`、`test/<desc>`、`chore/<desc>`、`hotfix/<desc>`
3. **Commit 规范**：遵循 [Conventional Commits](https://www.conventionalcommits.org/)——`<type>(<scope>): <subject>`
4. **PR 合并**：至少 1 人 approve，通过 **Squash and merge** 合并到 `main`

---

## 文档索引

- [CLAUDE.md](./CLAUDE.md) — 项目计划、技术栈决策、架构原则、工作约定
- [CONTRIBUTING.md](./CONTRIBUTING.md) — 详细贡献规范：分支策略、commit 规范、PR 流程、review 期望
