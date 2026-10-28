-- 0013 down：删除工作室成员表。
-- 注意：若 0014 的 studio_project_members 仍引用本表，须先回滚 0014。

BEGIN;

DROP INDEX IF EXISTS idx_studio_members_source_candidate;
DROP INDEX IF EXISTS idx_studio_members_active;
DROP INDEX IF EXISTS idx_studio_members_direction;
DROP INDEX IF EXISTS idx_studio_members_user_id;
DROP INDEX IF EXISTS idx_studio_members_name_birth_year;
DROP TABLE IF EXISTS studio_members;

COMMIT;
