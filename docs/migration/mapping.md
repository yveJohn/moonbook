# 旧数据迁移映射总表

## 状态说明

本文件记录 M0 的数据域范围，后续每个里程碑补齐目标 PostgreSQL 表、字段转换、MinIO 对象、校验 SQL 和迁移测试。旧 MySQL 只读，旧管理系统框架数据不迁移。

| 数据域 | 主要旧表/来源 | 目标 | ID 规则 | 校验重点 | 状态 |
| --- | --- | --- | --- | --- | --- |
| 分类 | 现行 `sys_dict_data` 的 `novel_book_category` / `novel_book_sub_category`、历史 `book_category` | `novel_categories` | `dict_code`/历史分类 ID 原值写入 bigint；`dict_value` 保留为稳定 code | code、名称、类型、排序、启停及来源 | M2 元数据切片已实现 |
| 书籍 | 当前 `novel_book`、`reader_product(product_type='book')`、`novel_book_sub_category_rel`；更早 `book` 待 M6 盘点决定是否仍有有效独有数据 | `novel_books`、`novel_book_sub_categories`、`novel_book_tags` | 当前书籍与关联 ID 原值写入 bigint；新序列推进到迁移最大值之后 | 行数、分类、作者、状态、定价、字数、末章、副分类及 Long ID | M2 当前书籍切片已实现；更早历史表待 M6 完整性盘点 |
| 作者 | 当前 `novel_book.author_id/author_name`、历史 `book_author`、`author` | `novel_authors`；书籍关联在 M2 书籍切片补齐 | 当前/历史作者 ID 原值优先作为 bigint 主键；更早 `author` 同时保留 `legacy_author_id` | 当前书籍作者优先，不按同名擅自合并不同旧 ID，孤立作者显式保留 | M2 元数据切片已实现 |
| 章节元数据 | 当前 `novel_chapter`；更早 `book_index` 待 M6 盘点决定是否仍有有效独有数据 | `novel_chapters` | 当前章节 ID 原值写入 bigint；新序列推进到迁移最大值之后 | 章节序号、书籍关联、状态、字数、价格、Long ID | M2 当前章节切片已实现；更早历史表待 M6 完整性盘点 |
| 章节正文 | 当前 `novel_chapter_content`；更早 `book_content0..9` 和 TXT 文件待 M5/M6 接入 | MinIO `chapters/{bookId}/{chapterId}/v{version}.txt` + `novel_objects` / `novel_object_references` | book/chapter ID 原值进入对象键和 bigint 关联 | UTF-8、16 MiB 上限、Stat 字节数与 SHA-256、活动引用、重跑对象数 | M2 当前章节正文迁移已实现；更早正文来源待 M5/M6 |
| 封面和附件 | `novel_book.cover_url`、`sys_file/sys_oss`、导入原文件 | 封面使用 MinIO `covers/{bookId}/v{version}.{ext}`；其他文件后续按独立命名空间接入同一对象注册模式 | 书籍 ID 原值进入对象键；URL 仅保存 SHA-256 来源指纹 | SSRF 防护、超时、10 MiB 上限、魔数、大小、哈希、活动引用、显式错误 | M2 当前书籍封面迁移已实现；其他附件待 M5/M6 |
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

## M2 当前书籍字段规则

- `book_status`：`0 -> serializing`，`1 -> completed`；其他值进入 `INVALID_BOOK_STATUS`。
- `publish_status`：`0 -> draft`，`1 -> published`，`2 -> deprecated`；其他值进入 `INVALID_PUBLISH_STATUS`。
- `charge_mode` 保留 `word_charge`、`membership_only`、`login_free`、`fixed_price`；整书定价从唯一的 `reader_product(product_type='book', target_id=book.id)` 读取 `price_coin`，非整书定价目标价格必须为 `NULL`。
- 主分类按当前分类 code 解析，作者优先按旧 ID 关联，不对不同旧 ID 的同名作者擅自合并；缺失主分类、作者或重复书名作者组合均进入错误清单。
- 副分类关系按旧关系 ID 游标迁移，目标保存分类 ID、code、名称和排序；坏分类、缺书或非法排序显式报错。
- `score`、计数、精选、采集状态和文本长度均在写入前校验；单行业务坏数据继续下一行，SQL、连接或事务错误回滚整批。
- `legacy_cover_url` 保留作迁移来源和人工排错；下载器默认拒绝私网/环回/非标准端口，只有显式主机白名单可访问受控内网。对象通过 MinIO `Put` + `Stat` 大小/SHA-256 校验后才激活引用。

## M2 当前章节字段规则

- `novel_chapter.id`、`book_id` 和 `book_price` 原值写入 bigint；管理 JSON 的 `id`、`bookId`、`bookPriceCoin` 全部输出字符串，目标 identity 序列推进到最大迁移 ID 之后。
- `chapter_status`：`0 -> enabled`，`1 -> disabled`；`ai_clean_status`：`0..6` 依次映射为 `pending`、`cleaning`、`cleaned`、`discarded`、`failed`、`expired`、`skipped`。不支持的值进入错误清单。
- 当前正文只读取 `novel_chapter_content.content`。缺正文、非 UTF-8、超过 `(16 MiB)-1`、缺目标书籍或非法章节字段均逐行记录，不产生章节元数据或活动对象引用。
- 字数按 Unicode code point 排除空白重新计算；与旧 `word_count` 不一致时仍迁移，并记录 `WORD_COUNT_RECALCULATED`。书籍总字数和末章统计在章节与正文引用激活的同一事务内刷新。
- 正文先写入 MinIO 并通过对象键、字节数、SHA-256 校验，再在单个 PostgreSQL 事务内写章节元数据、切换活动引用并更新书籍统计。事务失败的已验证对象转为无引用 `orphaned`，等待宽限期回收。
- 旧来源指纹由章节 ID、更新时间和正文 SHA-256 生成；即使 checkpoint 丢失并使用新的 migration name 重跑，也复用已成功的 legacy 对象，不增加对象版本。目标中同 ID 的非 legacy 章节不会被覆盖。

## M6 完整性门槛

- 所有源业务表必须有明确目标或“不迁移”理由；
- 所有目标字段必须有转换规则和空值策略；
- 所有金额与权益必须有双向核对 SQL；
- 所有 MinIO 对象必须有来源、对象键、字节数和哈希；
- `novel_objects` 只在 MinIO 上传并以 `StatObject` 校验后进入 `verified`；`novel_object_references` 只能通过复合外键引用同一 kind/book/owner 的对象；
- 激活新版本和旧版本转 `orphaned` 在同一 PostgreSQL 事务完成，上传成功但事务未提交的对象保持无引用；
- 回收只处理超过宽限期且无引用的 `uploading`、`failed`、`orphaned` 或可恢复 `deleting` 对象，删除过程通过事件表审计并支持进程崩溃后幂等收敛；
- 所有转换错误必须进入错误清单，禁止静默丢弃。
