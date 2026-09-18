-- 0005 回滚：恢复 mail_models.type 唯一，去掉邮箱唯一索引
--
-- ⚠️ 恢复 type 唯一索引时，若已存在同类型的多个模板会直接失败。
--    需要恢复时请先清理重复类型，再手工执行：
--      CREATE UNIQUE INDEX idx_mail_models_type ON mail_models (type);

BEGIN;

DROP INDEX IF EXISTS idx_users_name;
DROP INDEX IF EXISTS uniq_users_detail_email;

DROP INDEX IF EXISTS idx_mail_models_type;
CREATE UNIQUE INDEX IF NOT EXISTS idx_mail_models_type ON mail_models (type);

COMMIT;
