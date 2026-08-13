# 旧数据迁移映射总表

## 状态说明

本文件记录 M0 的数据域范围，后续每个里程碑补齐目标 PostgreSQL 表、字段转换、MinIO 对象、校验 SQL 和迁移测试。旧 MySQL 只读，旧管理系统框架数据不迁移。

| 数据域 | 主要旧表/来源 | 目标 | ID 规则 | 校验重点 | 状态 |
| --- | --- | --- | --- | --- | --- |
| 分类 | 现行 `sys_dict_data` 的 `novel_book_category` / `novel_book_sub_category`、历史 `book_category` | `novel_categories` | `dict_code`/历史分类 ID 原值写入 bigint；`dict_value` 保留为稳定 code | code、名称、类型、排序、启停及来源 | M2 元数据切片已实现 |
| 书籍 | `novel_book`、历史 `book`、分类/子分类关系 | PostgreSQL 小说域 | 原值保留 | 行数、分类、状态、字数、末章 | 待 M2 设计 |
| 作者 | 当前 `novel_book.author_id/author_name`、历史 `book_author`、`author` | `novel_authors`；书籍关联在 M2 书籍切片补齐 | 当前/历史作者 ID 原值优先作为 bigint 主键；更早 `author` 同时保留 `legacy_author_id` | 当前书籍作者优先，不按同名擅自合并不同旧 ID，孤立作者显式保留 | M2 元数据切片已实现 |
| 章节元数据 | `novel_chapter`、历史 `book_index` | PostgreSQL 章节 | 原值保留 | 章节序号、书籍关联、状态、字数 | 待 M2 设计 |
| 章节正文 | `novel_chapter_content`、`book_content0..9`、TXT 文件 | MinIO `chapters/{bookId}/{chapterId}/v{version}.txt` + `novel_objects` / `novel_object_references` | book/chapter ID 原值进入对象键和 bigint 关联 | 上传后 Stat 校验字节数与 SHA-256，通过后才允许事务切换引用 | M2 版本化对象服务已实现；待章节迁移接入 |
| 封面和附件 | 书籍 URL、`sys_file/sys_oss`、导入原文件 | 封面使用 MinIO `covers/{bookId}/v{version}.{ext}`；其他文件后续按独立命名空间接入同一对象注册模式 | 业务 ID 原值 | 下载可用性、大小、哈希、来源 | M2 封面对象能力已实现；待书籍迁移接入 |
| 读者账号 | `reader_user`、历史 `user` | PostgreSQL 读者域 | 原值保留 | 用户名唯一、状态、密码摘要 | 待 M3 设计 |
| 书架/历史/偏好/点赞 | `reader_bookshelf`、`reader_reading_history`、`reader_reading_preference`、`reader_book_like`、历史对应表 | PostgreSQL 读者域 | 原值保留 | 外键、去重、进度、Long ID | 待 M3 设计 |
| 反馈 | `reader_feedback`、历史 `user_feedback` | PostgreSQL 读者域 | 原值保留 | 状态、回复和时间 | 待 M3 设计 |
| 钱包与流水 | `reader_wallet`、`reader_wallet_ledger`、奖励桶和调整表 | PostgreSQL 交易域 | 原值保留 | 余额=流水汇总、方向、币种 | 待 M4 设计 |
| 商品与订单 | `reader_product`、`reader_order`、历史购买记录 | PostgreSQL 交易域 | 原值保留 | 金额快照、幂等键、权益 | 待 M4 设计 |
| 会员和权益 | `reader_entitlement`、`reader_membership_grant` | PostgreSQL 交易域 | 原值保留 | 有效期、永久标志、来源订单 | 待 M4 设计 |
| 签到与邀请 | `reader_checkin_record`、奖励规则、邀请关系/奖励/邀请码 | PostgreSQL 运营域 | 原值保留 | 连续天数、唯一日期、奖励汇总 | 待 M4 设计 |
| 充值与支付 | `reader_recharge_*`、`reader_payment_*`、历史 `order_pay` | PostgreSQL 支付域 | 原值保留 | 金额、状态、回调幂等、凭据不明文迁移 | 待 M4 设计 |
| 采集 | `novel_crawl_*`、历史 `crawl_*` | PostgreSQL 内容生产域 | 原值保留 | 来源、游标、候选、任务与日志关联 | 待 M5 设计 |
| TXT 导入 | `novel_txt_import_task`、原文件和失败修复记录 | PostgreSQL + MinIO | 原值保留 | 文件哈希、任务状态、章节结果 | 待 M5 设计 |
| 书籍合并 | `novel_book_merge_*` | PostgreSQL 内容生产域 | 原值保留 | 源/目标书、章节映射、执行结果 | 待 M5 设计 |
| AI 配置和模型 | `novel_ai_config*` | PostgreSQL 配置域 | 原值保留 | 非秘密参数；密钥改由 Secret 注入 | 待 M5 设计 |
| AI 清洗/摘要/画像 | clean、summary、profile 配置/任务/结果表 | PostgreSQL 内容生产域 | 原值保留 | 任务结果、采用状态、失败重试 | 待 M5 设计 |
| 读者 SEO | `novel_reader_seo_config`、网站配置 | PostgreSQL 配置域 | 原值保留 | robots、sitemap、开关与站点字段 | 待 M2/M3 设计 |
| 每日活动统计 | `reader_daily_activity` | PostgreSQL 分析事实 | 复合键保留 | 日期时区、读者关联、去重 | 待 M4 设计 |

## 明确不迁移

- RuoYi/GVA 管理员、角色、菜单、租户、工作流、代码生成和框架配置数据；
- Redis 缓存、在线会话、临时限流和旧 Token；
- 纯运行日志、构建产物、缓存、临时中间文件；
- 已废弃或无业务追溯价值的任务瞬时数据；
- 生产密钥、Token、私钥和支付凭据明文。

## M6 完整性门槛

- 所有源业务表必须有明确目标或“不迁移”理由；
- 所有目标字段必须有转换规则和空值策略；
- 所有金额与权益必须有双向核对 SQL；
- 所有 MinIO 对象必须有来源、对象键、字节数和哈希；
- `novel_objects` 只在 MinIO 上传并以 `StatObject` 校验后进入 `verified`；`novel_object_references` 只能通过复合外键引用同一 kind/book/owner 的对象；
- 激活新版本和旧版本转 `orphaned` 在同一 PostgreSQL 事务完成，上传成功但事务未提交的对象保持无引用；
- 回收只处理超过宽限期且无引用的 `uploading`、`failed`、`orphaned` 或可恢复 `deleting` 对象，删除过程通过事件表审计并支持进程崩溃后幂等收敛；
- 所有转换错误必须进入错误清单，禁止静默丢弃。
