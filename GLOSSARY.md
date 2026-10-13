# Glossary

iot-pillot 的领域术语表（single-context 布局，仓库根唯一术语源）。新增或修改术语时在此更新，并保持与 `CLAUDE.md` 的业务模块描述一致。术语缺失时，用 `/domain-modeling` 补充，不要另造项目不用的同义词。

## 招新域

- **表单 (Form)**：管理员创建的报名信息收集器，由若干字段组成，可匿名填写、可分享链接。
- **字段 (Field)**：表单的构成单元。首期支持 `text`、`email`、`textarea`、`select`、`checkbox`。
- **提交 (Submission)**：匿名申请人经分享链接填写表单后生成的记录。
- **意向成员 (Prospect)**：提交报名后、尚未接受邀请的潜在成员。
- **候选人 (Candidate)**：接受邀请函后角色推进的人员，状态含 `interviewing` / `offered` / `hired` / `rejected`。
- **招新流程 (Recruitment)**：`Prospect → Candidate → offered/hired` 或 `rejected` 的状态机。

## 邮件域

- **邮件模板 (Mail Template)**：统一配置的可复用邮件正文，类型 `invitation` / `offer` / `rejection` / `custom`；渲染统一走 `TemplateModule`。
- **邀请函 (Invitation)**：发给 Prospect、引导其成为 Candidate 的邮件动作。
- **Offer**：发给通过面试 Candidate 的录用邮件动作。
- **感谢信 (Rejection)**：发给未通过面试 Candidate 的邮件动作。
- **SMTP**：外发邮件传输；所有外发统一走 `MailModule`，业务代码不直接调用 SMTP 库。

## 权限域

- **角色 (Role)**：`admin` / `member` 两级 RBAC；权限判定最终由后端 `AuthModule` 强制，前端仅控制可见性。

## 系统域

- **分享令牌 (Share Token)**：表单匿名填写页的访问凭证，对应公开路由 `/f/:shareToken`。
