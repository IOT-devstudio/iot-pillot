-- 0003 回滚：把列名改回保留字形式
--
-- 注意：改回 "from" / "to" 之后，所有手写 SQL 都必须重新加双引号。

BEGIN;

ALTER INDEX IF EXISTS idx_mails_to_user_id   RENAME TO idx_mails_to;
ALTER INDEX IF EXISTS idx_mails_from_user_id RENAME TO idx_mails_from;

ALTER TABLE mails RENAME COLUMN to_user_id   TO "to";
ALTER TABLE mails RENAME COLUMN from_user_id TO "from";

COMMIT;
