-- 0001 回滚：移除招新域字段
--
-- ⚠️ 会丢弃 detail_* 四列与 parse 列的数据，执行前请确认。
--    name 的唯一约束不会被恢复：若原来的重名数据还在，恢复约束会直接失败。
--    需要恢复时请先清理重复的 name，再手工执行：
--      CREATE UNIQUE INDEX idx_users_name ON users (name);

BEGIN;

DROP INDEX IF EXISTS idx_users_detail_student_id;
DROP INDEX IF EXISTS idx_users_detail_email;
DROP INDEX IF EXISTS idx_users_parse;

ALTER TABLE users DROP COLUMN IF EXISTS parse;
ALTER TABLE users DROP COLUMN IF EXISTS detail_email;
ALTER TABLE users DROP COLUMN IF EXISTS detail_direction;
ALTER TABLE users DROP COLUMN IF EXISTS detail_class;
ALTER TABLE users DROP COLUMN IF EXISTS detail_student_id;

COMMIT;
