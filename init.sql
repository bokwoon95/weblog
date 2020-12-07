DROP TABLE IF EXISTS wb_users_posts;
DROP TABLE IF EXISTS wb_posts;
DROP TABLE IF EXISTS pm_routes;
DROP TABLE IF EXISTS pm_users;

-- pagemanager
CREATE TABLE pm_users (
    user_id BIGINT NOT NULL PRIMARY KEY
);

CREATE TABLE pm_routes (
    url TEXT NOT NULL PRIMARY KEY
    ,disabled BOOLEAN
    ,page TEXT
    ,redirect_url TEXT
    ,handler_url TEXT
);

-- blog
-- TODO: how to design blog tables such that it can accomodate arbitrary URL hierarchies?
CREATE TABLE wb_posts (
    post_id BIGINT NOT NULL PRIMARY KEY
    ,slug TEXT
    ,title TEXT
    ,summary TEXT
    ,published_on TIMESTAMPTZ
    ,unpublished_on TIMESTAMPTZ
    ,created_at TIMESTAMPTZ
);

CREATE TABLE wb_users_posts (
    user_id BIGINT
    ,post_id BIGINT

    ,FOREIGN KEY (user_id) REFERENCES pm_users (user_id)
    ,FOREIGN KEY (post_id) REFERENCES wb_posts (post_id)
);
