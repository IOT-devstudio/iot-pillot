-- 0012 down：回滚资料不合规账号定时冻结
--
-- 只删本迁移新增的列与索引，不触碰既有 users 数据。
-- 注意：down 会丢失已冻结账号的冻结状态（frozen_at / freeze_reason /
-- freeze_notified_at 三列整体消失），回滚后这些账号重新变为可登录。
-- 生产回滚前请先确认这是预期行为。

BEGIN;

DROP INDEX IF EXISTS idx_users_active_created_at;
DROP INDEX IF EXISTS idx_users_frozen_at;

ALTER TABLE users DROP COLUMN IF EXISTS freeze_notified_at;
ALTER TABLE users DROP COLUMN IF EXISTS freeze_reason;
ALTER TABLE users DROP COLUMN IF EXISTS frozen_at;

COMMIT;
