-- 0001 users 表补充招新域字段
--
-- 对应 domain/user.go：
--   Detail    Detail `gorm:"embedded;embeddedPrefix:detail_"`  → detail_* 四列
--   Parse     int    `gorm:"column:parse"`                    → parse 一列
--
-- 执行方式（本仓库尚无迁移工具，直接走 psql）：
--   psql "$DSN" -v ON_ERROR_STOP=1 -f 0001_users_recruitment_fields.up.sql
--
-- 注意：本文件为幂等写法（IF NOT EXISTS / DROP ... IF EXISTS），
-- 重复执行不会报错，方便在已有库上补列。

BEGIN;

-- ── 新库 / 首次部署：整表创建 ──────────────────────────────────────
-- 已有库会被下面的 IF NOT EXISTS 跳过。这里的列定义与 domain.User 的
-- GORM 约定保持一致：int → bigint、string → text、time.Time → timestamptz、
-- 非主键列一律可空（GORM 默认不加 NOT NULL）。
CREATE TABLE IF NOT EXISTS users (
    id                 bigserial PRIMARY KEY,
    name               text,
    detail_student_id  bigint,
    detail_class       text,
    detail_direction   text,
    detail_email       text,
    parse              bigint      DEFAULT 0,
    password           text,
    created_at         timestamptz,
    updated_at         timestamptz
);

-- ── 已有库：补列 ──────────────────────────────────────────────────
ALTER TABLE users ADD COLUMN IF NOT EXISTS detail_student_id bigint;
ALTER TABLE users ADD COLUMN IF NOT EXISTS detail_class      text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS detail_direction  text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS detail_email      text;

-- 第几轮招新；0 = 工作室成员（domain/user.go 的约定）。
--
-- DEFAULT 0 是刻意选的：既有行的 parse 会被填成 0，而既有用户本来就是
-- 工作室成员，正好对上，不需要额外回填。
--
-- ⚠️ 副作用：注册流程若忘记显式写 parse，新招进来的人也会落成 0 = 工作室成员。
--    注册时必须显式传入当前招新轮次，这一点在后续实现 register 时要落实。
ALTER TABLE users ADD COLUMN IF NOT EXISTS parse bigint DEFAULT 0;

-- ── 放开 name 的唯一约束 ──────────────────────────────────────────
-- 依据：domain/user.go 里 Name 字段的 uniqueIndex 已被移除。
--
-- ⚠️ 请先确认这是有意为之。name 是登录标识，GetByName 用的是 First()，
--    一旦存在重名，登录会取到"任意一行"，表现为"密码明明对却登不上"。
--    若不打算放开唯一性，请删掉下面两行再执行。
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_name_key;
DROP INDEX IF EXISTS idx_users_name;

-- ── 索引 ─────────────────────────────────────────────────────────
-- 管理端按轮次筛选、按邮箱/学号检索用户时用得上。
CREATE INDEX IF NOT EXISTS idx_users_parse            ON users (parse);
CREATE INDEX IF NOT EXISTS idx_users_detail_email     ON users (detail_email);
CREATE INDEX IF NOT EXISTS idx_users_detail_student_id ON users (detail_student_id);

COMMIT;
