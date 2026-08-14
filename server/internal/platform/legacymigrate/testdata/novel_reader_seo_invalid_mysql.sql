SET NAMES utf8mb4;

CREATE TABLE novel_reader_seo_config (
    id bigint NOT NULL PRIMARY KEY,
    seo_enabled tinyint NOT NULL,
    indexing_enabled tinyint NOT NULL,
    sitemap_enabled tinyint NOT NULL,
    site_name varchar(100),
    site_url varchar(2048),
    default_description varchar(2000),
    home_title varchar(500),
    home_description varchar(2000),
    books_title_template varchar(500),
    books_description_template varchar(2000),
    book_title_template varchar(500),
    book_description_template varchar(2000),
    create_time datetime,
    update_time datetime
);

INSERT INTO novel_reader_seo_config VALUES
    (1,1,1,1,'无效配置','https://legacy.example.com/path','默认描述','首页标题','首页描述',
     '{siteName}','{siteName}','{unknown}','{bookDesc}',
     '2026-08-01 10:00:00','2026-08-02 11:00:00');
