# FE04：邀请接受、候选人处理与招新结果（#91）

基线：团队仓库 main `2ec5308`。使用主线二维认证页，不引入此前的 3D 视觉实验。本次只改前端，复用 #90 / #99 已交付的后端接口。

## 页面与操作

- `/recruitment/applications`：从真实申请进入该申请的邀请记录。
- `/recruitment/invitations?application_id=42`：查看申请人，选择 invitation 模板发送或重发邀请，查看投递时间、有效期、接受时间，确认撤销待接受邀请。无筛选时显示全部邀请，服务端分页。
- `/recruitment/candidates`：按状态筛选、分页查看候选人。
- `/recruitment/candidates/:id`：查看真实时间线与备注；面试中可发送 Offer 或感谢信，已发 Offer 可确认录用或发送感谢信；终态不展示操作入口。
- `/user/invitations`：查看本人邀请及候选人结果。邮件链接可携带 `?token=...`；未登录时沿用认证守卫保存完整返回地址，登录后返回。打开链接不会自动接受，用户必须主动确认。
- 旧 `/recruitment/prospects` 及详情路径转到真实申请列表；删除旧 fixture、内存 store 和模拟发信/状态流转代码。

## 接口对应

所有请求复用 `session-request` 的鉴权、刷新令牌与错误解包。DTO 复用 shared-types 中 BE04 的 number ID 类型，不使用旧 UUID 草案。所有写请求绑定打开页面时的用户 ID，账号切换后拒绝继续操作。

| 方法与接口                                       | 用途                                         |
| ------------------------------------------------ | -------------------------------------------- |
| GET `/api/v1/admin/recruitment/applications/:id` | 确认邀请对象与申请状态                       |
| GET `/api/v1/admin/mail-templates`               | 选择 invitation / offer / rejection 类型模板 |
| POST `/api/v1/admin/invitations/send`            | 按 application_id 发送邀请                   |
| GET `/api/v1/admin/invitations`                  | 服务端分页、application_id 筛选              |
| POST `/api/v1/admin/invitations/:id/revoke`      | 撤销邀请                                     |
| GET `/api/v1/admin/candidates`                   | 服务端分页、status 筛选                      |
| GET `/api/v1/admin/candidates/:id`               | 候选人详情与时间线                           |
| POST `/api/v1/admin/candidates/:id/advance`      | send_offer / hire / reject                   |
| GET `/api/v1/me/invitations`                     | 本人邀请、服务端过期判定与接受凭据           |
| POST `/api/v1/me/invitations/accept`             | 用户主动接受邀请                             |
| GET `/api/v1/me/candidates`                      | 本人候选人当前结果                           |

邮件通过上述业务接口触发后端 MailModule，前端不另行调用通用发信接口，避免出现“发了信但没流转”的两次请求。预览复用现有沙箱 iframe，变量使用现有转义规则。

## 邮件模板与状态约束

邀请模板可放置 `https://实际部署域名/user/invitations?token={{invitation_token}}`，后台表单展示当前站点对应写法。`invitation_token`、`invitation_expires_at` 由服务端填入，不提供编辑入口。后端不会自动在模板末尾追加链接；旧模板未包含链接时，用户仍可从个人空间的“邀请与结果”进入。

重发复用 pending 邀请且不延长有效期。过期邀请先撤销，再重新发送。`email_sent_at` 为空时明确显示“尚未投递成功”，不把数据库存在记录视为发送成功。

发送、撤销、接受、状态流转都有同步提交锁。邮件失败时不修改候选人状态，保留模板和备注供修正重试。成功后重新查询；若写成功但查询失败，同时保留成功提示与读取错误，用户可单独刷新。提交期间阻止站内导航切换对象。

不属于当前账号或无法匹配的邀请链接显示统一提示，不展示他人信息；过期、撤销、已接受状态没有接受按钮。服务端仍是归属、过期和幂等的最终判定。

## 验证与限制

- `corepack pnpm@8.15.9 -r typecheck`
- `corepack pnpm@8.15.9 --filter @iot-pillot/web test`
- `corepack pnpm@8.15.9 --filter @iot-pillot/web build`
- 浏览器脚本：`apps/web/tests/recruitment.browser.cjs`。运行 Vite 于 5184，设置 `PLAYWRIGHT_MODULE` 为本机 Playwright 模块路径后执行 `node apps/web/tests/recruitment.browser.cjs`。可设置 `TEST_BASE_URL` 和 `SCREENSHOT_DIR`。
- 浏览器检查覆盖 1440px / 390px：邀请发送失败与重试、撤销、Offer、录用、感谢信、接受失败与重试、双击防重、成功后刷新失败、页面刷新、错账号链接、匿名访问后实际填写登录表单并返回邀请页（不自动接受）、成员拒绝访问后台及手机横向溢出。业务请求全部拦截，无真实 SMTP 外发。

用户候选人接口仅提供当前状态、创建与更新时间，没有本人完整候选人时间线接口。用户页展示这些真实时间及邀请时间，管理员详情展示完整服务端时间线，不虚构中间步骤。

尚未用真实管理员/申请人账号和测试邮箱完成生产接口与 SMTP 联调；因此本次不能视为完整招新链路的线上验收。发布前需在测试环境实际走一遍发送邀请、用户接受、发送 Offer/感谢信及录用，并核对邮箱投递和数据库状态。

## 合并前复查

已同步团队主线 `091d703`，保留表单模块、申请审核开关修复和个人设置更新。规范与需求两轴审查未发现本次前端引入的阻塞缺陷；327 项单测、工作区类型检查、生产构建和上述双尺寸浏览器流程通过。

另发现既有后端 `RecruitmentInvitationUseCase.existingCandidateView` 未校验 `userID`：持有已接受邀请 token 的其他登录账号可能读取该候选结果。前端始终从本人邀请列表选择 token，未提供跨账号接受入口；此项属于后端既有缺陷，需要独立修复，不能将前端检查视为服务端鉴权保证。本次不修改后端，也不关闭 #91 的完整验收。
