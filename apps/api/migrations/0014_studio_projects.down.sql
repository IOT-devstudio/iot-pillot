-- 0014 down：删除项目与参与表。

BEGIN;

DROP INDEX IF EXISTS idx_studio_project_members_member;
DROP INDEX IF EXISTS idx_studio_project_members_project;
DROP INDEX IF EXISTS idx_studio_project_members_project_name;
DROP TABLE IF EXISTS studio_project_members;

DROP INDEX IF EXISTS idx_studio_projects_year;
DROP INDEX IF EXISTS idx_studio_projects_status;
DROP INDEX IF EXISTS idx_studio_projects_name;
DROP TABLE IF EXISTS studio_projects;

COMMIT;
