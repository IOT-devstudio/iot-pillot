-- 0013 工作室成员档案（Studio Member）
--
-- 执行方式：cd apps/api && go run ./tools/migrate up
--
-- 决策见 docs/adr/0001-studio-member-project-model.md：
--   - 独立于 recruitment_candidates；hired 不自动建成员。
--   - user_id 可空：收录无系统账号的校友/外部人员。
--   - (name, birth_year) 唯一：同名不同届可共存。
--
-- 幂等写法（IF NOT EXISTS），重复执行不报错。

BEGIN;

CREATE TABLE IF NOT EXISTS studio_members (
    id              bigserial PRIMARY KEY,
    user_id         bigint,                 -- NULL = 无系统账号
    name            text        NOT NULL,
    birth_year      integer     NOT NULL,
    cohort          text        NOT NULL,   -- e.g. "2024 级"
    direction       text        NOT NULL,   -- 与招新方向码对齐时可复用 front-end/back-end/agent...
    role_in_studio  text        NOT NULL DEFAULT '',
    internship      text        NOT NULL DEFAULT '',
    employment      text        NOT NULL DEFAULT '',
    is_active       boolean     NOT NULL DEFAULT true,
    archived_at     timestamptz,
    source_candidate_id bigint,             -- 可选：从 hired 候选人转入时的溯源
    created_at      timestamptz NOT NULL DEFAULT NOW(),
    updated_at      timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_members_name_birth_year
    ON studio_members (name, birth_year);

CREATE INDEX IF NOT EXISTS idx_studio_members_user_id
    ON studio_members (user_id)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_studio_members_direction
    ON studio_members (direction);

CREATE INDEX IF NOT EXISTS idx_studio_members_active
    ON studio_members (is_active);

CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_members_source_candidate
    ON studio_members (source_candidate_id)
    WHERE source_candidate_id IS NOT NULL;

COMMENT ON TABLE studio_members IS
  '工作室成员档案（0013）：与招新候选人分离；可无 user_id。';

COMMENT ON COLUMN studio_members.source_candidate_id IS
  '可选溯源：从 recruitment_candidates.id 手动转入时写入，防重复转入。';

COMMIT;
