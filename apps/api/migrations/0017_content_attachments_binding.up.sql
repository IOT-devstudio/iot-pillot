-- 0017: content_attachments 归属绑定列 + 孤儿约束。
--
-- #216 只落孤儿列；#217 补三列归属（post_id / reply_id / roadmap_id），
-- 让附件能挂到招新问答帖子 / 内部论坛内容 / 学习路线正文里（具体绑定
-- 由 #218 / #223 在保存时通过正文正则 /api/v1/attachments/(\d+) 完成）。
-- CHECK 保证至多一列非空；部分索引只覆盖"已绑定"行，孤儿清理靠三列全空。
BEGIN;
ALTER TABLE content_attachments
    ADD COLUMN IF NOT EXISTS post_id    bigint,
    ADD COLUMN IF NOT EXISTS reply_id   bigint,
    ADD COLUMN IF NOT EXISTS roadmap_id bigint;

ALTER TABLE content_attachments
    DROP CONSTRAINT IF EXISTS chk_content_attachments_single_binding;

ALTER TABLE content_attachments
    ADD CONSTRAINT chk_content_attachments_single_binding
        CHECK (num_nonnulls(post_id, reply_id, roadmap_id) <= 1);

-- 已绑定行（三列至少一列非空）的部分索引：论坛 / 路线查询用，孤儿行不进索引。
DROP INDEX IF EXISTS idx_content_attachments_bound;
CREATE INDEX idx_content_attachments_bound
    ON content_attachments (COALESCE(post_id, reply_id, roadmap_id))
    WHERE post_id IS NOT NULL OR reply_id IS NOT NULL OR roadmap_id IS NOT NULL;

COMMENT ON COLUMN content_attachments.post_id IS '绑定的招新问答帖子 id（forum_posts）；NULL 表示未绑定到帖子。';
COMMENT ON COLUMN content_attachments.reply_id IS '绑定的回复 id（forum_replies）；NULL 表示未绑定到回复。';
COMMENT ON COLUMN content_attachments.roadmap_id IS '绑定的学习路线 id（learning_roadmaps）；NULL 表示未绑定到路线。';
COMMIT;
