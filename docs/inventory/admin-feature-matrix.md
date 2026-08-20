# 旧管理后台功能矩阵

## 基线与使用方式

本矩阵基于旧仓库 commit `8e7f57316638d199d7a8d7c964a52a89e13fa281` 的管理页面、API、Controller、Service、测试和 SQL。它是完整迁移的追踪入口；每个功能在对应里程碑完成时必须补充新代码、管理路由、权限、迁移映射和测试证据。

状态不再使用无法验收的“基线已盘点”。每项必须给出新代码/路由、自动测试或运行报告；明确移除的框架能力必须记录批准边界。

统一证据索引：GVA 基座见 `docs/verification/m7-gva-foundation-final.md`；M2 小说核心见 `docs/progress/refactor-status.md` 的 M2 退出结论；M3 Reader/交易契约见 `docs/verification/m7-backend-quality-baseline.md` 与 `docs/verification/m7-module-boundary-workflow-inventory.md`；M4 见 `docs/verification/m4-exit-audit.md`；M5/M6 见 `docs/verification/m5-content-worker-connection-and-recovery.md`、`docs/verification/m5-forum-cookie-secret-reference.md`、`docs/verification/m5-m6-content-migration-audit.md` 和 `docs/verification/m6-global-migration-object-audit.md`。下表中的代码、路由和迁移名与该索引共同构成逐项证据。

## GVA 系统基座（M1）

| 功能 | 旧来源 | 新系统验收 | 状态 |
| --- | --- | --- | --- |
| 管理员登录、JWT、验证码、登出 | RuoYi 系统模块 | 使用 GVA 认证并完成安全基线 | 真实 PostgreSQL/Redis HTTP 验收覆盖验证码登录、首次强制改密、重新登录、注销和 JWT 黑名单撤销；受限角色越权被 Casbin 拒绝 |
| 管理员、角色、菜单、API 权限 | system user/role/menu | GVA 用户、角色、菜单、Casbin；不迁移旧管理员数据 | GVA 源码、页面、种子与 Casbin 已导入；真实 HTTP 验收覆盖角色策略和数据权限，旧管理员及角色数据按批准边界不迁移 |
| 部门、岗位、字典、参数、通知 | system 页面与 Controller | 保留组织、岗位、字典和参数；移除旧通知公告 | 部门、岗位、字典和参数已通过真实管理员 HTTP；旧库 `sys_notice` 仅 2 条同批框架初始化公告，未发现 Moonbook 业务引用，通知公告明确移出范围 |
| 操作日志、登录日志、在线状态 | monitor 页面 | 保留结构化审计；移除在线会话管理 UI | 登录、操作、文件和数据权限日志入口存在，真实 HTTP/浏览器验收通过；旧在线列表/强退为瞬态框架会话能力，明确移除，保留 JWT 注销黑名单、账号禁用及审计 |
| 文件与对象存储配置 | system/oss | 固定 MinIO 业务存储，不迁移通用 OSS 管理 | 旧 `sys_oss` 为 0 行，5 条 `sys_oss_config` 均为框架配置；通用 OSS 页面/配置明确移除，业务对象继续使用 MinIO 大小、SHA-256、引用和回收闭环 |
| 系统健康、配置和任务监控 | monitor/admin/cache/snailjob | 健康、指标、只读配置、持久化任务监控 | 配置 API 只返回白名单与 Secret 状态且不可写；平台任务列表/详情/attempt 只读页、health、metrics 和定时任务均已通过真实 HTTP/浏览器验收，见 `docs/verification/m7-gva-foundation-final.md` |

旧 RuoYi 的租户、工作流、代码生成和 Demo 不是 Moonbook 业务必需项；只有 GVA 基座自身正常运行所需或经实际使用证据确认的能力才保留，不迁移旧框架数据。

## 小说核心（M2）

| 功能 | 旧页面/Controller | 关键操作 | 新系统证据 | 状态 |
| --- | --- | --- | --- | --- |
| 书籍管理 | `novel/book` / `NovelBookController` | 查询、新增、编辑、状态、推荐、分类/子分类、标签、定价、封面 | `novel_books`、`/novel/books`、`view/novel/books`、`moonbook-legacy-migrate novel-books` | M2 书籍纵向切片已实现并通过真实 HTTP/双库/MinIO 验收 |
| 章节管理 | `novel/chapter` / `NovelChapterController` | 目录、正文查看/编辑、状态、统计、删除 | `novel_chapters`、`/novel/chapters`、`view/novel/chapters`、`moonbook-legacy-migrate novel-chapters` | M2 章节元数据、MinIO 正文、迁移和对象回收闭环已通过真实 HTTP/双库/MinIO 验收 |
| 分类与子分类 | 书籍页面及相关 API/SQL | 分类筛选、方向、子分类关系 | `novel_categories`、`novel_book_sub_categories`、`/novel/categories`、`view/novel/metadata` | M2 分类管理与书籍副分类关系已实现 |
| 小说分类 | 书籍领域、现行字典及历史分类表 | 一级/二级分类 CRUD、排序、启停、迁移 | `novel_categories`、`/novel/categories`、`view/novel/metadata`、`novel_book_sub_categories` | M2 元数据及书籍关联已实现 |
| 作者信息 | 书籍领域及旧数据表 | 作者 CRUD、状态、作品方向、历史 ID 与迁移 | `novel_authors`、`/novel/authors`、`view/novel/authors`、`novel_books.author_id` | M2 元数据及书籍关联已实现 |
| SEO 配置 | `novel/readerSeo` / `NovelReaderSeoController` | SEO 开关、站点信息、robots、sitemap | `novel_reader_seo_config`、`/novel/readerSeo/config`、`view/novel/readerSeo`、`moonbook-legacy-migrate novel-reader-seo` | M2 管理配置、权限、审计和迁移已完成；公开 SEO、robots、sitemap 接口在 M3 接入冻结 Reader 契约 |
| 书籍画像 | `novel/bookProfile` / `NovelBookProfileController` | AI 建议、采用、失败重试 | `novel_book_profile_*`、`/novel/bookProfile`、`view/novel/bookProfile` | M5 配置、不可变建议、审核采用、失败重试和快照并发保护已实现 |

## 读者运营与交易（M3/M4）

| 功能 | 旧页面/Controller | 关键操作 | 里程碑 | 状态 |
| --- | --- | --- | --- | --- |
| 读者用户 | `reader/user` / `ReaderUserAdminController` | 查询、详情、启停、重置密码 | M3 | 查询、详情、启停、密码重置已实现；重置统一 bcrypt 并撤销现有会话，真实 PostgreSQL 集成测试覆盖 |
| 书架、历史、偏好、点赞 | 读者 API 与相关表 | 查看读者行为、兼容读者端 | M3 | Reader 自有事实、HTTP 契约及冻结 UI 旅程已完成；旧后台没有独立业务管理页，按批准边界不新增写操作型管理入口，行为通过 Reader 隔离接口和审计证据验收 |
| 反馈处理 | `reader/feedback` / `ReaderFeedbackAdminController` | 列表、详情、回复 | M3 | 列表、详情、单次回复已实现 |
| 运营概览 | `/dashboard/overview` / `DashboardOverviewService` | 用户总量、周期新增、日活、去重周期活跃、30 天趋势 | M4 | `00055`、`GET /dashboard/overview` 和 GVA Dashboard 已实现；按 Kuala Lumpur 自然日采集，30 天趋势零填充，真实 PostgreSQL 与双库迁移测试通过 |
| 邀请码 | `reader/inviteCode` / `ReaderInviteCodeController` | 生成、编辑、删除、状态 | M4 | `00057`、`/reader/inviteCodes` 和管理页面已实现生成、编辑、删除、启停；人工码可编辑邀请码、最大次数、过期时间和备注，读者自动码仅允许编辑状态与备注且禁止删除，真实 PostgreSQL 与浏览器验收通过 |
| 签到奖励规则 | `reader/checkinReward` / `ReaderCheckinRewardRuleController` | CRUD、启停 | M4 | `00025`、`/reader/checkinRules`、管理页面已实现；fixed/random 校验及启用唯一约束有集成测试 |
| 钱包与流水 | `reader/wallet` / `ReaderWalletAdminController` | 余额、流水、调整、签到、邀请奖励 | M4 | 余额、不可变流水、人工调账及“签到记录/邀请奖励”只读页签已实现；请求 ID 幂等、钱包锁、余额保护和桌面/移动 E2E 通过 |
| 消费商品 | `reader/product` / `ReaderProductController` | 书籍、章节、会员、免广告商品及上下架 | M4 | 管理端分页筛选、CRUD、在售状态、目标存在性校验和历史订单删除保护已实现；真实 PostgreSQL 集成测试通过 |
| 消费订单 | `reader/order` / `ReaderOrderAdminController` | 查询、人工充值、确认 | M4 | 管理端列表/详情和筛选已实现；模拟充值按“创建 pending 订单、单独确认入账”两阶段执行，请求/确认流水幂等且操作受审计；退款仍需独立财务审批设计 |
| 会员人工发放 | `reader/user` / `ReaderMembershipGrantController` | 限时或永久会员发放、幂等请求、备注审计 | M4 | `POST /reader/users/:id/membership` 已实现；事务锁定读者账号，会员授予独立叠加且不修改购买订单 |
| 充值产品与设置 | `reader/payment/product` / `ReaderRechargeProductAdminController` | 预设档位、自定义兑换范围 | M4 | 充值档位 CRUD、自定义充值开关/汇率/最小最大范围管理已实现；真实 PostgreSQL 集成测试覆盖非法规则 |
| 支付渠道 | `reader/payment/channel` / `ReaderPaymentChannelAdminController` | 配置、启停、连通性检查、凭据保护 | M4 | 渠道查询、启停、凭据状态和脱敏连通性检查已实现；真实生产网关连通性仍需切换前使用受控凭据验收 |
| 充值订单与回调 | `reader/payment/order` / `ReaderRechargeOrderAdminController` | 创建、详情、回调日志、主动同步、异常状态 | M4 | 列表、筛选、详情、EPUSDT 同步创建、失败分类、活动订单替换、过期释放、历史凭据回调、防重放、人工补单、主动同步和每次回调审计均已验收；Full 财务核对器通过合成边界数据，剩余仅为受控真实最小金额支付外部验收 |

## 内容生产（M5）

| 功能 | 旧页面/Controller | 关键操作 | 新系统证据 | 状态 |
| --- | --- | --- | --- | --- |
| 论坛来源 | `crawl/forumSource` / `CrawlForumSourceController` | CRUD、启停、连接与规则配置 | M5 | 来源 CRUD、Secret 引用、URL/节流校验和脱敏连接检查已实现；SSRF、重定向、超时、响应大小和真实 PostgreSQL 测试通过，真实论坛凭据验收归外部清单 |
| 论坛板块 | `crawl/forumBoard` / `CrawlForumBoardController` | CRUD、增量游标、抓取策略 | M5 | 板块 CRUD、来源关联、引用保护、同来源名称唯一、URL 域名和跟进策略校验已实现；自动发现调度已接入生命周期 |
| 线程候选 | `crawl/threadCandidate` / `CrawlThreadCandidateController` | 筛选、状态、加入导入 | M5 | 候选列表/详情、来源板块筛选、跳过/恢复/删除状态管理已实现；新增受控板块发现 API，按来源与外部帖子 ID 幂等写入候选 |
| 导入任务 | `crawl/importTask` / `CrawlImportTaskController` | 创建、执行、暂停/恢复、重试、日志 | M5 | 导入任务创建、队列入库、同源多页抓取、状态/质量字段、取消、有限重试和人工恢复已实现；真实 PostgreSQL + MinIO 集成测试覆盖成功及重试耗尽终态 |
| 运行任务 | `CrawlRuntimeTaskController` | 调度、租约、恢复、取消 | `platform_jobs`、各内容 Worker、自动发现调度 | 六类内容 Worker 的租约恢复、停止、重试和热重载已验收；`candidate.DiscoveryWorker` 按 auto-follow 配置、advisory lock 和批次限制持续发现，旧 running 行不会未经审核自动恢复 |
| 抓取日志 | `crawl/fetchLog` / `CrawlFetchLogController` | 检索、错误详情、重试线索 | M5 | 抓取日志表、阶段/结果约束、任务/状态筛选和详情管理页面已实现；Worker 逐页记录 URL、状态码、字节数、耗时与失败线索，后页失败保留已成功页面证据 |
| TXT 导入 | `novel/txtImport` / `TxtImportController` | 上传、解析、预览、导入、失败文件修复 | `novel_txt_import_task`、`/novel/txtImports`、`view/novel/txtImports` | 上传、MinIO 校验、持久化队列、章节导入、受限预览和失败任务文件替换已实现 |
| 书籍合并 | `novel/bookMerge` / `NovelBookMergeController` | 候选、章节映射、执行、审计 | `novel_book_merge_*`、`/novel/bookMerges`、`view/novel/bookMerges` | 论坛导入来源筛选、时间排序、重复标记、人工排除、active 清洗稿优先、MinIO 正文复制、事务执行、源书下架和血缘审计已实现（迁移至 `00047`） |
| AI 配置与模型 | `novel/aiConfig` / `NovelAiConfigController` | OpenAI 兼容地址、有序模型、启停、失败切换、秘密保护 | `novel_ai_config*`、`/novel/aiConfigs`、`view/novel/aiConfigs` | 配置 CRUD、启用选项、流式模式、有序模型、连续失败循环切换、状态版本和人工重置已实现（迁移 `00048`）；API Key 改为环境变量 Secret 引用，连通性随 AI 执行器切片验证 |
| 章节清洗 | `novel/chapterClean` / `NovelChapterCleanController` | 配置、任务、结果、采用、失败重试 | `novel_chapter_clean_*`、`/novel/chapterClean`、`view/novel/chapterClean` | 配置、OpenAI 兼容流式/非流式传输、持久化任务、模型失败切换、自动采用、人工采用/丢弃、停止/续跑和失败重洗已实现（迁移 `00049`）；`00051` 前向修复清洗稿对象类型约束 |
| 章节摘要 | `NovelChapterSummaryController` | 配置、批量回填、失败重试 | `novel_chapter_summary_*`、`/novel/chapterSummary`、`view/novel/chapterClean` 简介补全页签 | 单例配置、状态/任务分页及详情、手动与自动批处理、停止/续跑、租约恢复、MinIO 清洗稿读取、JSON/SSE、拒答备用 AI、模型轮换和 7 天诊断清理已实现（迁移 `00050`、`00052`） |
| 书籍画像 | `novel/bookProfile` / `NovelBookProfileController` | 配置、建议、采用、自动应用、重试 | `novel_book_profile_*`、`/novel/bookProfile`、`view/novel/bookProfile` | 单例配置、正文/简介输入降级、持久化 Worker、JSON/SSE、拒答备用 AI、人工审核、自动应用、批量重试和陈旧快照保护已实现（迁移 `00053`） |

## 当前有效集成候选

| 集成 | 代码证据 | M0 结论 |
| --- | --- | --- |
| EPUSDT | 充值、回调 Controller 及 Flyway 表 | 当前业务实现存在；生产是否启用待脱敏环境确认 |
| OpenAI 兼容 AI | AI 配置、模型、清洗、摘要、画像模块 | 当前业务实现存在；具体供应商与模型由新配置迁移 |
| SMTP 邮件 | 仅发现框架登录/工作流/Demo 引用，无 Moonbook 业务调用 | 邮件插件、API 元数据和 Casbin 策略已移除，不纳入重构范围 |
| HTTP 代理 | 采集和 AI 外部请求配置 | 当前业务实现存在；只迁移实际启用配置 |
| 对象存储 | RuoYi OSS 与历史文件字段 | 新系统统一为 MinIO，不迁移旧框架 OSS 配置 |
| Redis | 登录、缓存、任务协调 | 新系统继续使用，但不迁移缓存数据 |

## 完整性检查规则

- 每个旧业务页面必须映射到本矩阵或明确记录“框架示例/未使用/不迁移”理由。
- 每个旧业务 Controller 必须映射到管理能力、读者契约或第三方回调。
- 每个业务 SQL 表必须进入 `docs/migration/mapping.md`，不能只靠页面矩阵推断。
- M7 只有在所有“待补”替换为可定位证据且不存在未解释旧入口时才可通过。
