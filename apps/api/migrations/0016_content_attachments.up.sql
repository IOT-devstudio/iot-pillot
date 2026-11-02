-- 0016: 内容附件（图片）。
--
-- #216 只建「孤儿上传」所需的基础列：上传时还不知道附件属于哪篇帖子 / 回复 /
-- 学习路线。三列绑定（post_id / reply_id / roadmap_id）由 #217 的迁移补上
-- ——那时被引用的 forum_posts / forum_replies / learning_roadmaps 表才存在，
-- CHECK 约束与孤儿清理所需的部分索引也随 #217 一并落地。
BEGIN;
CREATE TABLE IF NOT EXISTS content_attachments (
    id           bigserial PRIMARY KEY,
    object_key   text NOT NULL UNIQUE,
    uploader_id  bigint NOT NULL,
    content_type text NOT NULL,
    size_bytes   bigint NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_content_attachments_uploader ON content_attachments (uploader_id);
CREATE INDEX IF NOT EXISTS idx_content_attachments_created ON content_attachments (created_at);
COMMENT ON TABLE content_attachments IS '内容附件（图片）。上传后先以孤儿态落库，归属由正文里的附件引用决定（见 ADR 0004）。';
COMMENT ON COLUMN content_attachments.object_key IS '对象存储里的对象键：随机生成，扩展名从真实字节嗅探得出，不含用户文件名。';
COMMENT ON COLUMN content_attachments.uploader_id IS '上传者 users.id；不强制 FK，避免用户软删牵连。';
COMMENT ON COLUMN content_attachments.content_type IS '按真实字节嗅探出的类型：image/jpeg / image/png / image/webp。';
COMMENT ON COLUMN content_attachments.size_bytes IS '上传字节数。';
COMMIT;
