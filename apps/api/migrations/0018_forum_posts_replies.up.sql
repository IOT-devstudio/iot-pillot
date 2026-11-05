-- 0018: 招新问答 / 内部论坛的内容模型（帖子 + 回复）。
--
-- 决策见 docs/adr/0003-content-scope-shared-model.md：
--   - 一套内容模型用 scope 分区（recruitment | studio），两域共用本表与检索口径；
--   - 帖子与回复是两张表，回复经 post_id 继承内容域，不自引用；
--   - scope 是分区维度不是权限等级：路由分组管"哪个 scope 要什么角色"，
--     service 管"这条记录归不归你"；
--   - 状态列成对出现（resolved_*/deleted_*/edited_at），均不加原因列；
--   - direction_code 软引用招新方向码，不建外键（方向是可关闭运营数据）；
--   - 关键词检索走 ILIKE 扫标题+正文，不建索引（量级小，见 ADR 决策 7）。
--
-- 执行方式：cd apps/api && go run ./tools/migrate up

BEGIN;

CREATE TABLE IF NOT EXISTS forum_posts (
    id             bigserial   PRIMARY KEY,
    scope          text        NOT NULL,
    title          text        NOT NULL DEFAULT '',
    author_id      bigint      NOT NULL,   -- users.id；不强制 FK，避免用户软删牵连
    direction_code text,                    -- 软引用 recruitment_directions.code；留空为合法状态
    body           text        NOT NULL DEFAULT '',
    resolved_at    timestamptz,             -- NULL=未解决
    resolved_by    bigint,                  -- 标记者 users.id；不强制 FK
    deleted_at     timestamptz,             -- NULL=未软删
    deleted_by     bigint,                  -- 删除者 users.id；不强制 FK
    edited_at      timestamptz,             -- 最近编辑时刻；NULL=从未编辑
    created_at     timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_forum_posts_scope
        CHECK (scope IN ('recruitment', 'studio')),
    CONSTRAINT chk_forum_posts_title_len
        CHECK (char_length(title) <= 200),
    CONSTRAINT chk_forum_posts_body_len
        CHECK (char_length(body) <= 20000)
);

-- 列表过滤：覆盖 WHERE (scope = ? AND deleted_at IS NULL) 的等值/半开区间过滤
-- （recruitment 与 studio 各走一个 scope 分组）。排序用的是查询里算出来的
-- last_active_at（= 帖子创建时间或最近一条未删回复时间，取较大者）计算列，
-- 无法用 B-tree 索引覆盖，但每页仅 20~100 行、排序开销可忽略，索引只负责过滤。
CREATE INDEX IF NOT EXISTS idx_forum_posts_list
    ON forum_posts (scope, deleted_at);

-- 方向筛选：只在该具体方向时加条件（"全部"不加）。
CREATE INDEX IF NOT EXISTS idx_forum_posts_direction
    ON forum_posts (scope, direction_code)
    WHERE direction_code IS NOT NULL;

CREATE TABLE IF NOT EXISTS forum_replies (
    id          bigserial   PRIMARY KEY,
    post_id     bigint      NOT NULL,       -- 经 post_id 继承 scope；不强制 FK（软删不牵连）
    author_id   bigint      NOT NULL,
    body        text        NOT NULL DEFAULT '',
    deleted_at  timestamptz,                -- NULL=未软删
    deleted_by  bigint,                     -- 删除者 users.id；不强制 FK
    edited_at   timestamptz,                -- 最近编辑时刻；NULL=从未编辑
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_forum_replies_body_len
        CHECK (char_length(body) <= 20000)
);

-- 详情页拉某帖回复按时间正序。
CREATE INDEX IF NOT EXISTS idx_forum_replies_post
    ON forum_replies (post_id, deleted_at, created_at);

COMMENT ON TABLE forum_posts IS
  '招新问答与内部论坛的帖子（ADR 0003）。scope 分区两域；方向/已解决/软删/编辑均为可选列。';
COMMENT ON COLUMN forum_posts.scope IS '内容域：recruitment=招新问答（登录用户可见），studio=内部论坛（本期仅 admin）。';
COMMENT ON COLUMN forum_posts.direction_code IS '帖子方向（招新方向码）；发帖时显式选或取作者资料方向，留空为合法。';
COMMENT ON COLUMN forum_posts.resolved_at IS '已解决时刻；NULL=未解决。撤销=置 NULL。';

COMMENT ON TABLE forum_replies IS
  '帖子回复（ADR 0003）。经 post_id 继承 scope，无 title/scope/direction/resolved 列。';
COMMENT ON COLUMN forum_replies.post_id IS '所属帖子 forum_posts.id；回复的可见性随帖子 scope 与软删状态。';

COMMIT;
