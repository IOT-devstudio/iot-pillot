-- 0004 补齐邮件功能成立所必需的三列
--
-- 这三列不是"顺便加的"，缺任何一列对应的功能都不成立，理由逐条写在下面。
-- 若你不同意其中某一列，可以把对应语句删掉再执行——但删掉后我也会把相应的
-- 功能一并去掉，不会留一个写不进去的字段。
--
-- 执行：psql "$DSN" -v ON_ERROR_STOP=1 -f 0004_mail_required_columns.up.sql

BEGIN;

-- ① mail_models.title —— 邮件主题模板
-- 表里原本只有 mail_model（正文）。但 mails 表有 title（实际发出的主题），
-- 说明"发出的邮件是有主题的"；主题如果不来自模板，就只能让调用方每次手填，
-- 那么"统一选用模板发信"这个前提就不成立了（同一模板可以发出各种主题）。
-- 用独立列而不是"正文第一行当主题"这类隐式约定：隐式约定写错了不会报错，
-- 只会让收件人收到一封主题不对的邮件。
ALTER TABLE mail_models ADD COLUMN IF NOT EXISTS title text;
COMMENT ON COLUMN mail_models.title        IS '邮件主题模板，支持 {{变量}} 占位符';
COMMENT ON COLUMN mail_models.mail_model   IS '邮件正文模板，支持 {{变量}} 占位符';
COMMENT ON COLUMN mail_models.mail_example IS '示例：填好变量后的效果，供管理端预览';

-- ② mails.created_at —— 发送时间
-- "发过的邮件记录"如果没有时间，就没法回答"什么时候发的""最近发过谁"，
-- 只能靠 id 大小猜顺序。新增列给 now() 默认值，既有行也能拿到一个值。
ALTER TABLE mails ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now();
COMMENT ON COLUMN mails.created_at IS '发送时间';

-- ③ mails.to_email —— 收件邮箱
-- 需求里有"按邮箱直发"（发给还没注册的报名者）。那种情况下没有 userID，
-- 只记 to_user_id=0 会让这条记录无法追溯发给了谁，等于没记。
-- 按 userID 发送时这一列为空，两种收件人各占一列，语义不重叠。
ALTER TABLE mails ADD COLUMN IF NOT EXISTS to_email text;
COMMENT ON COLUMN mails.to_email IS '收件邮箱；按 userID 发送时为空';

-- 历史列表按时间倒序取，补个索引
CREATE INDEX IF NOT EXISTS idx_mails_created_at ON mails (created_at DESC);

COMMIT;
