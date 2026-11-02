-- 0017 回滚：去掉三列绑定与约束，回到 #216 的孤儿态表。
BEGIN;
ALTER TABLE content_attachments
    DROP CONSTRAINT IF EXISTS chk_content_attachments_single_binding;
DROP INDEX IF EXISTS idx_content_attachments_bound;
ALTER TABLE content_attachments
    DROP COLUMN IF EXISTS post_id,
    DROP COLUMN IF EXISTS reply_id,
    DROP COLUMN IF EXISTS roadmap_id;
COMMENT ON TABLE content_attachments IS '内容附件（图片）。上传后先以孤儿态落库，归属由正文里的附件引用决定（见 ADR 0004）。';
COMMIT;
