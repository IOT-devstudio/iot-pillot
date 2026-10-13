-- 0009 回滚：删掉表单模块的三张表。
--
-- 顺序与依赖相反：提交 → 字段 → 表单。
-- 申请域的表（0007 的 recruitment_applications / 0008 的邀请与候选人）不在此回滚 ——
-- 表单模块只借用统一申请模型，不拥有它；回滚表单不该动报名数据。
--
-- 幂等写法（DROP ... IF EXISTS），重复执行不报错。

BEGIN;

DROP TABLE IF EXISTS form_submissions;
DROP TABLE IF EXISTS form_fields;
DROP TABLE IF EXISTS forms;

COMMIT;
