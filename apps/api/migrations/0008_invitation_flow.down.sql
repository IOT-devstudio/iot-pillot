-- 0008 邀请接受与候选人状态流转：回滚
-- 顺序与建表相反。只删 BE-04 的三张表；
-- recruitment_applications 属于 #88/BE-03 域，由 0007 的回滚负责，本迁移不碰。

BEGIN;

DROP TABLE IF EXISTS recruitment_candidate_timeline;
DROP TABLE IF EXISTS recruitment_candidates;
DROP TABLE IF EXISTS recruitment_invitations;

COMMIT;
