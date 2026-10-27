-- 0014 工作室项目与参与关系
--
-- 执行方式：cd apps/api && go run ./tools/migrate up
--
-- 依赖 0013 studio_members。
-- 决策见 docs/adr/0001-studio-member-project-model.md：
--   - studio_projects + studio_project_members 多对多
--   - name 快照冗余；member_id 可空（外部参与者）

BEGIN;

CREATE TABLE IF NOT EXISTS studio_projects (
    id          bigserial PRIMARY KEY,
    name        text        NOT NULL,
    year        text        NOT NULL,   -- e.g. "2023"；展示用字符串，不做日期校验
    summary     text        NOT NULL DEFAULT '',
    status      text        NOT NULL DEFAULT 'active',
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_studio_projects_status
        CHECK (status IN ('active', 'completed', 'archived'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_projects_name
    ON studio_projects (name);

CREATE INDEX IF NOT EXISTS idx_studio_projects_status
    ON studio_projects (status);

CREATE INDEX IF NOT EXISTS idx_studio_projects_year
    ON studio_projects (year);

CREATE TABLE IF NOT EXISTS studio_project_members (
    id          bigserial PRIMARY KEY,
    project_id  bigint      NOT NULL REFERENCES studio_projects (id) ON DELETE CASCADE,
    member_id   bigint      REFERENCES studio_members (id) ON DELETE SET NULL,
    name        text        NOT NULL,   -- 姓名快照
    role        text        NOT NULL DEFAULT 'contributor',
    started_at  timestamptz NOT NULL DEFAULT NOW(),
    ended_at    timestamptz,
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_studio_project_members_role
        CHECK (role IN ('lead', 'contributor'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_project_members_project_name
    ON studio_project_members (project_id, name);

CREATE INDEX IF NOT EXISTS idx_studio_project_members_project
    ON studio_project_members (project_id);

CREATE INDEX IF NOT EXISTS idx_studio_project_members_member
    ON studio_project_members (member_id)
    WHERE member_id IS NOT NULL;

COMMENT ON TABLE studio_projects IS
  '工作室项目（0014）。';

COMMENT ON TABLE studio_project_members IS
  '项目参与：name 为快照；member_id 可空表示外部人员。';

COMMIT;
