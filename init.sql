-- wb
DROP TRIGGER IF EXISTS blg_posts_after_insert;
DROP TRIGGER IF EXISTS blg_posts_after_delete;
DROP TRIGGER IF EXISTS blg_posts_after_update;
DROP TABLE IF EXISTS blg_users_posts;
DROP TABLE IF EXISTS blg_posts_fts;
DROP TABLE IF EXISTS blg_posts;
DROP TABLE IF EXISTS blg_config;
-- pm
DROP TABLE IF EXISTS pm_routes;
DROP TABLE IF EXISTS pm_templates;
DROP TABLE IF EXISTS pm_users;
DROP TABLE IF EXISTS pm_kv;

-- pagemanager
CREATE TABLE pm_kv (
    key TEXT NOT NULL PRIMARY KEY
    ,value TEXT
);

CREATE TABLE pm_users (
    user_id BIGINT NOT NULL PRIMARY KEY
);

CREATE TABLE pm_templates (
    template_id BIGINT NOT NULL PRIMARY KEY
    ,plugin TEXT
    ,name TEXT
    ,template_name TEXT
    ,template_body TEXT
    ,template_gob BYTEA
);

CREATE TABLE pm_routes (
    url TEXT NOT NULL PRIMARY KEY
    ,disabled BOOLEAN
    ,page TEXT
    ,redirect_url TEXT
    ,handler_url TEXT
);

-- blog
CREATE TABLE blg_config (
    config_id INT NOT NULL PRIMARY KEY
    ,json JSONB
    ,pagination_format TEXT
    ,posts_per_page INT
    ,date_format TEXT
    ,index_post_format TEXT
    ,url_format TEXT
);

CREATE TABLE blg_posts (
    post_id BIGINT NOT NULL PRIMARY KEY
    ,slug TEXT
    ,title TEXT
    ,summary TEXT
    ,body TEXT
    ,published_on TIMESTAMPTZ
    ,unpublished_on TIMESTAMPTZ
    ,created_at TIMESTAMPTZ
    ,updated_at TIMESTAMPTZ
);

-- https://kimsereylam.com/sqlite/2020/03/06/full-text-search-with-sqlite.html
CREATE VIRTUAL TABLE blg_posts_fts USING FTS5 (
    title
    ,summary
    ,body
    ,content='blg_posts'
    ,content_rowid='post_id'
);

CREATE TRIGGER blg_posts_after_insert AFTER INSERT ON blg_posts
BEGIN
    INSERT INTO blg_posts_fts
        (rowid, title, summary, body)
    VALUES
        (NEW.post_id, NEW.title, NEW.summary, NEW.body)
    ;
END;

CREATE TRIGGER blg_posts_after_delete AFTER DELETE ON blg_posts
BEGIN
    INSERT INTO blg_posts_fts
        (blg_posts_fts, rowid, title, summary, body)
    VALUES
        ('delete', OLD.id, OLD.title, OLD.summary, OLD.body)
    ;
END;

CREATE TRIGGER blg_posts_after_update AFTER UPDATE ON blg_posts
BEGIN
    INSERT INTO blg_posts_fts
        (blg_posts_fts, rowid, title, summary, body)
    VALUES
        ('delete', OLD.id, OLD.title, OLD.summary, OLD.body)
    ;
    INSERT INTO blg_posts_fts
        (rowid, title, summary, body)
    VALUES
        (NEW.id, NEW.title, NEW.summary, NEW.body)
    ;
END;

CREATE TABLE blg_users_posts (
    user_id BIGINT
    ,post_id BIGINT

    ,FOREIGN KEY (user_id) REFERENCES pm_users (user_id)
    ,FOREIGN KEY (post_id) REFERENCES blg_posts (post_id)
);
