-- 0008 邀请接受与候选人状态流转：回滚
-- 顺序与建表相反；锚点表 recruitment_applications 属于 #88 域，
-- 仅当 #88 的轮次表不存在（即锚点表由本迁移创建）时才一并删除。

BEGIN;

DROP TABLE IF EXISTS recruitment_candidate_timeline;
DROP TABLE IF EXISTS recruitment_candidates;
DROP TABLE IF EXISTS recruitment_invitations;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'recruitment_rounds') THEN
        DROP TABLE IF EXISTS recruitment_applications;
    END IF;
END $$;

COMMIT;
