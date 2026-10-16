-- 0010 同一轮次同一方向至多一张开放表单（issue #137）
--
-- 0009 只给 forms (round_id, direction_code) 建了**普通**索引
-- （idx_forms_round_direction），于是同一轮同一方向可以存在多张表单，
-- 甚至多张同时 is_open = true。取表规则（#135 D1：is_open + updated_at DESC,
-- id DESC）只让结果**稳定**，没让数据**正确**：多张开放时用户点「面试」跳到
-- 哪张取决于最后修改时间，而其余 share_token 仍是有效提交入口。
--
-- 本迁移把「同时开放」这一种坏状态变成数据库层的不可能：
--   · 部分唯一索引只约束 is_open = true 的行 —— 同轮同方向的**历史（关闭）**
--     表单照旧可以共存，管理员「换张表重做」的正常操作不受影响。
--   · 选部分索引而不是普通唯一索引：后者会禁止同轮同方向建第二张表单，
--     与 0009 注释「排查重复建表」的语义相反。
--
-- 存量数据：**本脚本不替业务删/改数据**（沿用 0005 邮箱唯一立的先例）。
-- 库里若已有同轮同方向多张开放表单，建索引必然失败，因此先检测、把冲突的
-- (round_id, direction_code) 列出来报错退出，由维护者人工决定保留哪张、
-- 关闭其余，然后重跑。这样坏数据不会被静默改写，迁移也不会留下半成品。
--
-- 执行方式（本仓库用 iot-migrate，不再直接走 psql）：
--   cd apps/api && go run ./tools/migrate up
--
-- 幂等写法（IF NOT EXISTS），重复执行不报错。

BEGIN;

-- ── 存量检测：有重复开放行就直接失败，不建索引、不改数据 ──────────────
DO $$
DECLARE
    conflicts text;
BEGIN
    SELECT string_agg(
               format('(round_id=%s, direction_code=%s) 有 %s 张开放表单',
                      round_id, direction_code, total),
               '；' ORDER BY round_id, direction_code)
    INTO conflicts
    FROM (
        SELECT round_id, direction_code, count(*) AS total
        FROM forms
        WHERE is_open
        GROUP BY round_id, direction_code
        HAVING count(*) > 1
    ) AS duplicated;

    IF conflicts IS NOT NULL THEN
        RAISE EXCEPTION
            '同轮同方向存在多张开放表单，请人工关闭多余的表单后重跑本迁移（脚本不会替你改数据）：%',
            conflicts;
    END IF;
END $$;

-- ── 约束：同一轮次同一方向至多一张开放表单 ────────────────────────────
-- 并发下的最终防线：两个管理员同时开放两张表时，第二个事务在此约束上失败，
-- 仓储层把冲突翻译成 ErrFormAlreadyOpen 给调用方（服务层的前置检查只是
-- 提前给出干净错误，不是防线 —— 并发窗口下它看不到对方未提交的行）。
CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_open_uniq
    ON forms (round_id, direction_code)
    WHERE is_open;

COMMENT ON INDEX idx_forms_open_uniq IS '同轮同方向至多一张开放表单（部分唯一索引，issue #137）';

COMMIT;
