# M5/M6 内容生产迁移审计

审计日期：2026-08-20

## 结论

M5 的运行时长任务恢复证据已经覆盖论坛导入、TXT 导入、章节清洗、章节摘要和作品画像。当前已新增内容生产 legacy 来源键，并将论坛来源/板块、候选帖子、导入任务、抓取日志、TXT 任务/原文件和 AI 配置/模型 stage 接入独立命令和 `all` 编排；这些 stage 保留旧 ID、按来源键幂等重跑，历史运行任务不会自动访问外部系统，抓取日志缺失关联会保留为 NULL 并进入错误清单，TXT 文件必须通过显式清单、大小和 SHA-256 校验，旧 AI Key 只转换为 Secret 引用，并且不把 Cookie 或 Key 明文写入 PostgreSQL。清洗、合并、AI 任务结果等其余内容生产表、双库集成报告和全域核对器仍未完成，因此 M5 和 M6 不能关闭。

## 旧数据范围

冻结旧库至少包含以下当前业务事实：

- 论坛采集：`novel_crawl_forum_source`、`novel_crawl_forum_board`、`novel_crawl_thread_candidate`、`novel_crawl_import_task`、`novel_crawl_fetch_log`；
- 旧运行开关：`novel_crawl_runtime_task`；
- TXT 导入：`novel_txt_import_task` 及上传原文件或失败修复文件；
- 书籍合并：`novel_book_merge_task`、`novel_book_merge_source`、`novel_book_merge_chapter`；现已接入任务、源书、章节明细三阶段，按已迁移章节和对象重新建立血缘；
- AI 配置：`novel_ai_config`、`novel_ai_config_model`；
- 章节清洗：`novel_chapter_clean_config`、`novel_chapter_clean_task`、`novel_chapter_clean_result`；更早版本使用 `ai_chapter_clean_config`、`ai_chapter_clean_task`、`ai_chapter_clean_result`，现已由清洗 stage 自动识别并映射；
- 章节摘要：`novel_chapter_summary_config`、`novel_chapter_summary_task_log`；
- 作品画像：`novel_book_profile_config`、`novel_book_profile_suggestion`。

更早的 `crawl_source`、`crawl_single_task`、`crawl_batch_task`、`crawl_board`、`crawl_board_thread`、`crawl_forum_source_state` 仍需由 M6 完整副本盘点确定行数和启用状态，再逐表给出迁移目标或“不迁移”理由。不得仅凭当前 Java 模块未引用就静默忽略。

## 当前编排缺口

`server/cmd/moonbook-legacy-migrate/main.go` 当前接受以下业务命令：

`novel-metadata`、`novel-books`、`novel-chapters`、`novel-chapter-clean`、`novel-reader-seo`、`reader-identity`、`reader-commerce`、`reader-finance`、`reader-activity`。

`all` 已在小说章节之后组合 AI 配置、章节清洗任务/结果、书籍合并任务/源书/章节明细、四个论坛 stage 和 `novel-txt-imports`；`novel-chapter-clean` 与 `novel-book-merge` 可按依赖顺序独立重跑。清洗结果正文经 MinIO 大小/SHA-256 校验并以 `chapter_clean` 对象激活，旧 `raw_response` 不迁移。AI 摘要/画像配置之外仍存在待补齐的内容生产 stage。

内容生产迁移必须在已有小说、章节和 Reader stage 之后按外键依赖执行，并至少拆为可独立重跑的阶段。建议依赖顺序由后续设计确定，但必须满足：

1. 来源和板块先于候选、导入任务和抓取日志；
2. AI 配置和模型先于清洗、摘要和画像配置；
3. 清洗任务先于结果；清洗正文完成 MinIO 写入和对象激活后，才能写清洗结果引用；旧 `running` 任务必须转为失败；
4. 论坛导入任务、章节及清洗结果均可引用后，才能迁移书籍合并血缘；合并血缘不得直接引用旧对象 ID；
5. 每个阶段具有独立 checkpoint、错误清单、源表计数和目标核对。

## 幂等与 ID 支持缺口

前向迁移 `00063` 已为论坛、TXT、清洗、合并和 AI 相关目标表增加可空 `legacy_source_key` 及局部唯一索引。已实现的论坛、TXT 和 AI 配置/模型 stage 使用 `moonbook-v1:<table>:<legacy-id>`，可在 checkpoint 丢失或更换 migration name 后按来源键证明幂等，并拒绝覆盖具有不同来源键的运行时记录。其余 stage 接入时必须复用同一约束。

冲突必须进入结构化错误清单，禁止覆盖运行时已存在的不同事实；候选 stage 已对来源/板块缺失、书籍缺失、非法状态和非法文本分别记录错误，导入任务 stage 已对候选缺失、目标书籍缺失、非法状态和质量状态分别记录错误，抓取日志 stage 已对关联缺失、非法阶段/状态、非法计数和非法 JSON 分别记录错误。

## 对象迁移缺口

以下旧数据不能直接复制到 PostgreSQL：

- 旧 `novel_chapter_clean_result.cleaned_text` 是数据库大文本；目标要求写入 MinIO `chapter-clean` 对象并在大小和 SHA-256 校验后激活引用；
- TXT stage 使用显式 JSON 清单把旧任务 ID 映射到受限根目录内的原文件，拒绝路径逃逸、缺失、空文件、超限和大小不符；上传对象按确定键写入并校验大小/SHA-256。完整副本仍需提供实际文件清单，缺失项必须清零或完成人工处置；
- 书籍合并目标血缘引用原始或清洗对象 ID，必须解析已经迁移并激活的章节/清洗对象，不能沿用旧库不存在的对象 ID；
- 旧 AI `raw_response` 和输入快照可能包含大段正文或敏感诊断，迁移时必须遵守 7 天保留策略和报告脱敏要求。

每个对象 stage 必须核对源对象数、成功数、缺失数、总字节数和逐对象哈希；PostgreSQL 事务失败后的已上传对象必须保持无活动引用并进入孤立对象回收流程。

## 第三方秘密缺口

批准设计要求第三方凭据只通过环境变量或 Secret 注入。AI 迁移映射已明确旧 `api_key` 不写入 PostgreSQL，而是生成环境变量引用和待注入清单。

论坛 Cookie 当前不满足同一约束：`novel_crawl_forum_source.cookie_text` 在 PostgreSQL 中保存明文，管理页面也允许直接提交 Cookie，Worker 从数据库读取并发送。迁移旧 `cookie_text` 会把第三方会话凭据继续写入目标库。

在内容迁移前必须完成论坛凭据存储设计：目标表只保存 Secret 引用和配置状态，旧 Cookie 值不进入 PostgreSQL、日志、测试快照或迁移报告。真实值只能由受控环境在部署时注入。

## 运行任务能力缺口

旧 `novel_crawl_runtime_task` 提供 `forumDiscover` 和 `forumImport` 运行开关、状态、计数和停止请求。新平台 Worker 已替代论坛导入的领取、租约、恢复和停止语义，但没有与旧 `forumDiscover` 等价的持续自动发现调度器。

`novel_crawl_forum_board.auto_follow_enabled`、`follow_interval_minutes` 和 `follow_import_limit` 当前只被管理 CRUD 保存，运行时代码没有调度扫描。管理功能矩阵中的“运行任务”不能标记为被 `platform_jobs` 完整替代，自动发现/自动跟进仍是 M5 运行能力缺口。

旧 runtime 行本身不应按 `running=true` 原样恢复。运行中或待执行任务迁移后必须进入明确的中断/待人工重试状态，避免切换时未经审核访问外部论坛或重复导入。

## 状态转换与核对

旧库使用的 `success`、`pending`、`running`、`failed`、`stopped` 等状态与新库的 `succeeded`、`cancelled`、`completed`、`pending_review` 等枚举并不完全一致。每张表都需要显式转换表和非法值错误码。

最终核对至少覆盖：

- 每张旧表的总行数、成功数、错误数和明确跳过数守恒；
- 所有旧 bigint 主键原值保留且新序列大于迁移最大值；
- 来源、板块、候选、任务、日志、书籍和章节关联无孤儿；
- 每个目标业务任务与 `platform_jobs` 的状态和尝试次数一致；
- 清洗/摘要/画像的活动唯一约束、审核状态和重试血缘一致；
- TXT、清洗和合并对象数量、字节数、SHA-256 及活动引用零差异；
- 旧 AI Key、论坛 Cookie 和完整敏感正文不出现在 PostgreSQL 非对象列、日志或报告中。

## 退出门

M5/M6 的内容生产迁移部分只有在以下证据齐全后才能关闭：

1. 内容生产 stage 纳入独立命令和 `all` 编排；
2. 前向迁移提供 legacy 来源键、唯一约束和序列推进能力；
3. 隔离只读 MySQL、真实 PostgreSQL 和 MinIO 集成测试覆盖中断恢复与幂等重跑；
4. 旧数据库正文和文件全部通过对象大小与哈希校验，缺失项进入错误清单；
5. 自动发现/自动跟进能力完成或经用户确认从有效范围移除；
6. 第三方 Secret 不写入数据库和仓库；
7. 完整约 8 GB 副本演练给出行数、对象、耗时和差异报告。
