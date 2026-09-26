-- 0006 users 表补充 QQ 字段（issue #57）
--
-- 对应 domain/user.go：
--   Detail.QQ  `gorm:"embedded;embeddedPrefix:detail_"`  → detail_qq 一列
--
-- 执行方式（本仓库尚无迁移工具，直接走 psql）：
--   psql "$DSN" -v ON_ERROR_STOP=1 -f 0006_users_detail_qq.up.sql
--
-- 幂等写法（IF NOT EXISTS），重复执行不会报错，方便在已有库上补列。

BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS detail_qq text;

-- 不建索引：QQ 不参与任何查询路径，个人设置页只按主键读写。

COMMIT;
