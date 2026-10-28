-- 0015 down：删除知识库修订与条目表。

BEGIN;

DROP INDEX IF EXISTS idx_studio_knowledge_revisions_entry;
DROP INDEX IF EXISTS idx_studio_knowledge_revisions_entry_version;
DROP TABLE IF EXISTS studio_knowledge_revisions;

DROP INDEX IF EXISTS idx_studio_knowledge_author;
DROP INDEX IF EXISTS idx_studio_knowledge_parent;
DROP INDEX IF EXISTS idx_studio_knowledge_permission;
DROP TABLE IF EXISTS studio_knowledge_entries;

COMMIT;
