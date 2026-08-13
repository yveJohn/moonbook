SET NAMES utf8mb4;

CREATE TABLE novel_book (
    id bigint NOT NULL PRIMARY KEY,
    work_direction varchar(20), category_code varchar(32), cover_url varchar(500),
    book_name varchar(100) NOT NULL, author_id bigint, author_name varchar(100),
    book_desc varchar(2000), score decimal(5,2), book_status varchar(20), publish_status varchar(20),
    source_type varchar(30), featured tinyint, featured_sort int, featured_note varchar(255),
    visit_count bigint, like_count int, word_count int, comment_count int, yesterday_buy int,
    last_chapter_id bigint, last_chapter_name varchar(255), last_chapter_update_time datetime,
    charge_mode varchar(30), legacy_crawl_source_id bigint, legacy_crawl_book_id varchar(128),
    legacy_crawl_last_time datetime, legacy_crawl_is_stop tinyint, create_time datetime, update_time datetime
);

INSERT INTO novel_book VALUES
 (9007199254740996,'0','fantasy','https://legacy.test/cover.jpg','迁移书籍',9007199254740993,'现行作者','迁移简介',9.25,'0','1','legacy',1,7,'精选',123,4,5678,2,3,9007199254740997,'第一章','2026-08-01 12:00:00','fixed_price',12,'remote-1','2026-08-02 12:00:00',0,'2026-07-01 10:00:00','2026-08-03 10:00:00'),
 (9007199254740997,'1','fantasy','','无ID作者书',NULL,'无ID作者','',0,'1','2','txt_import',0,0,'',0,0,0,0,0,NULL,NULL,NULL,'login_free',NULL,NULL,NULL,1,NULL,'2026-08-03 11:00:00'),
 (9007199254740998,'0','missing','','坏分类书',9007199254740993,'现行作者','',8,'0','0','legacy',0,0,'',0,0,0,0,0,NULL,NULL,NULL,'word_charge',NULL,NULL,NULL,0,NULL,'2026-08-03 12:00:00'),
 (9007199254740999,'0','fantasy','','坏状态书',9007199254740993,'现行作者','',8,'9','0','legacy',0,0,'',0,0,0,0,0,NULL,NULL,NULL,'word_charge',NULL,NULL,NULL,0,NULL,'2026-08-03 13:00:00');

CREATE TABLE reader_product (
    id bigint NOT NULL PRIMARY KEY, product_type varchar(20) NOT NULL, target_id bigint,
    product_name varchar(100) NOT NULL, price_coin bigint NOT NULL,
    UNIQUE KEY uk_reader_product_book_target (product_type,target_id)
);
INSERT INTO reader_product VALUES (9101,'book',9007199254740996,'迁移书籍',9007199254740993);

CREATE TABLE novel_book_sub_category_rel (
    id bigint NOT NULL PRIMARY KEY, book_id bigint NOT NULL, category_code varchar(32) NOT NULL,
    category_name varchar(50), sort int
);
INSERT INTO novel_book_sub_category_rel VALUES
 (9201,9007199254740996,'system','系统',1),
 (9202,9007199254740996,'missing','缺失',2),
 (9203,9223372036854775807,'system','系统',3);
