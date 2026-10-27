-- Up migration for studio_members table
-- Purpose: Independent member archive for in-studio members (current and alumni)
-- including those without system accounts. See ADR 0001.

CREATE TABLE studio_members (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id), -- nullable; external members have no account
    name TEXT NOT NULL,
    birth_year INTEGER NOT NULL,
    cohort TEXT NOT NULL, -- e.g., "2024 级"
    direction VARCHAR(50) NOT NULL, -- frontend/back-end/agent/all/game/other
    role_in_studio TEXT NOT NULL, -- e.g., "前端负责人"
    internship TEXT DEFAULT '', -- internship destination
    employment TEXT DEFAULT '', -- employment destination
    is_active BOOLEAN DEFAULT TRUE, -- true = current member
    archived_at TIMESTAMPTZ, -- graduation/exit time
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_studio_members_name_cohort ON studio_members(name, birth_year);
CREATE INDEX idx_studio_members_user_id ON studio_members(user_id);
CREATE INDEX idx_studio_members_direction ON studio_members(direction);

-- Down migration
DROP INDEX IF EXISTS idx_studio_members_name_cohort;
DROP INDEX IF EXISTS idx_studio_members_user_id;
DROP INDEX IF EXISTS idx_studio_members_direction;
DROP TABLE IF EXISTS studio_members;
