-- 0006 回滚：移除 QQ 列
--
-- ⚠️ 会丢弃 detail_qq 列的数据，执行前请确认。

BEGIN;

ALTER TABLE users DROP COLUMN IF EXISTS detail_qq;

COMMIT;
