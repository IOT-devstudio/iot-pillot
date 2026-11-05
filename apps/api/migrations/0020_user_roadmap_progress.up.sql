-- 用户的学习进度（#224，ADR 0006 决策 4）。
--
-- 一人一路线一行：UNIQUE (user_id, roadmap_id)。
-- 「标记学到这」= UPSERT；再点同一个阶段 = 删掉这一行（等价于撤销）。
--
-- 为什么不塞进 users 表加一列 JSONB：users 已经承载认证、冻结、资料完整性
-- 三套语义，再塞学习进度会污染它，且每次查询都要解析 JSONB。
--
-- stage_id 存阶段的稳定 id（s_ + 8 位 base36），**不是序号**：
-- 管理员重排阶段后进度仍指向原来那个阶段；用序号会静默指到别的阶段。
CREATE TABLE IF NOT EXISTS user_roadmap_progress (
    id         bigserial PRIMARY KEY,
    user_id    bigint      NOT NULL,
    roadmap_id bigint      NOT NULL,
    stage_id   text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT user_roadmap_progress_stage_id_not_blank
        CHECK (length(btrim(stage_id)) > 0)
);

-- 一人一路线只有一行：这是 UPSERT 的落点，也是「进度只属于本人」的结构保证。
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roadmap_progress_unique
    ON user_roadmap_progress (user_id, roadmap_id);

-- 按路线读「我的进度」：列表与详情都会用到。
CREATE INDEX IF NOT EXISTS idx_user_roadmap_progress_user
    ON user_roadmap_progress (user_id);

COMMENT ON TABLE user_roadmap_progress IS
    '用户的学习进度（ADR 0006 决策 4）。一人一路线一行，只属于本人：没有「查看他人进度」的端点。';
COMMENT ON COLUMN user_roadmap_progress.stage_id IS
    '阶段的稳定 id（服务端生成的 s_xxx），不是序号：管理员重排后进度仍指向正确的阶段。';
