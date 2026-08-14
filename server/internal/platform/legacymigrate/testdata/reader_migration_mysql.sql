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

CREATE TABLE reader_wallet (
    reader_id bigint NOT NULL PRIMARY KEY, recharge_coin_balance bigint NOT NULL, bonus_coin_balance bigint NOT NULL,
    total_recharge_coin_income bigint NOT NULL, total_bonus_coin_income bigint NOT NULL,
    total_recharge_coin_expense bigint NOT NULL, total_bonus_coin_expense bigint NOT NULL,
    version int NOT NULL, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_wallet VALUES
    (9007199254740993,900,150,1000,200,100,50,4,'2026-06-11 00:00:00','2026-07-11 00:00:00'),
    (9007199254740994,0,0,0,0,0,0,0,'2026-06-11 00:00:00','2026-07-11 00:00:00');

CREATE TABLE reader_wallet_ledger (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, ledger_no varchar(64) NOT NULL,
    idempotency_key varchar(128) NOT NULL, biz_type varchar(40) NOT NULL, biz_id bigint,
    order_no varchar(64), direction varchar(20) NOT NULL, coin_type varchar(20) NOT NULL,
    amount bigint NOT NULL, balance_before bigint NOT NULL, balance_after bigint NOT NULL,
    remark varchar(255) NOT NULL, create_time datetime NOT NULL
);
INSERT INTO reader_wallet_ledger VALUES
    (9007199254743001,9007199254740993,'LEDGER-R-IN','ledger-r-in','recharge',9007199254743701,'RECHARGE-FIXTURE','income','recharge',1000,0,1000,'充值','2026-06-12 00:00:00'),
    (9007199254743002,9007199254740993,'LEDGER-R-OUT','ledger-r-out','purchase',9007199254743201,'ORDER-FINANCE-1','expense','recharge',100,1000,900,'购书','2026-06-13 00:00:00'),
    (9007199254743003,9007199254740993,'LEDGER-B-IN','ledger-b-in','invite',9007199254743501,NULL,'income','bonus',200,0,200,'邀请奖励','2026-06-14 00:00:00'),
    (9007199254743004,9007199254740993,'LEDGER-B-OUT','ledger-b-out','adjustment',9007199254743502,NULL,'expense','bonus',50,200,150,'人工扣减','2026-06-15 00:00:00');

CREATE TABLE reader_bonus_coin_bucket (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, source_type varchar(40) NOT NULL, source_id bigint,
    original_amount bigint NOT NULL, remaining_amount bigint NOT NULL, expire_time datetime NOT NULL,
    status varchar(20) NOT NULL, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_bonus_coin_bucket VALUES
    (9007199254743101,9007199254740993,'invite',9007199254743501,200,150,'2027-06-14 00:00:00','active','2026-06-14 00:00:00','2026-07-14 00:00:00');

CREATE TABLE reader_order (
    id bigint NOT NULL PRIMARY KEY, order_no varchar(64) NOT NULL, reader_id bigint NOT NULL,
    order_type varchar(30) NOT NULL, product_id bigint, product_type varchar(20), target_id bigint,
    book_id_snapshot bigint, product_name_snapshot varchar(100) NOT NULL, price_coin_snapshot bigint NOT NULL,
    chapter_word_count_snapshot int, pricing_word_unit_snapshot int, pricing_coin_unit_snapshot bigint,
    recharge_coin_amount bigint NOT NULL, bonus_coin_amount bigint NOT NULL, status varchar(20) NOT NULL,
    idempotency_key varchar(128) NOT NULL, remark varchar(255) NOT NULL, operator_id bigint,
    paid_time datetime, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_order VALUES
    (9007199254743201,'ORDER-FINANCE-1',9007199254740993,'buy_book',9007199254741201,'book',9007199254742001,NULL,'迁移整书',100,NULL,NULL,NULL,100,0,'paid','order-finance-1','fixture',NULL,'2026-06-13 00:00:00','2026-06-13 00:00:00','2026-07-13 00:00:00'),
    (9007199254743202,'ORDER-FINANCE-BAD',9007199254740993,'unknown_order',NULL,NULL,NULL,NULL,'坏订单',0,NULL,NULL,NULL,0,0,'paid','order-finance-bad','fixture',NULL,NULL,'2026-06-13 00:00:00','2026-07-13 00:00:00');

CREATE TABLE reader_checkin_reward_rule (
    id bigint NOT NULL PRIMARY KEY, rule_type varchar(20) NOT NULL, continuous_days int,
    reward_mode varchar(20) NOT NULL, fixed_coin bigint, min_coin bigint, max_coin bigint,
    status varchar(20) NOT NULL, sort_order int NOT NULL, remark varchar(255) NOT NULL,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_checkin_reward_rule VALUES
    (9007199254743301,'continuous',3,'fixed',30,NULL,NULL,'disabled',30,'迁移签到规则','2026-06-16 00:00:00','2026-07-16 00:00:00');

CREATE TABLE reader_checkin_record (
    id bigint NOT NULL PRIMARY KEY, reader_id bigint NOT NULL, checkin_date date NOT NULL,
    continuous_days int NOT NULL, base_reward_coin bigint NOT NULL, milestone_reward_coin bigint NOT NULL,
    total_reward_coin bigint NOT NULL, idempotency_key varchar(128) NOT NULL, create_time datetime NOT NULL
);
INSERT INTO reader_checkin_record VALUES
    (9007199254743401,9007199254740993,'2026-06-16',3,10,20,30,'checkin-fixture-1','2026-06-16 00:00:00');

CREATE TABLE reader_invite_reward_record (
    id bigint NOT NULL PRIMARY KEY, relation_id bigint NOT NULL, inviter_reader_id bigint NOT NULL,
    invitee_reader_id bigint NOT NULL, reward_stage varchar(30) NOT NULL, reward_coin bigint NOT NULL,
    status varchar(20) NOT NULL, idempotency_key varchar(128) NOT NULL, grant_time datetime,
    remark varchar(255) NOT NULL, create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_invite_reward_record VALUES
    (9007199254743501,9007199254741102,9007199254740993,9007199254740994,'register',200,'granted','invite-finance-1','2026-06-14 00:00:00','fixture','2026-06-14 00:00:00','2026-07-14 00:00:00');

CREATE TABLE reader_wallet_adjustment (
    id bigint NOT NULL PRIMARY KEY, adjustment_no varchar(64) NOT NULL, reader_id bigint NOT NULL,
    coin_type varchar(20) NOT NULL, direction varchar(20) NOT NULL, amount bigint NOT NULL,
    reason varchar(255) NOT NULL, status varchar(20) NOT NULL, idempotency_key varchar(128) NOT NULL,
    operator_id bigint, operator_name varchar(64) NOT NULL, ledger_id bigint,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_wallet_adjustment VALUES
    (9007199254743502,'ADJUST-FINANCE-1',9007199254740993,'bonus','decrease',50,'fixture','confirmed','adjust-finance-1',1,'管理员',9007199254743004,'2026-06-15 00:00:00','2026-07-15 00:00:00');

CREATE TABLE reader_recharge_product (
    id bigint NOT NULL PRIMARY KEY, product_name varchar(100) NOT NULL, diamond_amount bigint NOT NULL,
    price_usdt decimal(20,2) NOT NULL, sale_status varchar(20) NOT NULL, sort_order int NOT NULL,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_recharge_product VALUES
    (9007199254743601,'1000钻石',1000,100.00,'on_sale',100,'2026-06-17 00:00:00','2026-07-17 00:00:00');

CREATE TABLE reader_recharge_setting (
    id bigint NOT NULL PRIMARY KEY, custom_enabled tinyint(1) NOT NULL, diamonds_per_usdt decimal(20,8) NOT NULL,
    min_diamond_amount bigint NOT NULL, max_diamond_amount bigint NOT NULL, amount_scale int NOT NULL,
    rounding_mode varchar(20) NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_recharge_setting VALUES (1,1,10.00000000,10,100000,2,'CEILING','2026-07-18 00:00:00');

CREATE TABLE reader_payment_channel (
    id bigint NOT NULL PRIMARY KEY, provider varchar(30) NOT NULL, enabled tinyint(1) NOT NULL,
    currency varchar(20) NOT NULL, token varchar(20) NOT NULL, network varchar(20) NOT NULL,
    update_time datetime NOT NULL
);
INSERT INTO reader_payment_channel VALUES (1,'epusdt',0,'usd','usdt','tron','2026-07-18 00:00:00');

CREATE TABLE reader_recharge_order (
    id bigint NOT NULL PRIMARY KEY, order_no varchar(32) NOT NULL, reader_id bigint NOT NULL,
    request_id varchar(76) NOT NULL, source_type varchar(20) NOT NULL, product_id bigint,
    diamond_amount bigint NOT NULL, price_usdt decimal(20,2) NOT NULL, provider varchar(30) NOT NULL,
    channel_id bigint NOT NULL, payment_credential_id bigint NOT NULL, merchant_pid_snapshot varchar(64) NOT NULL,
    currency varchar(20) NOT NULL, token varchar(20) NOT NULL, network varchar(20) NOT NULL,
    gateway_trade_id varchar(64), gateway_actual_amount decimal(24,8), receive_address varchar(255),
    payment_url varchar(1000), block_transaction_id varchar(128), status varchar(30) NOT NULL,
    gateway_status int, wallet_ledger_id bigint, expire_time datetime, paid_time datetime,
    failure_code varchar(64) NOT NULL, failure_message varchar(500) NOT NULL,
    create_time datetime NOT NULL, update_time datetime NOT NULL
);
INSERT INTO reader_recharge_order VALUES
    (9007199254743701,'RECHARGE-FIXTURE',9007199254740993,'recharge-request-1','preset',9007199254743601,1000,100.00,'epusdt',1,77,'legacy-merchant-pid','usd','usdt','tron','TRADE-FIXTURE',100.00000000,'TAddress','https://pay.example.invalid/fixture','TX-FIXTURE','paid',2,9007199254743001,'2026-06-18 01:00:00','2026-06-18 00:30:00','','','2026-06-18 00:00:00','2026-07-18 00:00:00');

CREATE TABLE reader_payment_callback_log (
    id bigint NOT NULL PRIMARY KEY, provider varchar(30) NOT NULL, recharge_order_id bigint,
    merchant_order_no varchar(32) NOT NULL, gateway_trade_id varchar(64) NOT NULL, source_ip varchar(64) NOT NULL,
    payload_hash char(64) NOT NULL, payload_snapshot json, signature_valid tinyint(1) NOT NULL,
    processing_result varchar(30) NOT NULL, failure_reason varchar(500) NOT NULL,
    response_status int NOT NULL, response_body varchar(64) NOT NULL,
    request_time datetime NOT NULL, create_time datetime NOT NULL
);
INSERT INTO reader_payment_callback_log VALUES
    (9007199254743801,'epusdt',9007199254743701,'RECHARGE-FIXTURE','TRADE-FIXTURE','203.0.113.9','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{"status":2,"trade_id":"TRADE-FIXTURE"}',1,'success','',200,'ok','2026-06-18 00:30:00','2026-06-18 00:30:01'),
    (9007199254743802,'epusdt',NULL,'BAD-CALLBACK','','203.0.113.10','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','{"status":"unexpected"}',0,'unknown','invalid fixture',400,'fail','2026-06-18 00:31:00','2026-06-18 00:31:01');
