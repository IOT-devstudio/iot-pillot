-- 0011 用户软删（issue #142：创始管理员注销账号）
--
-- 执行方式：cd apps/api && go run ./tools/migrate up
--
-- 裁决（#142 Q2/Q5，维护者 2026-10-16「按推荐」拍板）：
--   - 注销 = 软删：users 加 deleted_at timestamptz，NULL = 未注销；
--     招新申请 / 表单提交 / 邮件记录按 user_id 保留（历史与审计不丢）。
--   - 注销后同邮箱可重新注册：uniq_users_detail_email 重建为
--     partial 索引，谓词加 AND deleted_at IS NULL。
--     （name 无唯一约束——0001 已 DROP idx_users_name，同名复用天然允许。）
--
-- 本迁移只加列与重建索引，不删数据；up/down 可逆。

BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

COMMENT ON COLUMN users.deleted_at IS
  '软删标记（#142）：NULL=未注销；非 NULL=注销时刻。登录面查询排除该行，管理端列表按此字段标 is_deleted。';

-- 0005 的邮箱唯一索引（空邮箱不冲突的 partial 谓词）扩展软删条件：
-- 已注销行不再占用邮箱，新账号可用同邮箱重新注册。
DROP INDEX IF EXISTS uniq_users_detail_email;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_detail_email
  ON users (detail_email)
  WHERE detail_email IS NOT NULL AND detail_email <> '' AND deleted_at IS NULL;

COMMENT ON INDEX uniq_users_detail_email IS
  '邮箱唯一（#142 Q5）：仅未注销行参与唯一性，注销后同邮箱可重新注册；空邮箱不冲突（沿 0005）。';

COMMIT;
