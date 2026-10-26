-- 0012 资料不合规账号定时冻结
--
-- 执行方式：cd apps/api && go run ./tools/migrate up
--
-- 业务口径（维护者 2026-10-26 裁决）：
--   - 注册满 7 天后资料仍不合规（姓名非全中文 / 班级非「汉字+6位数字」/
--     学号非 9 位数字）→ 发通知邮件并冻结账号。
--   - 冻结后禁止登录，仅管理员可在管理端解冻。
--   - 管理端用户列表默认不显示冻结账号，可用 include_frozen 切换查看。
--
-- 为什么**不复用** 0011 的 deleted_at：
--   deleted_at 是「用户主动注销且不可逆」——它同时驱动登录面排除、邮箱释放
--   （uniq_users_detail_email 的 partial 谓词）与注销通知邮件。冻结是系统判定、
--   可解冻、邮箱仍归本人，两者混用会让上面每条既有语义产生歧义。
--   因此冻结独立成列，与 deleted_at 正交。
--
-- 本迁移只加列与索引，不删不改数据；up/down 可逆。

BEGIN;

-- 冻结时刻。NULL = 未冻结；非 NULL = 冻结生效时间。
ALTER TABLE users ADD COLUMN IF NOT EXISTS frozen_at timestamptz;

COMMENT ON COLUMN users.frozen_at IS
  '冻结标记（0012）：NULL=正常；非 NULL=冻结时刻。登录与刷新令牌均拒绝该账号，管理端列表默认不显示。';

-- 冻结原因快照，例如「班级不是汉字+6位数字」。
-- 存快照而不是每次重算：判定规则将来若调整，已冻结账号的原因不该被悄悄改写。
ALTER TABLE users ADD COLUMN IF NOT EXISTS freeze_reason text;

COMMENT ON COLUMN users.freeze_reason IS
  '冻结原因快照（0012）：写冻结时刻的判定结果，供管理端展示与解冻后审计；解冻时清空。';

-- 通知邮件发出时间，用于扫描幂等：
-- 邮件已发但冻结写失败时，下一轮靠它跳过重复发送（避免一天一封骚扰）。
ALTER TABLE users ADD COLUMN IF NOT EXISTS freeze_notified_at timestamptz;

COMMENT ON COLUMN users.freeze_notified_at IS
  '冻结通知邮件发出时间（0012）：扫描任务据此避免重复发信；解冻时清空。';

-- 部分索引：扫描任务只关心「未冻结」的行，解冻/冻结切换不产生大量索引碎片。
-- 谓词写成 IS NOT NULL，与 deleted_at 的 partial 索引写法保持一致。
CREATE INDEX IF NOT EXISTS idx_users_frozen_at
  ON users (frozen_at)
  WHERE frozen_at IS NOT NULL;

-- 扫描任务的主查询是「未冻结 + 未注销 + 注册已满 7 天」。
-- 现有 idx_users_parse 帮不上忙，这里补一个覆盖 created_at 的 partial 索引，
-- 让定时任务在用户表增长后仍是索引扫描而不是全表扫描。
CREATE INDEX IF NOT EXISTS idx_users_active_created_at
  ON users (created_at)
  WHERE frozen_at IS NULL AND deleted_at IS NULL;

COMMIT;
