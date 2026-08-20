# 旧数据迁移映射总表

## 状态说明

本文件记录 M0 的数据域范围，后续每个里程碑补齐目标 PostgreSQL 表、字段转换、MinIO 对象、校验 SQL 和迁移测试。旧 MySQL 只读，旧管理系统框架数据不迁移。

2026-08-16 模块边界整改没有改变旧库表、目标表、ID 或字段映射。Reader 账号迁移仍在每批 PostgreSQL 事务内同步写入 Commerce 自有的 `commerce_reader_search_projection`，账号、投影和 checkpoint 同成同败；迁移和运行时业务代码均通过注入合同写入，不允许 Commerce 直接读取 `reader_accounts`。真实 MySQL 到 PostgreSQL 回归、回滚和幂等重跑通过，隔离验收库的 `id/username/nickname/status` 双向差异为 0。约 8 GB 完整副本演练仍属于 M6，未因本次整改提前关闭。

| 数据域 | 主要旧表/来源 | 目标 | ID 规则 | 校验重点 | 状态 |
| --- | --- | --- | --- | --- | --- |
| 分类 | 现行 `sys_dict_data` 的 `novel_book_category` / `novel_book_sub_category`、历史 `book_category` | `novel_categories` | `dict_code`/历史分类 ID 原值写入 bigint；`dict_value` 保留为稳定 code | code、名称、类型、排序、启停及来源 | M2 元数据切片已实现 |
| 书籍 | 当前 `novel_book`、`reader_product(product_type='book')`、`novel_book_sub_category_rel`；更早 `book` 待 M6 盘点决定是否仍有有效独有数据 | `novel_books`、`novel_book_sub_categories`、`novel_book_tags` | 当前书籍与关联 ID 原值写入 bigint；新序列推进到迁移最大值之后 | 行数、分类、作者、状态、定价、字数、末章、副分类及 Long ID | M2 当前书籍切片已实现；更早历史表待 M6 完整性盘点 |
| 作者 | 当前 `novel_book.author_id/author_name`、历史 `book_author`、`author` | `novel_authors`；书籍关联在 M2 书籍切片补齐 | 当前/历史作者 ID 原值优先作为 bigint 主键；更早 `author` 同时保留 `legacy_author_id` | 当前书籍作者优先，不按同名擅自合并不同旧 ID，孤立作者显式保留 | M2 元数据切片已实现 |
| 章节元数据 | 当前 `novel_chapter`；更早 `book_index` 待 M6 盘点决定是否仍有有效独有数据 | `novel_chapters` | 当前章节 ID 原值写入 bigint；新序列推进到迁移最大值之后 | 章节序号、书籍关联、状态、字数、价格、Long ID | M2 当前章节切片已实现；更早历史表待 M6 完整性盘点 |
| 章节正文 | 当前 `novel_chapter_content`；更早 `book_content0..9` 和 TXT 文件待 M5/M6 接入 | MinIO `chapters/{bookId}/{chapterId}/v{version}.txt` + `novel_objects` / `novel_object_references` | book/chapter ID 原值进入对象键和 bigint 关联 | UTF-8、16 MiB 上限、Stat 字节数与 SHA-256、活动引用、重跑对象数 | M2 当前章节正文迁移已实现；更早正文来源待 M5/M6 |
| 封面和附件 | `novel_book.cover_url`、`sys_file/sys_oss`、导入原文件 | 封面使用 MinIO `covers/{bookId}/v{version}.{ext}`；其他文件后续按独立命名空间接入同一对象注册模式 | 书籍 ID 原值进入对象键；URL 仅保存 SHA-256 来源指纹 | SSRF 防护、超时、10 MiB 上限、魔数、大小、哈希、活动引用、显式错误 | M2 当前书籍封面迁移已实现；其他附件待 M5/M6 |
| 读者账号 | 当前 `reader_user`；仅当该表不存在时回退历史 `user` | `reader_accounts` | 原值保留；密码只保存摘要并标记 `bcrypt`/`md5` | 用户名唯一、状态、坏摘要错误清单；旧 `token_version` 和 Token 不迁移，新会话重新登录签发 | M3 映射和真实 MySQL→PostgreSQL 测试已实现 |
| 书架/历史/偏好/点赞 | `reader_bookshelf`、`reader_reading_history`、`reader_reading_preference`、`reader_book_like`、历史对应表 | `reader_bookshelf_entries`、`reader_reading_history`、`reader_reading_preferences`、`reader_book_likes` | ID、书籍/章节/读者关联原值保留；identity 序列推进 | 去重、章节序号、位置类型/值、阅读百分比、字号、行高、主题和 Long ID | M3 映射和真实 MySQL→PostgreSQL 测试已实现 |
| 反馈 | 当前 `reader_feedback`、历史 `user_feedback` | `reader_feedback` | ID 原值保留；`reply_content/reply_time` 映射为 `reply/replied_at` | 状态、回复、回复时间、创建/更新时间；identity 序列推进 | M3 映射和真实 MySQL→PostgreSQL 测试已实现 |
| 钱包与流水 | `reader_wallet`、`reader_wallet_ledger`、`reader_bonus_coin_bucket`、`reader_wallet_adjustment` | `reader_wallets`、`reader_wallet_ledgers`、`reader_bonus_coin_buckets`、`reader_wallet_adjustments` | reader/ledger/bucket/adjustment ID 原值保留并推进 identity 序列；流水 `biz_id` 按十进制字符串保留 | 每个流水余额跃迁、方向、币种、金额和关联；钱包余额/收入/支出由不可变流水重算必须零差异 | M4 `reader-finance` 映射和真实 MySQL→PostgreSQL 核对已实现；完整副本仍待 M6 |
| 商品与订单 | `reader_product`、`reader_order`、历史购买记录 | `commerce_products`、`reader_purchase_orders` | 商品、订单、目标 ID 原值保留并推进订单序列 | `buy_*` 转目标订单类型；金额快照、扣款拆分、状态、幂等键和来源引用 | 当前商品/订单映射已真实验证；更早历史购买表待 M6 盘点 |
| 会员和权益 | `reader_entitlement`、`reader_membership_grant` | `commerce_entitlements`、`commerce_membership_grants` | ID、读者和目标 ID 原值保留 | 有效期、永久标志、来源订单；非法类型/状态/目标写错误清单 | M3/M4 映射和真实 MySQL→PostgreSQL 测试已实现 |
| 签到与邀请 | `reader_checkin_record`、`reader_checkin_reward_rule`、邀请关系/奖励/邀请码 | `reader_checkin_records`、`reader_checkin_reward_rules`、`reader_invite_reward_records` 及现有邀请表 | 原值保留并推进 identity 序列 | 连续天数、唯一日期、奖励分项汇总、关系和读者关联、奖励枚举 | M4 当前表映射和真实 MySQL→PostgreSQL 测试已实现；完整副本仍待 M6 |
| 充值与支付 | `reader_recharge_product`、`reader_recharge_setting`、`reader_payment_channel`、`reader_recharge_order`、`reader_payment_callback_log`、`reader_payment_credential`、历史 `order_pay` | `reader_recharge_*`、`reader_payment_*` | 当前业务事实 ID 原值保留并推进 identity 序列；渠道/凭据 ID 仅保存为旧数字引用 | 金额、订单/回调状态、关联、回调 JSON；商户 PID 和来源 IP 只保存 SHA-256；支付 Secret 不迁移 | M4 当前 EPUSDT 事实映射已真实验证；`reader_payment_credential` 明文/密文均不迁移，历史 `order_pay` 待 M6 盘点 |
| 采集 | `novel_crawl_*`、历史 `crawl_*` | PostgreSQL 内容生产域 | 原值保留 | 来源、游标、候选、任务与日志关联 | `novel_crawl_*` 当前表已由来源/候选/导入任务/抓取日志 stage 接入；历史 `crawl_*` 表启用状态、Cookie Secret 转换、自动发现运行状态仍待完整副本盘点 |
| TXT 导入 | `novel_txt_import_task`、原文件和失败修复记录 | PostgreSQL + MinIO | 原值保留 | 文件哈希、任务状态、章节结果 | `00042` 建立任务事实；上传和失败文件替换都先完成大小/SHA-256 校验，再写任务和 `txt_import` 队列；预览只读已校验对象并限制返回量 |
| 书籍合并 | `novel_book_merge_*` | PostgreSQL + MinIO | 原值保留 | 源/目标书、章节映射、源/目标对象、执行结果 | `00046` 建立任务、来源快照和章节血缘，`00047` 补齐目标外键；新目标章节对象先完成大小/SHA-256 校验，再与目标书、章节、对象引用及源书下架在同一事务提交 |
| AI 配置和模型 | `novel_ai_config`、`novel_ai_config_model` | `novel_ai_config`、`novel_ai_config_model` | 配置和模型 ID 原值保留 | 非秘密参数、有序模型、当前模型、失败阈值/计数、状态版本；密钥改由 Secret 注入 | `00048` 建立目标结构和管理闭环；旧 `api_key` 值禁止写入 PostgreSQL、日志或报告，仅按旧配置 ID 生成 `MOONBOOK_AI_LEGACY_<ID>_API_KEY` 引用并列入迁移后的待注入 Secret 清单，旧单 `model` 在缺少模型子表时作为顺序 1 模型迁移 |
| AI 清洗/摘要/画像 | clean、summary、profile 配置/任务/结果表 | PostgreSQL 内容生产域；清洗正文进入 MinIO | 旧配置、任务和建议 ID 原值保留；新序列推进 | 任务结果、采用状态、失败重试、对象版本、正文哈希、原始/建议/审核快照和 Long ID | `00049` 建立清洗配置/任务/结果，清洗稿经 `chapter_clean` 对象校验后引用；`00050`/`00052` 建立摘要配置/任务并映射旧日志；`00053` 建立画像单例配置和建议事实，`00065` 补齐画像/摘要来源键。旧明文 AI 密钥、原始响应和历史输入正文不迁移，运行中旧任务按失败中断记录导入后人工重试；完整副本对象哈希和 AI 沙箱验收仍待 M6/M7 |
| 读者 SEO | `novel_reader_seo_config` | `novel_reader_seo_config` 单例 | 固定配置 `id=1` 原值保留；额外 ID 不迁移 | 开关、站点字段、模板、时间、单例、错误清单和幂等 | M2 管理配置与迁移已实现；M3 接入公开 SEO、robots、sitemap 契约 |
| 每日活动统计 | `reader_daily_activity` | `reader_daily_activity`、`reader_activity_settings` | `(activity_date,reader_id)` 复合键和读者 bigint 原值保留 | Kuala Lumpur 日期、首次活跃时间、读者关联、按日去重、追踪起始边界 | `reader-activity` 阶段已实现；真实只读 MySQL 8.4 → PostgreSQL 17 覆盖大 ID、复合游标、错误清单与幂等重跑 |

## 明确不迁移

- RuoYi/GVA 管理员、角色、菜单、租户、工作流、代码生成和框架配置数据；
- Redis 缓存、在线会话、临时限流和旧 Token；
- 纯运行日志、构建产物、缓存、临时中间文件；
- 已废弃或无业务追溯价值的任务瞬时数据；
- 生产密钥、Token、私钥和支付凭据明文。

## M4 当前财务字段规则

- `moonbook-legacy-migrate reader-finance` 固定先执行 `reader-identity`、`reader-commerce`，再按钱包、不可变流水、奖励桶、购买订单、签到、邀请奖励、人工调账、充值配置、充值订单和回调日志的依赖顺序迁移。
- `reader_order.order_type` 映射为 `buy_book -> book`、`buy_chapter -> chapter`、`buy_ad_free -> ad_free`、`buy_membership -> membership`、`mock_recharge -> mock_recharge`；保留 `pending/paid/closed/failed` 历史状态。
- 钱包汇总保留旧余额与累计值，但验收必须调用 `commerce/reconcile.Wallets` 按不可变流水重新计算；任一币种的余额、收入或支出差异均阻止切换。
- 旧 `reader_payment_credential.encrypted_secret` 和任何支付密钥不进入 PostgreSQL。充值订单只保留旧渠道 ID、凭据 ID 数字引用和商户 PID 的 SHA-256，回调来源 IP 只保存 SHA-256。
- `payload_snapshot` 仅迁移旧 DDL 明确标注的已解析非敏感 JSON；迁移前验证 JSON，有效载荷不写日志或错误消息。无效枚举、金额、余额跃迁和关联进入 `migration_errors`，不静默丢弃。
- 详细命令、合成数据覆盖和本次真实依赖结果见 `docs/migration/m4-reader-finance.md`；约 8 GB 完整副本的逐表、财务和耗时验收仍属于 M6。

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

## M2 读者 SEO 字段规则

- 目标表固定为单例 `novel_reader_seo_config(id=1)`；管理 JSON 的 `id` 输出字符串，PUT 只接受 12 个可编辑字段，不接受客户端提交 ID 或时间。
- 旧 `seo_enabled`、`indexing_enabled`、`sitemap_enabled` 的 `0/1` 转为 PostgreSQL boolean；站点名称、根 URL、默认描述、首页文案、书库模板、书籍模板和创建/更新时间完整迁移。
- 站点 URL 只能是无用户信息、路径、查询和片段的 HTTP/HTTPS 根地址；模板只允许各页面上下文声明的占位符，文本非空且遵守目标长度限制。
- 旧表缺失记录 `SOURCE_TABLE_NOT_FOUND`，额外 ID 记录 `UNEXPECTED_CONFIG_ID`，字段无效记录 `INVALID_SEO_CONFIG`；这些业务错误保留新库默认单例并继续其他迁移，不发生部分覆盖。
- `moonbook-legacy-migrate novel-reader-seo` 使用固定 ID 游标和同事务 checkpoint，重复执行不增加目标行。真实只读 MySQL 验证覆盖合法、无效、缺表、额外 ID 与幂等分支。

## M6 完整性门槛

- 所有源业务表必须有明确目标或“不迁移”理由；
- 所有目标字段必须有转换规则和空值策略；
- 所有金额与权益必须有双向核对 SQL；
- 所有 MinIO 对象必须有来源、对象键、字节数和哈希；
- `novel_objects` 只在 MinIO 上传并以 `StatObject` 校验后进入 `verified`；`novel_object_references` 只能通过复合外键引用同一 kind/book/owner 的对象；
- 激活新版本和旧版本转 `orphaned` 在同一 PostgreSQL 事务完成，上传成功但事务未提交的对象保持无引用；
- 回收只处理超过宽限期且无引用的 `uploading`、`failed`、`orphaned` 或可恢复 `deleting` 对象，删除过程通过事件表审计并支持进程崩溃后幂等收敛；
- 所有转换错误必须进入错误清单，禁止静默丢弃。
