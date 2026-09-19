-- 0003 把 mails 的 from / to 改名为 from_user_id / to_user_id
--
-- 原因：from 与 to 都是 SQL 保留字，任何手写 SQL、BI 工具、导出脚本都必须加双引号
-- （CREATE TABLE 时就得写成 "from"），是个长期的坑。改名后列含义也更直白：存的是 userID。
--
-- GORM 侧同步改了 domain/mail.go 的 column tag，两边必须一起改。
--
-- 执行：psql "$DSN" -v ON_ERROR_STOP=1 -f 0003_rename_mail_user_id_columns.up.sql

BEGIN;

ALTER TABLE mails RENAME COLUMN "from" TO from_user_id;
ALTER TABLE mails RENAME COLUMN "to"   TO to_user_id;

-- 索引会跟着列一起改名（定义里的列名自动更新），但**索引名字**不会变，
-- 顺手改掉，免得日后从索引名反推出一个已不存在的列名。
ALTER INDEX IF EXISTS idx_mails_from RENAME TO idx_mails_from_user_id;
ALTER INDEX IF EXISTS idx_mails_to   RENAME TO idx_mails_to_user_id;

COMMENT ON COLUMN mails.from_user_id IS '发件人 userID（管理端操作者）';
COMMENT ON COLUMN mails.to_user_id   IS '收件人 userID；按邮箱直发且对方未注册时为 0';

COMMIT;
