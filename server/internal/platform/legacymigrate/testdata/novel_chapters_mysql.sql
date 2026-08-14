SET NAMES utf8mb4;

CREATE TABLE novel_chapter (
    id bigint NOT NULL PRIMARY KEY,
    book_id bigint NOT NULL,
    chapter_no int NOT NULL,
    chapter_name varchar(255),
    word_count int,
    is_vip tinyint,
    book_price bigint,
    storage_type varchar(20),
    chapter_status varchar(20),
    ai_clean_status varchar(20),
    create_time datetime,
    update_time datetime
);

CREATE TABLE novel_chapter_content (
    id bigint NOT NULL AUTO_INCREMENT PRIMARY KEY,
    chapter_id bigint NOT NULL,
    content mediumtext,
    UNIQUE KEY uk_chapter_content (chapter_id)
);

INSERT INTO novel_chapter VALUES
    (9007199254741010,9007199254741000,0,'第一章',1,0,0,'db','0','0','2026-08-01 10:00:00','2026-08-01 10:10:00'),
    (9007199254741011,9007199254741000,1,'第二章',4,1,9007199254740993,'db','1','5','2026-08-01 11:00:00','2026-08-01 11:10:00'),
    (9007199254741012,9007199254741000,2,'坏状态章',4,0,0,'db','9','0','2026-08-01 12:00:00','2026-08-01 12:10:00'),
    (9007199254741013,9223372036854775807,0,'缺书章节',4,0,0,'db','0','0','2026-08-01 13:00:00','2026-08-01 13:10:00'),
    (9007199254741014,9007199254741000,3,'缺正文章',4,0,0,'db','0','0','2026-08-01 14:00:00','2026-08-01 14:10:00');

INSERT INTO novel_chapter_content(chapter_id,content) VALUES
    (9007199254741010,'第一章 正文 test'),
    (9007199254741011,'第二章正文'),
    (9007199254741012,'坏状态章正文'),
    (9007199254741013,'缺书章节正文');
