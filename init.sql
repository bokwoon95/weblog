DROP TABLE IF EXISTS pm_routes;
CREATE TABLE pm_routes (
    url TEXT NOT NULL PRIMARY KEY
    ,disabled BOOLEAN
    ,page TEXT
    ,handler_url TEXT
);
