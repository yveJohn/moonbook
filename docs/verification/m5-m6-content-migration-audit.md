# M5/M6 内容生产迁移审计

审计日期：2026-08-20

## 结论

M5 的本地代码与自动化门已闭合：长任务恢复覆盖论坛导入、TXT、清洗、摘要、画像和自动发现，六类 Worker 的重载/失租接管有真实依赖证据；论坛 Cookie 与 AI Key 只迁移为 Secret 引用。M6 的 `all` Stage 覆盖合同、双库行数/主键/关联、Full 财务和 MinIO 逐对象字节/SHA-256 核对均已实现并完成合成故障注入。真实论坛/AI 调用属于外部验收，不是代码缺口。完整副本仍为 No-Go：16 个 TXT 任务原文件及受控清单缺失，因此目标业务迁移尚未运行，M6 不能关闭。

## 旧数据范围

冻结旧库至少包含以下当前业务事实：

- 论坛采集：`novel_crawl_forum_source`、`novel_crawl_forum_board`、`novel_crawl_thread_candidate`、`novel_crawl_import_task`、`novel_crawl_fetch_log`；
- 旧运行开关：`novel_crawl_runtime_task`；
- TXT 导入：`novel_txt_import_task` 及上传原文件或失败修复文件；
- 书籍合并：`novel_book_merge_task`、`novel_book_merge_source`、`novel_book_merge_chapter`；现已接入任务、源书、章节明细三阶段，按已迁移章节和对象重新建立血缘；
- AI 配置：`novel_ai_config`、`novel_ai_config_model`；
- 章节清洗：`novel_chapter_clean_config`、`novel_chapter_clean_task`、`novel_chapter_clean_result`；更早版本使用 `ai_chapter_clean_config`、`ai_chapter_clean_task`、`ai_chapter_clean_result`，现已由清洗 stage 自动识别并映射；
- 章节摘要：`novel_chapter_summary_config`、`novel_chapter_summary_task_log`；现已接入摘要配置和任务日志阶段，敏感 API Key/原始响应不迁移；
- 作品画像：`novel_book_profile_config`、`novel_book_profile_suggestion`；现已接入配置和建议阶段，使用 `00065` 来源键迁移，原始 AI 响应不迁移而保留结构化 JSON 快照。

更早的 `crawl_source`、`crawl_single_task`、`crawl_batch_task`、`crawl_board`、`crawl_board_thread`、`crawl_forum_source_state` 仍需由 M6 完整副本盘点确定行数和启用状态，再逐表给出迁移目标或“不迁移”理由。不得仅凭当前 Java 模块未引用就静默忽略。

## 当前编排证据

`server/cmd/moonbook-legacy-migrate/main.go` 接受以下业务命令：

`all`、`preflight`、`novel-metadata`、`novel-books`、`novel-chapters`、`novel-chapter-clean`、`novel-chapter-summary`、`novel-book-profile`、`novel-book-merge`、`novel-crawl-sources`、`novel-crawl-candidates`、`novel-crawl-import-tasks`、`novel-crawl-fetch-logs`、`novel-txt-imports`、`novel-ai-configs`、`novel-reader-seo`、`reader-identity`、`reader-commerce`、`reader-finance`、`reader-activity`。

`all` 在小说章节之后组合 AI 配置、章节清洗任务/结果、章节摘要配置/任务、作品画像配置/建议、书籍合并任务/源书/章节明细、四个论坛 Stage 和 `novel-txt-imports`；独立命令可按依赖顺序幂等重跑。清洗结果经 MinIO 大小/SHA-256 校验并以 `chapter_clean` 对象激活，旧 `raw_response` 不迁移。Stage 注册集合与审计覆盖清单双向比较，遗漏或多余 Stage 会使自动测试失败。

内容生产迁移已在小说、章节和 Reader Stage 之后按外键依赖执行，并拆为可独立重跑的阶段；实际顺序满足：

1. 来源和板块先于候选、导入任务和抓取日志；
2. AI 配置和模型先于清洗、摘要和画像配置；
3. 清洗任务先于结果；清洗正文完成 MinIO 写入和对象激活后，才能写清洗结果引用；旧 `running` 任务必须转为失败；
4. 论坛导入任务、章节及清洗结果均可引用后，才能迁移书籍合并血缘；合并血缘不得直接引用旧对象 ID；
5. 每个阶段具有独立 checkpoint、错误清单、源表计数和目标核对。

## 幂等与 ID 支持

前向迁移 `00063`、`00064`、`00065` 已为论坛、TXT、清洗、合并和 AI 相关目标表增加 `legacy_source_key` 及局部唯一索引。全部内容 Stage 使用 `moonbook-v1:<table>:<legacy-id>` 或对应稳定来源键，可在 checkpoint 丢失后按来源键证明幂等，并拒绝覆盖具有不同来源键的运行时记录。

冲突必须进入结构化错误清单，禁止覆盖运行时已存在的不同事实；候选 stage 已对来源/板块缺失、书籍缺失、非法状态和非法文本分别记录错误，导入任务 stage 已对候选缺失、目标书籍缺失、非法状态和质量状态分别记录错误，抓取日志 stage 已对关联缺失、非法阶段/状态、非法计数和非法 JSON 分别记录错误。

## 对象迁移合同

以下旧数据不能直接复制到 PostgreSQL：

- 旧 `novel_chapter_clean_result.cleaned_text` 是数据库大文本；目标要求写入 MinIO `chapter-clean` 对象并在大小和 SHA-256 校验后激活引用；
- TXT stage 使用显式 JSON 清单把旧任务 ID 映射到受限根目录内的原文件，拒绝路径逃逸、缺失、空文件、超限和大小不符；上传对象按确定键写入并校验大小/SHA-256。完整副本仍需提供实际文件清单，缺失项必须清零或完成人工处置；
- 书籍合并目标血缘引用原始或清洗对象 ID，必须解析已经迁移并激活的章节/清洗对象，不能沿用旧库不存在的对象 ID；
- 旧 AI `raw_response` 和输入快照可能包含大段正文或敏感诊断，迁移时必须遵守 7 天保留策略和报告脱敏要求。

每个对象 stage 必须核对源对象数、成功数、缺失数、总字节数和逐对象哈希；PostgreSQL 事务失败后的已上传对象必须保持无活动引用并进入孤立对象回收流程。

## 第三方秘密边界

AI 迁移将旧 `api_key` 转为环境变量引用和待注入清单，不写入 PostgreSQL。迁移 `00069` 已把论坛 Cookie 改为 `MOONBOOK_FORUM_COOKIE_*` Secret 引用；API、页面、Worker 和迁移不再读写明文 `cookie_text`，日志与连接检查响应均脱敏。真实值只由受控环境注入，证据见 `docs/verification/m5-forum-cookie-secret-reference.md`。

## 运行任务能力

旧 `novel_crawl_runtime_task` 提供 `forumDiscover` 和 `forumImport` 运行开关、状态、计数和停止请求。新平台 Worker 已替代论坛导入的领取、租约、恢复和停止语义；`candidate.DiscoveryWorker` 提供等价的持续自动发现调度。

`novel_crawl_forum_board.auto_follow_enabled`、`follow_interval_minutes` 和 `follow_import_limit` 已由 `candidate.DiscoveryWorker` 纳入应用生命周期：每轮最多扫描 20 个到期板块，按 PostgreSQL 会话 advisory lock 防止多实例重复发现，成功后才更新 `last_follow_time`，单板抓取失败不会停止整个调度器。该调度器仍只负责候选发现，导入执行继续由 `platform_jobs` 的论坛导入 Worker 负责。

自动发现的调度、Secret 引用、SSRF 和限流合同已完成；真实第三方论坛凭据与联网验收按 `docs/runbooks/external-validation.md` 单独执行。

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

`server/cmd/moonbook-migration-audit` 输出固定表映射、主键范围、legacy 来源键、checkpoint/错误数、关联和对象汇总，并通过 `ObjectVerifier` 流式执行 MinIO Stat/Get 的实际字节与 SHA-256 核对；`moonbook-finance-reconcile` 覆盖全域财务。故障注入见 `docs/verification/m6-global-migration-object-audit.md`。完整副本因 16 个 TXT 原文件缺失在目标写入前停止，不能把核对器的合成通过冒充完整数据通过。

## 退出门

M5 本地运行能力已完成；M6 内容迁移只有在以下证据全部齐全后才能关闭：

1. `[x]` 内容生产 Stage 纳入独立命令和 `all` 编排；
2. `[x]` 前向迁移提供 legacy 来源键、唯一约束和序列推进能力；
3. `[x]` 隔离只读 MySQL、真实 PostgreSQL 和 MinIO 集成测试覆盖中断恢复与幂等重跑；
4. `[x]` 对象核对器对正文和文件执行大小与实际 SHA-256 校验，缺失项阻止切换；
5. `[x]` 自动发现/自动跟进能力完成；
6. `[x]` 第三方 Secret 不写入数据库和仓库；
7. `[ ]` 完整约 8 GB 副本演练给出业务 Stage、行数、对象、财务、耗时和差异报告；当前被 16 个 TXT 原文件清单阻断。
