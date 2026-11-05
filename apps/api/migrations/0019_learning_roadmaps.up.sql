-- 学习路线（#223，ADR 0006）。
--
-- 单表 + JSONB 存阶段与资源：阶段与资源没有独立查询需求
-- （没有页面会问「所有包含某资源链接的阶段」，也没有跨路线资源复用），
-- 它们只作为一条路线的组成部分被整体读写。拆三张表的代价是保存要处理三层 diff、
-- 要防「阶段被删而资源还在」，换来的能力本期用不上。
CREATE TABLE IF NOT EXISTS learning_roadmaps (
    id             bigserial PRIMARY KEY,
    title          text        NOT NULL,
    body           text        NOT NULL DEFAULT '',
    -- 方向是单值软引用：存 recruitment_directions.code，**不建外键**。
    -- 方向表是招新运营数据，路线是长期内容；一次招新结束后方向可能被清理，
    -- 路线不该跟着悬空失败。代价是可能指向已删除的方向——但那是可见的降级
    -- （列表显示 front-end 而不是「前端」），不是静默错误。
    direction_code text        NOT NULL,
    -- 阶段数组：[{ id, title, summary, resources: [{ title, url }] }]。
    -- 数据库只保证「是数组」，逐字段校验在服务层（ADR 0006 决策 1）。
    stages         jsonb       NOT NULL DEFAULT '[]'::jsonb,
    -- 乐观锁：管理端整体提交时带 version，冲突返回可读错误而不是覆盖他人修改。
    version        integer     NOT NULL DEFAULT 1,
    author_id      bigint      NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    -- 只有软删，不做状态机（ADR 0006 决策 5）：
    -- 用户进度行指向已删除的路线，硬删会留下悬空引用；软删后若恢复，进度还在。
    deleted_at     timestamptz,
    deleted_by     bigint,

    -- 只保证形状是数组，元素结构由服务层把关。
    CONSTRAINT learning_roadmaps_stages_is_array
        CHECK (jsonb_typeof(stages) = 'array'),
    CONSTRAINT learning_roadmaps_title_not_blank
        CHECK (length(btrim(title)) > 0),
    CONSTRAINT learning_roadmaps_version_positive
        CHECK (version > 0)
);

-- 列表按「未删 + 方向」筛选，并按更新时间倒序。
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_list
    ON learning_roadmaps (direction_code, updated_at DESC)
    WHERE deleted_at IS NULL;

COMMENT ON TABLE learning_roadmaps IS
    '学习路线（ADR 0006）。阶段与资源以 JSONB 整体读写，无独立查询需求。';
COMMENT ON COLUMN learning_roadmaps.direction_code IS
    '软引用 recruitment_directions.code，不建外键：方向是招新运营数据，路线是长期内容。';
COMMENT ON COLUMN learning_roadmaps.stages IS
    '阶段数组 [{id,title,summary,resources:[{title,url}]}]；id 由服务端生成（s_ + 8 位 base36）。';
COMMENT ON COLUMN learning_roadmaps.version IS
    '乐观锁版本号，每次保存 +1。';
COMMENT ON COLUMN learning_roadmaps.deleted_at IS
    '软删时刻；NULL=有效。不做物理删除，因为用户进度行会指向路线。';
