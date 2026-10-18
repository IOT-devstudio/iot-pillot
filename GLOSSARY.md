# Glossary

iot-pillot 的领域术语表（single-context 布局，仓库根唯一术语源）。新增或修改术语时在此更新，并保持与 `CLAUDE.md` 的业务模块描述一致。术语缺失时，用 `/domain-modeling` 补充，不要另造项目不用的同义词。

## 招新域

- **表单 (Form)**：管理员创建的报名信息收集器，由若干字段组成，需登录后填写、可分享链接。
- **字段 (Field)**：表单的构成单元。首期支持 `text`、`email`、`textarea`、`select`、`checkbox`。
- **提交 (Submission)**：已登录用户经分享链接填写表单后生成的记录。
- **意向成员 (Prospect)**：提交报名后、尚未接受邀请的潜在成员。
- **候选人 (Candidate)**：接受邀请函后角色推进的人员，状态含 `interviewing` / `offered` / `hired` / `rejected`。
- **招新流程 (Recruitment)**：`Prospect → Candidate → offered/hired` 或 `rejected` 的状态机。
- **申请 (Application)**：用户对某个招新方向的一条报名记录，含提交时的资料快照与审核状态。**报名**是创建申请的动作；直接报名与面试表单提交产生的是同一种申请，同一轮次同一方向至多一条。
- **资料完整 (Profile Completeness)**：班级与学号都已有效填写。判定只有一套规则，报名入口、登录补全提醒与个人设置必须共用，不得按页面另定必填项。「完善资料」指补全缺失项，与「资料为空」不是一回事。

## 邮件域

- **邮件模板 (Mail Template)**：统一配置的可复用邮件正文，类型 `invitation` / `offer` / `rejection` / `custom`；渲染统一走 `TemplateModule`。
- **邀请函 (Invitation)**：发给 Prospect、引导其成为 Candidate 的邮件动作。
- **Offer**：发给通过面试 Candidate 的录用邮件动作。
- **感谢信 (Rejection)**：发给未通过面试 Candidate 的邮件动作。
- **SMTP**：外发邮件传输；所有外发统一走 `MailModule`，业务代码不直接调用 SMTP 库。

## 权限域

- **角色 (Role)**：`admin` / `member` 两级 RBAC；权限判定最终由后端 `AuthModule` 强制，前端仅控制可见性。

## 系统域

- **分享令牌 (Share Token)**：表单分享填写页的定位符（不构成鉴权绕过），对应公开路由 `/f/:shareToken`，提交前须登录。

## 关于页对外口径

公开页面 `/user/about` 的静态内容有自己的用词，避免把「介绍」读成「承诺」：

- **技术主题 (Studio Topic)**：工作室资料（知识库、2025 年宣讲）里讲到的技术内容，如前端开发、后端开发、Agent 开发。**不是**「招新方向」（`RecruitmentDirection`，带开放状态、以招新页为准），也**不是**工作室的正式技术栈。标签位只写「学习内容」。
- **岗位职责 (Studio Role)**：总负责人 / 前端负责人 / 后端负责人各自负责的事务。是岗位，不是现任成员名单；与权限域的 **角色 (Role)**（`admin` / `member`）无关，两者不可混用。
- **成果统计范围 (Achievement Scope)**：成果数字的统计截止口径，当前为「截至 2022 级」。数字必须与它同屏出现。
- **历史项目 (Historical Project)**：资料里只列了名称的项目。其 `source` 是**资料来源时间**（如「2025 年宣讲资料」），不是项目实施年份，页面不得把 `source` 放进「年份」位。

