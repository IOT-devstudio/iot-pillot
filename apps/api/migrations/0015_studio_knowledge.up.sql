-- 0015 工作室知识库条目与修订历史
--
-- 执行方式：cd apps/api && go run ./tools/migrate up
--
-- 决策见 docs/adr/0001-studio-member-project-model.md：
--   - entries 存当前正文；revisions 存每次修订快照
--   - 不做自动回滚；恢复 = 再写新版本
--   - 首期路由仅 admin；permission 字段预留 member/public

BEGIN;

CREATE TABLE IF NOT EXISTS studio_knowledge_entries (
    id          bigserial PRIMARY KEY,
    title       text        NOT NULL,
    body        text        NOT NULL DEFAULT '',
    author_id   bigint      NOT NULL,   -- users.id；不强制 FK，避免用户软删牵连
    is_public   boolean     NOT NULL DEFAULT false,
    permission  text        NOT NULL DEFAULT 'admin',
    parent_id   bigint      REFERENCES studio_knowledge_entries (id) ON DELETE SET NULL,
    version     integer     NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_studio_knowledge_permission
        CHECK (permission IN ('public', 'member', 'admin')),
    CONSTRAINT chk_studio_knowledge_version
        CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS idx_studio_knowledge_permission
    ON studio_knowledge_entries (permission);

CREATE INDEX IF NOT EXISTS idx_studio_knowledge_parent
    ON studio_knowledge_entries (parent_id)
    WHERE parent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_studio_knowledge_author
    ON studio_knowledge_entries (author_id);

CREATE TABLE IF NOT EXISTS studio_knowledge_revisions (
    id          bigserial PRIMARY KEY,
    entry_id    bigint      NOT NULL REFERENCES studio_knowledge_entries (id) ON DELETE CASCADE,
    version     integer     NOT NULL,
    title       text        NOT NULL,
    body        text        NOT NULL,
    editor_id   bigint      NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_studio_knowledge_rev_version
        CHECK (version >= 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_knowledge_revisions_entry_version
    ON studio_knowledge_revisions (entry_id, version);

CREATE INDEX IF NOT EXISTS idx_studio_knowledge_revisions_entry
    ON studio_knowledge_revisions (entry_id);

COMMENT ON TABLE studio_knowledge_entries IS
  '知识库当前正文（0015）；version 与 revisions 最新版对齐。';

COMMENT ON TABLE studio_knowledge_revisions IS
  '知识库修订快照；恢复靠管理员基于旧版再写新版。';

COMMIT;
