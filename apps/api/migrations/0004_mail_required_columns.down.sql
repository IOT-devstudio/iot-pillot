-- 0004 回滚：移除邮件功能依赖的三列
--
-- ⚠️ 会丢弃主题模板、发送时间与直发邮箱，执行前请确认。

BEGIN;

DROP INDEX IF EXISTS idx_mails_created_at;

ALTER TABLE mails DROP COLUMN IF EXISTS to_email;
ALTER TABLE mails DROP COLUMN IF EXISTS created_at;

ALTER TABLE mail_models DROP COLUMN IF EXISTS title;

COMMIT;
