-- 0016 回滚：只删这张表。
-- 三列绑定与 CHECK 约束由 #217 的迁移加，随其 down 一起回滚。
BEGIN;
DROP INDEX IF EXISTS idx_content_attachments_uploader;
DROP INDEX IF EXISTS idx_content_attachments_created;
DROP TABLE IF EXISTS content_attachments;
COMMIT;
