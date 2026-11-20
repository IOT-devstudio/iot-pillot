-- 作品提交（学员交作业）。
--
-- 一个提交 = 一个学员针对某条学习路线（或整个工作室）交的一份作品，
-- 载体二选一或同时给：源码 zip 文件 / GitHub 仓库链接 / 其它公开仓库链接。
--
-- 为什么单独一张表而不是复用 content_attachments：
--   那个表是「正文里引用的图片」的绑定关系（ADR 0004），只接受 jpg/png/webp，
--   且生命周期跟着帖子正文走；作品是独立实体，要能列出来、被审阅、被评价，
--   而且要收 zip。两者语义不同，混用会让附件表承担它不该承担的职责。
--
-- 为什么不建外键指向 learning_roadmaps：
--   与 routes 的方向软引用同理——路线可能被软删或重构，
--   而作品是学员已交付的事实，不该因为路线没了就跟着悬空失败。
--   保留 roadmap_id + roadmap_title_snapshot，路线不在时仍能展示当时交的是哪条。
CREATE TABLE IF NOT EXISTS showcase_submissions (
    id                  bigserial   PRIMARY KEY,
    -- 提交人。作品只属于本人，没有「代他人提交」的端点。
    user_id             bigint      NOT NULL,
    -- 关联的学习路线（可空：允许提交与路线无关的自由作品）。
    roadmap_id          bigint,
    -- 提交时路线的标题快照：路线改名或被删后，这次提交仍能看到当时交的是哪条。
    roadmap_title       text        NOT NULL DEFAULT '',

    title               text        NOT NULL,
    -- 学员对自己作品的说明（Markdown，可为空）。
    summary             text        NOT NULL DEFAULT '',

    -- 三种载体，至少要有一个（约束在下面兜底，服务层也给可读错误）。
    repo_url            text        NOT NULL DEFAULT '',
    -- 源码 zip：存对象存储的 key，不存二进制。
    archive_object_key  text        NOT NULL DEFAULT '',
    archive_name        text        NOT NULL DEFAULT '',
    archive_size        bigint      NOT NULL DEFAULT 0,

    -- 提交后不可改状态的只有建立时间；审核状态由管理端推进。
    status              text        NOT NULL DEFAULT 'submitted',
    -- 审核意见（驳回时必填，通过时可空）。
    review_note         text        NOT NULL DEFAULT '',
    reviewed_at         timestamptz,
    reviewed_by         bigint,

    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT showcase_submissions_title_not_blank
        CHECK (length(btrim(title)) > 0),
    -- 至少要有一个载体：空提交对审阅者毫无意义。
    CONSTRAINT showcase_submissions_has_payload
        CHECK (length(btrim(repo_url)) > 0 OR length(btrim(archive_object_key)) > 0),
    -- 只允许已知状态，防止拼错的状态字符串混进列表。
    CONSTRAINT showcase_submissions_status_known
        CHECK (status IN ('submitted', 'approved', 'rejected'))
);

-- 列表：按时间倒序展示自己的作品。
CREATE INDEX IF NOT EXISTS idx_showcase_submissions_user
    ON showcase_submissions (user_id, created_at DESC);

-- 管理端按「待审阅优先 + 时间倒序」翻页。
CREATE INDEX IF NOT EXISTS idx_showcase_submissions_status
    ON showcase_submissions (status, created_at DESC);

COMMENT ON TABLE showcase_submissions IS
    '作品提交：学员交付的源码 zip 或仓库链接，供管理端审阅。附件表只收图片，故独立成表。';
COMMENT ON COLUMN showcase_submissions.roadmap_id IS
    '关联的学习路线（软引用，不建外键）：路线被删后作品仍保留。';
COMMENT ON COLUMN showcase_submissions.roadmap_title IS
    '提交时路线标题的快照，路线改名或删除后仍能看出当时交的是哪条。';
COMMENT ON COLUMN showcase_submissions.archive_object_key IS
    '源码 zip 在对象存储中的 key；不把二进制塞进数据库。';
