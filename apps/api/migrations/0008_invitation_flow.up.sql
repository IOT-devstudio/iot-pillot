-- 0008 邀请接受与候选人状态流转（BE-04，issue #90）
--
-- 三张表（依赖 0007 的 recruitment_applications —— 报名申请域由 #88/BE-03 拥有）：
--   recruitment_invitations           —— 邀请（token / 状态 / 过期 / 投递标记）
--   recruitment_candidates            —— 候选人（申请唯一，防并发重复接受）
--   recruitment_candidate_timeline    —— 候选人状态时间线
--
-- 状态机：
--   邀请     pending → accepted | revoked；过期 = pending 且 expires_at < now（查询期判定，无后台任务）
--   候选人   interviewing → offered → hired；interviewing|offered → rejected
--
-- 执行方式（本仓库尚无迁移工具，直接走 psql）：
--   psql "$DSN" -v ON_ERROR_STOP=1 -f 0008_invitation_flow.up.sql
--
-- 幂等写法（IF NOT EXISTS / WHERE NOT EXISTS），重复执行不报错。

BEGIN;
-- ── 邀请 ─────────────────────────────────────────────────────────
-- 一个申请同一时刻至多一条 pending 邀请（部分唯一索引保证）：
-- 重发失败重试复用同一行（email_sent_at 为空即可重投），撤销后才允许再邀。
-- email_sent_at 为 NULL 表示邮件从未投递成功，此时业务状态不算"已邀请"。
CREATE TABLE IF NOT EXISTS recruitment_invitations (
    id               bigserial PRIMARY KEY,
    application_id   bigint      NOT NULL,
    token            text        NOT NULL,
    status           text        NOT NULL DEFAULT 'pending',
    expires_at       timestamptz NOT NULL,
    email_sent_at    timestamptz,
    sent_by          bigint,
    accepted_at      timestamptz,
    created_at       timestamptz,
    updated_at       timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recruitment_invitations_token ON recruitment_invitations (token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_recruitment_invitations_pending_app
    ON recruitment_invitations (application_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_recruitment_invitations_app
    ON recruitment_invitations (application_id);

-- ── 候选人 ───────────────────────────────────────────────────────
-- application_id 唯一：并发接受同一申请的邀请时，第二个事务在此约束上失败，
-- 上层把冲突翻译成"该申请已是候选人"的业务错误（幂等兜底的数据库防线）。
CREATE TABLE IF NOT EXISTS recruitment_candidates (
    id             bigserial PRIMARY KEY,
    application_id bigint      NOT NULL,
    user_id        bigint      NOT NULL,
    status         text        NOT NULL DEFAULT 'interviewing',
    created_at     timestamptz,
    updated_at     timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recruitment_candidates_application
    ON recruitment_candidates (application_id);
CREATE INDEX IF NOT EXISTS idx_recruitment_candidates_user   ON recruitment_candidates (user_id);
CREATE INDEX IF NOT EXISTS idx_recruitment_candidates_status ON recruitment_candidates (status);

-- ── 候选人时间线 ─────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS recruitment_candidate_timeline (
    id          bigserial PRIMARY KEY,
    candidate_id bigint      NOT NULL,
    from_status text,
    to_status   text        NOT NULL,
    note        text,
    operator_id bigint,
    created_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_recruitment_candidate_timeline_candidate
    ON recruitment_candidate_timeline (candidate_id);

COMMIT;
