-- 0010 回滚：删掉「同轮同方向至多一张开放表单」的部分唯一索引。
--
-- 只删约束，不动数据：索引存在期间也不会改写过任何行，回滚后同轮同方向
-- 重新允许出现多张开放表单（回到 0009 的语义）。
--
-- 幂等写法（DROP ... IF EXISTS），重复执行不报错。

BEGIN;

DROP INDEX IF EXISTS idx_forms_open_uniq;

COMMIT;
