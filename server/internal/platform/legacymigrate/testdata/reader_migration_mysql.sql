SET NAMES utf8mb4;

CREATE TABLE reader_user (
    id bigint NOT NULL PRIMARY KEY,
    username varchar(64) NOT NULL UNIQUE,
    nickname varchar(64) NOT NULL DEFAULT '',
    password_hash varchar(100) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'enabled',
    token_version bigint NOT NULL DEFAULT 0,
    invite_code_id bigint,
    last_login_time datetime,
    create_time datetime NOT NULL,
    update_time datetime NOT NULL
);
INSERT INTO reader_user VALUES
    (9007199254740993,'reader_bcrypt','散文读者','$2a$10$7EqJtq98hPqEX7fNZaFWoO5QHM7G.ZD4D1z5M4dXq7j3iU3V8WQ8u','enabled',7,NULL,'2026-07-01 01:02:03','2026-06-01 01:02:03','2026-07-01 01:02:03'),
    (9007199254740994,'reader_md5','夜读者','5f4dcc3b5aa765d61d8327deb882cf99','disabled',9,NULL,NULL,'2026-06-02 01:02:03','2026-07-02 01:02:03'),
    (9007199254740995,'reader_invalid','坏摘要','not-a-password-hash','enabled',11,NULL,NULL,'2026-06-03 01:02:03','2026-07-03 01:02:03');

CREATE TABLE reader_invite_code (
    id bigint NOT NULL PRIMARY KEY, code varchar(64) NOT NULL, inviter_reader_id bigint,
    status varchar(20) NOT NULL, max_use_count int, used_count int NOT NULL, expire_time datetime,
    remark varchar(255) NOT NULL, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_invite_code VALUES
    (9007199254741101,'INVITE-READER',9007199254740993,'enabled',10,1,'2027-01-01 00:00:00','fixture','2026-06-01 00:00:00','2026-07-01 00:00:00');

CREATE TABLE reader_invite_relation (
    id bigint NOT NULL PRIMARY KEY, inviter_reader_id bigint NOT NULL, invitee_reader_id bigint NOT NULL,
    invite_code varchar(64) NOT NULL, status varchar(20) NOT NULL, create_time datetime NOT NULL
);
INSERT INTO reader_invite_relation VALUES
    (9007199254741102,9007199254740993,9007199254740994,'INVITE-READER','active','2026-06-02 00:00:00');

CREATE TABLE reader_product (
    id bigint NOT NULL PRIMARY KEY, product_type varchar(20) NOT NULL, target_id bigint,
    product_name varchar(100) NOT NULL, price_coin bigint NOT NULL, allow_bonus_coin tinyint(1) NOT NULL,
    duration_days int, sale_status varchar(20) NOT NULL, sort_order int NOT NULL,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_product VALUES
    (9007199254741201,'book',9007199254742001,'迁移整书',88,1,NULL,'on_sale',3,'2026-06-03 00:00:00','2026-07-03 00:00:00');

CREATE TABLE reader_membership_grant (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, grant_type varchar(20) NOT NULL,
    grant_no varchar(64) NOT NULL, create_time datetime NOT NULL, after_expire_time datetime,
    after_permanent tinyint(1) NOT NULL, status varchar(20) NOT NULL
);
INSERT INTO reader_membership_grant VALUES
    (9007199254741301,9007199254740993,'days_30','MG-FIXTURE-1','2026-06-04 00:00:00','2026-07-04 00:00:00',0,'confirmed'),
    (9007199254741302,9007199254740994,'unsupported','MG-FIXTURE-2','2026-06-04 00:00:00',NULL,1,'confirmed');

CREATE TABLE reader_entitlement (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, entitlement_type varchar(20) NOT NULL,
    target_id bigint NOT NULL, start_time datetime NOT NULL, expire_time datetime, status varchar(20) NOT NULL,
    source_order_no varchar(64) NOT NULL, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_entitlement VALUES
    (9007199254741401,9007199254740993,'book',9007199254742001,'2026-06-05 00:00:00',NULL,'active','ORDER-FIXTURE-1','2026-06-05 00:00:00','2026-07-05 00:00:00'),
    (9007199254741402,9007199254740994,'membership',0,'2026-06-05 00:00:00','2026-08-05 00:00:00','disabled','','2026-06-05 00:00:00','2026-07-05 00:00:00'),
    (9007199254741403,9007199254740994,'book',0,'2026-06-05 00:00:00',NULL,'active','','2026-06-05 00:00:00','2026-07-05 00:00:00');

CREATE TABLE reader_book_like (id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, book_id bigint NOT NULL, create_time datetime NOT NULL);
INSERT INTO reader_book_like VALUES (9007199254741501,9007199254740993,9007199254742001,'2026-06-06 00:00:00');

CREATE TABLE reader_bookshelf (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, book_id bigint NOT NULL, last_chapter_id bigint,
    last_read_time datetime, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_bookshelf VALUES (9007199254741601,9007199254740993,9007199254742001,9007199254742101,'2026-07-07 00:00:00','2026-06-07 00:00:00','2026-07-07 00:00:00');

CREATE TABLE reader_reading_history (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, book_id bigint NOT NULL, chapter_id bigint NOT NULL,
    chapter_no int NOT NULL, position_type varchar(20) NOT NULL, position_value int NOT NULL,
    progress_percent decimal(5,2) NOT NULL, last_read_time datetime NOT NULL,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_reading_history VALUES (9007199254741701,9007199254740993,9007199254742001,9007199254742101,12,'page',4,37.50,'2026-07-08 00:00:00','2026-06-08 00:00:00','2026-07-08 00:00:00');

CREATE TABLE reader_reading_preference (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, font_size int NOT NULL,
    line_height decimal(4,2) NOT NULL, theme varchar(20) NOT NULL, reading_mode varchar(20) NOT NULL,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_reading_preference VALUES (9007199254741801,9007199254740993,24,2.25,'green','scroll','2026-06-09 00:00:00','2026-07-09 00:00:00');

CREATE TABLE reader_feedback (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, content varchar(1000) NOT NULL,
    status varchar(16) NOT NULL, reply_content varchar(1000), reply_time datetime,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_feedback VALUES (9007199254741901,9007199254740993,'迁移反馈','replied','已处理','2026-07-10 00:00:00','2026-06-10 00:00:00','2026-07-10 00:00:00');
