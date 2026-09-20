-- 0002 邮件模板与发信记录
--
-- 对应 domain/mail.go：
--   MailModel → mail_models（模板：name / type 唯一，mail_example 示例，mail_model 模板正文）
--   Mail      → mails      （发信记录：title / content / from / to）
--
-- 执行方式：
--   psql "$DSN" -v ON_ERROR_STOP=1 -f 0002_mail_tables.up.sql

BEGIN;

-- ── 邮件模板 ──────────────────────────────────────────────────────
-- mail_model 存模板正文（含占位符），mail_example 存一份填好的示例，
-- 编辑模板时用于预览渲染结果。
CREATE TABLE IF NOT EXISTS mail_models (
    id           bigserial PRIMARY KEY,
    name         text,
    type         text,
    mail_example text,
    mail_model   text
);

-- name 唯一：模板名是管理端的标识，重名会让人分不清选中了哪一个。
CREATE UNIQUE INDEX IF NOT EXISTS idx_mail_models_name ON mail_models (name);

-- ⚠️ type 也建成了唯一索引，依据是 domain.MailModel 上 Type 字段的 uniqueIndex。
--    含义是「每种类型只能有一个模板」（例如 invitation / offer / rejection 各一个）。
--    但 CLAUDE.md 对模板模块的要求是「统一配置，支持扩展新增，支持删除」——
--    若将来要允许同类型多个模板（例如按轮次各存一份邀请函），
--    这里必须换成普通索引，否则第二条同类型模板会插不进去。
--    确认后我可以同步改 domain 的 tag 与这条 DDL。
CREATE UNIQUE INDEX IF NOT EXISTS idx_mail_models_type ON mail_models (type);

-- ── 发信记录 ──────────────────────────────────────────────────────
-- from / to 是 SQL 保留字，必须加双引号。GORM 查询时也会自动加引号，
-- 所以 Go 侧不用改；但手写 SQL 时千万别漏引号。
--
-- ⚠️ 建议把这两列改名成 from_user_id / to_user_id（同时改 domain/mail.go 的
--    gorm column tag）。现在这样能跑，但任何手写 SQL、BI 工具、导出脚本
--    都要记得加引号，是个长期的坑。
CREATE TABLE IF NOT EXISTS mails (
    id      bigserial PRIMARY KEY,
    title   text,
    content text,
    "from"  bigint,
    "to"    bigint
);

CREATE INDEX IF NOT EXISTS idx_mails_to   ON mails ("to");
CREATE INDEX IF NOT EXISTS idx_mails_from ON mails ("from");

COMMIT;
