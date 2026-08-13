SET NAMES utf8mb4;

CREATE TABLE sys_dict_data (
    dict_code bigint NOT NULL PRIMARY KEY,
    dict_sort int,
    dict_label varchar(100),
    dict_value varchar(100),
    dict_type varchar(100),
    status char(1)
);

INSERT INTO sys_dict_data VALUES
    (9007199254740993, 1, '玄幻', 'fantasy', 'novel_book_category', '0'),
    (9007199254740994, 2, '系统', 'system', 'novel_book_sub_category', '1'),
    (9007199254740995, 3, '', 'invalid', 'novel_book_category', '0');

CREATE TABLE book_category (
    id bigint NOT NULL PRIMARY KEY,
    name varchar(100),
    work_direction varchar(20),
    sort int
);

INSERT INTO book_category VALUES (7001, '历史分类', '0', 9);

CREATE TABLE novel_book (
    id bigint NOT NULL PRIMARY KEY,
    author_id bigint,
    author_name varchar(100),
    work_direction varchar(20)
);

INSERT INTO novel_book VALUES
    (8001, 9007199254740993, '现行作者', '0'),
    (8002, NULL, '无ID作者', '1'),
    (8003, NULL, ' 无 ID 作者 ', '1'),
    (8004, NULL, '', '0');

CREATE TABLE book_author (
    id bigint NOT NULL PRIMARY KEY,
    pen_name varchar(100),
    work_direction varchar(20),
    status int
);

INSERT INTO book_author VALUES
    (9007199254740993, '不应覆盖现行作者', '1', 2),
    (9007199254740994, '审核作者', '1', 0);

CREATE TABLE author (
    id bigint NOT NULL PRIMARY KEY,
    pen_name varchar(100),
    work_direction varchar(20),
    status int
);

INSERT INTO author VALUES
    (9007199254740993, '更早作者', '0', 1),
    (9007199254740995, '独立历史作者', '0', 0);
