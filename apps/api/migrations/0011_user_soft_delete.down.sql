-- 0011 用户软删 down：还原邮箱唯一索引为 0005 定义，再删列。
--
-- 注意：还原索引会把「已注销行占用的邮箱」重新纳入唯一性——
-- 若 down 前已用同邮箱重注册（新老两行同邮箱），本步会因唯一冲突失败；
-- 这类数据冲突需人工处理（删行或改邮箱）后再执行 down。

BEGIN;

DROP INDEX IF EXISTS uniq_users_detail_email;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_detail_email
  ON users (detail_email)
  WHERE detail_email IS NOT NULL AND detail_email <> '';

COMMENT ON INDEX uniq_users_detail_email IS
  '邮箱唯一（0005）：仅非空邮箱参与唯一性。';

ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;

COMMIT;
