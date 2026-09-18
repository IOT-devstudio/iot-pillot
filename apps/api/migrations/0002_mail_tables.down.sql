-- 0002 回滚：移除邮件模板与发信记录
--
-- ⚠️ 会连同模板与发信历史一起丢弃，执行前请确认。

BEGIN;

DROP TABLE IF EXISTS mails;
DROP TABLE IF EXISTS mail_models;

COMMIT;
