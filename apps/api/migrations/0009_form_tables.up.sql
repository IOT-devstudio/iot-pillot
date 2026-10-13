-- 0009 表单模块（BE-05，issue #92）
--
-- 三张表。申请域的表**不新建**：提交命中已存在的统一申请模型（#88 的
-- recruitment_applications），表单只负责"要不要收这份答案"与"答案存档"。
--
--   forms             —— 表单定义（绑定一轮招新 + 一个方向 + 分享令牌 + 开放状态）
--   form_fields       —— 表单字段（五种类型；select 的候选项存 jsonb 数组）
--   form_submissions  —— 提交记录（登录用户一条，答案 jsonb，关联申请 id）
--
-- 设计要点（issue #92 维护者拍板）：
--   · 提交强制登录，不存在匿名提交路径；share_token 只是分享定位符，
--     不构成鉴权绕过（读取与提交都挂在 AuthRequired 之后）。
--   · 表单绑定 (round_id, direction_code)：提交即按 #88 的统一申请模型创建申请，
--     于是申请侧唯一约束 (user_id, round_id, direction_code) 继续生效 ——
--     同一轮同一方向不会因为有表单入口就多出第二条申请。
--   · 每用户每表单只允许一条提交：(form_id, submitter_id) 唯一，重复提交返回业务错误。
--   · 答案一律只存档：不回写 users.detail_*、不改账号绑定邮箱。
--
-- 执行方式（本仓库尚无迁移工具，直接走 psql）：
--   psql "$DSN" -v ON_ERROR_STOP=1 -f 0009_form_tables.up.sql
--
-- 幂等写法（IF NOT EXISTS），重复执行不报错。

BEGIN;

-- ── 表单定义 ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS forms (
    id             bigserial PRIMARY KEY,
    title          text    NOT NULL,
    description    text,
    round_id       bigint  NOT NULL,
    direction_code text    NOT NULL,
    share_token    text    NOT NULL,
    is_open        boolean NOT NULL DEFAULT false,
    created_at     timestamptz,
    updated_at     timestamptz
);

-- 分享链接靠 token 定位，必须唯一（服务层生成 128bit 随机串）
CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_share_token ON forms (share_token);
-- 关闭的表单不接受提交，管理端也会按这个状态筛选
CREATE INDEX IF NOT EXISTS idx_forms_open ON forms (is_open);
-- 按轮次/方向核对表单归属与排查重复建表
CREATE INDEX IF NOT EXISTS idx_forms_round_direction ON forms (round_id, direction_code);

-- ── 表单字段 ──────────────────────────────────────────────────────
-- options 只在 field_type = 'select' 时有意义（jsonb 数组）；其他类型为 NULL。
-- 不用 text[] 是因为 select 之外的类型没有候选项，jsonb 的 NULL 语义更直白，
-- 且导出/展示时不必处理数组字面量。
CREATE TABLE IF NOT EXISTS form_fields (
    id          bigserial PRIMARY KEY,
    form_id     bigint  NOT NULL,
    field_type  text    NOT NULL,
    label       text    NOT NULL,
    required    boolean NOT NULL DEFAULT false,
    placeholder text,
    options     jsonb,
    sort_order  bigint  NOT NULL DEFAULT 0,
    created_at  timestamptz,
    updated_at  timestamptz
);

-- 取字段一律按表单 + 人工排序，保证渲染顺序稳定
CREATE INDEX IF NOT EXISTS idx_form_fields_form ON form_fields (form_id, sort_order, id);

-- ── 提交记录 ──────────────────────────────────────────────────────
-- answers 是 字段 id(十进制字符串) → 答案 的 jsonb 对象；checkbox 的答案是字符串数组。
-- application_id 指向 #88 的 recruitment_applications：表单提交与统一申请模型
-- 的关联就落在这里（不产生第二条申请）。
CREATE TABLE IF NOT EXISTS form_submissions (
    id             bigserial PRIMARY KEY,
    form_id        bigint  NOT NULL,
    submitter_id   bigint  NOT NULL,
    application_id bigint,
    answers        jsonb   NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz,
    updated_at     timestamptz
);

-- 防重规则（issue #92 拍板）：同一用户对同一表单只能提交一次
CREATE UNIQUE INDEX IF NOT EXISTS idx_form_submissions_uniq ON form_submissions (form_id, submitter_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_form       ON form_submissions (form_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_submitter  ON form_submissions (submitter_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_application ON form_submissions (application_id);

COMMIT;
