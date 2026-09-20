-- 0005 邮箱唯一；mail_models.type 放开唯一约束
--
-- 两处都是"约束与业务语义对齐"的调整，来自两个明确决定：
--   · A6：账号唯一性落在**邮箱**上（用户名允许重复，登录支持用邮箱）
--   · C5：同一类型允许存在多个模板（原来 type 唯一 → 每类型只能一个）
--
-- 执行：psql "$DSN" -v ON_ERROR_STOP=1 -f 0005_email_unique_and_template_type.up.sql

BEGIN;

-- ── ① 邮箱唯一 ────────────────────────────────────────────────────
-- 先检查重复邮箱：有重复就直接报错，**不替业务删数据**。
-- 保留哪一条账号是业务决定（可能牵涉已发出的邮件记录、候选人状态），
-- 脚本自动删掉一条会造成难以察觉的数据丢失。
DO $$
DECLARE
    duplicate_count int;
BEGIN
    SELECT count(*) INTO duplicate_count FROM (
        SELECT detail_email
        FROM users
        WHERE detail_email IS NOT NULL AND detail_email <> ''
        GROUP BY detail_email
        HAVING count(*) > 1
    ) AS duplicated;

    IF duplicate_count > 0 THEN
        RAISE EXCEPTION
            '存在 % 组重复邮箱，请先人工合并账号后再执行本迁移（脚本不会替你删数据）',
            duplicate_count;
    END IF;
END $$;

-- 部分唯一索引：只约束"有邮箱"的行。
-- 用 WHERE 子句而不是普通唯一索引，是因为空邮箱不该互相冲突
-- （NULL 在唯一索引里本来就不比较，但空字符串会）。
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_detail_email
    ON users (detail_email)
    WHERE detail_email IS NOT NULL AND detail_email <> '';

COMMENT ON INDEX uniq_users_detail_email IS '邮箱唯一约束（账号唯一性以此为准）';

-- 用户名不再唯一，但按用户名查询（登录）仍然频繁，补普通索引。
-- 注意：这是**非唯一**索引，不要据此推断用户名唯一。
CREATE INDEX IF NOT EXISTS idx_users_name ON users (name);

-- ── ② mail_models.type 放开唯一 ───────────────────────────────────
-- 改成普通索引：同一类型可以有多个模板（例如每轮招新各存一份邀请函）。
DROP INDEX IF EXISTS idx_mail_models_type;
CREATE INDEX IF NOT EXISTS idx_mail_models_type ON mail_models (type);

COMMENT ON COLUMN mail_models.type IS '模板类型（invitation/offer/rejection/custom），非唯一';

COMMIT;
