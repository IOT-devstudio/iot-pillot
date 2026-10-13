-- 0007 招新模块：轮次 / 方向 / 报名申请 / 时间线
--
-- 对应 domain/recruitment.go 的四张表：
--   recruitment_rounds               —— 招新轮次
--   recruitment_directions           —— 可报名方向（本期只含 front-end/back-end/agent）
--   recruitment_applications         —— 报名申请
--   recruitment_application_timeline —— 状态流转时间线
--
-- 设计要点（issue #88 维护者拍板）：
--   · 状态机 applied → reviewing → accepted | rejected，每次流转追加时间线
--   · 重复报名规则：每轮 (user_id, round_id, direction_code) 唯一，重复提交返回业务错误
--   · 招新方向独立建模，仅开放 front-end/back-end/agent；all/game/other
--     保留为用户资料倾向（users.detail_direction），不在此作为可报名方向
--
-- 执行方式（本仓库尚无迁移工具，直接走 psql）：
--   psql "$DSN" -v ON_ERROR_STOP=1 -f 0007_recruitment_tables.up.sql
--
-- 幂等写法（IF NOT EXISTS / WHERE NOT EXISTS），重复执行不报错，方便在已有库上补表。

BEGIN;

-- ── 招新轮次 ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS recruitment_rounds (
    id         bigserial PRIMARY KEY,
    round_num  bigint      NOT NULL,
    name       text,
    accepting  boolean     NOT NULL DEFAULT false,
    created_at timestamptz,
    updated_at timestamptz
);

-- round_num 是轮的展示标识，不能重复
CREATE UNIQUE INDEX IF NOT EXISTS idx_recruitment_rounds_round_num ON recruitment_rounds (round_num);

-- ── 招新方向 ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS recruitment_directions (
    id         bigserial PRIMARY KEY,
    code       text    NOT NULL,
    name       text,
    open       boolean NOT NULL DEFAULT false,
    sort_order bigint  NOT NULL DEFAULT 0,
    created_at timestamptz,
    updated_at timestamptz
);

-- code 是方向的标识（front-end/back-end/agent），不能重复
CREATE UNIQUE INDEX IF NOT EXISTS idx_recruitment_directions_code ON recruitment_directions (code);

-- ── 报名申请 ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS recruitment_applications (
    id             bigserial PRIMARY KEY,
    user_id        bigint      NOT NULL,
    round_id       bigint      NOT NULL,
    direction_code text        NOT NULL,
    status         text        NOT NULL DEFAULT 'applied',
    name           text,
    email          text,
    class          text,
    student_id     bigint,
    qq             text,
    note           text,
    created_at     timestamptz,
    updated_at     timestamptz
);

-- 每轮同一用户对同一方向只能报名一次（重复报名规则）
CREATE UNIQUE INDEX IF NOT EXISTS idx_recruitment_apps_uniq
    ON recruitment_applications (user_id, round_id, direction_code);

-- 常用查询路径：按用户查本人申请、按轮次/方向/状态筛选
CREATE INDEX IF NOT EXISTS idx_recruitment_apps_user       ON recruitment_applications (user_id);
CREATE INDEX IF NOT EXISTS idx_recruitment_apps_round      ON recruitment_applications (round_id);
CREATE INDEX IF NOT EXISTS idx_recruitment_apps_status     ON recruitment_applications (status);
CREATE INDEX IF NOT EXISTS idx_recruitment_apps_direction  ON recruitment_applications (direction_code);

-- ── 状态流转时间线 ────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS recruitment_application_timeline (
    id             bigserial PRIMARY KEY,
    application_id bigint      NOT NULL,
    to_status      text        NOT NULL,
    created_at     timestamptz
);

CREATE INDEX IF NOT EXISTS idx_recruitment_timeline_app ON recruitment_application_timeline (application_id);

-- ── 种子数据 ──────────────────────────────────────────────────────
-- 首次部署批量插入当前可用数据；幂等用 WHERE NOT EXISTS 避免重复执行时插出重复行。
-- 轮次：第 1 轮招新，默认关闭报名（由管理端打开）。
INSERT INTO recruitment_rounds (round_num, name, accepting)
SELECT 1, '2026 秋季招新', false
WHERE NOT EXISTS (SELECT 1 FROM recruitment_rounds WHERE round_num = 1);

-- 方向：本期只开放三个可报名方向，默认打开；all/game/other 是用户倾向，不在此列。
INSERT INTO recruitment_directions (code, name, open, sort_order)
SELECT * FROM (VALUES
    ('front-end', '前端开发', true,  10),
    ('back-end',  '后端开发', true,  20),
    ('agent',     '智能体',   true,  30)
) AS seed(code, name, open, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM recruitment_directions WHERE code = 'front-end');

COMMIT;