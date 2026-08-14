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
    (1,1,0,1,'迁移书城','https://legacy.example.com/','旧默认描述','旧首页标题','旧首页描述',
     '书库 - {siteName}','{keyword}{categoryName}{subCategoryName}',
     '{bookName} - {authorName} - {siteName}','{bookDesc}{categoryName}',
     '2026-08-01 10:00:00','2026-08-02 11:00:00'),
    (2,1,1,1,'额外配置','https://extra.example.com','额外描述','额外标题','额外首页描述',
     '{siteName}','{siteName}','{siteName}','{siteName}',
     '2026-08-03 10:00:00','2026-08-03 11:00:00');
