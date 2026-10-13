-- 0007 回滚：移除招新模块四张表
--
-- ⚠️ 会连同报名申请与时间线一起丢弃，执行前请确认。
--    种子数据（round_num=1 与三个方向）随之清除，下次 up 会重新插入。

BEGIN;

DROP TABLE IF EXISTS recruitment_application_timeline;
DROP TABLE IF EXISTS recruitment_applications;
DROP TABLE IF EXISTS recruitment_directions;
DROP TABLE IF EXISTS recruitment_rounds;

COMMIT;