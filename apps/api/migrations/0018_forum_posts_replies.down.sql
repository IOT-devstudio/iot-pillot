-- 0018 回滚：删除帖子与回复表（先 replies 后 posts，避免依赖顺序问题）。

BEGIN;

DROP TABLE IF EXISTS forum_replies;
DROP TABLE IF EXISTS forum_posts;

COMMIT;
