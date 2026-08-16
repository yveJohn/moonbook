# 旧管理后台功能矩阵

## 基线与使用方式

本矩阵基于旧仓库 commit `8e7f57316638d199d7a8d7c964a52a89e13fa281` 的管理页面、API、Controller、Service、测试和 SQL。它是完整迁移的追踪入口；每个功能在对应里程碑完成时必须补充新代码、管理路由、权限、迁移映射和测试证据。

状态：`基线已盘点` 表示旧能力已确认但新系统尚未实现。

## GVA 系统基座（M1）

| 功能 | 旧来源 | 新系统验收 | 状态 |
| --- | --- | --- | --- |
| 管理员登录、JWT、验证码、登出 | RuoYi 系统模块 | 使用 GVA 认证并完成安全基线 | GVA 认证、首管理员、强制改密、真实登录和严格 CORS 已验证；最终 RBAC 越权、登录限流、会话与浏览器回归待 M7 固定 commit 验收 |
| 管理员、角色、菜单、API 权限 | system user/role/menu | GVA 用户、角色、菜单、Casbin；不迁移旧管理员数据 | GVA 源码、页面、种子与 Casbin 已导入；旧框架数据按边界不迁移，角色授权与数据权限最终验收待补 |
| 部门、岗位、字典、参数、通知 | system 页面与 Controller | 按实际管理需要保留 GVA 等价能力 | 部门、岗位、字典和参数实现存在但缺统一真实验收；旧通知公告无 GVA 等价模块，也未找到 Moonbook 业务使用证据，需按实际数据确认实现或移除 |
| 操作日志、登录日志、在线状态 | monitor 页面 | 结构化审计、检索、脱敏 | 登录/操作/文件日志实现存在且业务写操作有抽样审计证据；旧在线管理员列表和强退入口无等价实现，需实现或经实际使用确认移除 |
| 文件与对象存储配置 | system/oss | 固定 MinIO 业务存储和受控管理能力 | 业务对象已统一 MinIO 并有哈希/大小/回收证据；旧 OSS 配置不迁移，通用对象运维入口待确认；GVA 通用配置 API 当前会回显 MinIO 等 Secret，生产前必须关闭或脱敏 |
| 系统健康、配置和任务监控 | monitor/admin/cache/snailjob | 健康、指标、持久化任务监控 | health、受保护 metrics、GVA 定时任务和分域任务页已有实现；通用配置 API 违反 Secret 注入边界，且 `platform_jobs` 无统一只读管理入口，详见 `docs/verification/m7-gva-foundation-audit.md` |

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
| 书架、历史、偏好、点赞 | 读者 API 与相关表 | 查看读者行为、兼容读者端 | M3 | 基线已盘点 |
| 反馈处理 | `reader/feedback` / `ReaderFeedbackAdminController` | 列表、详情、回复 | M3 | 列表、详情、单次回复已实现 |
| 运营概览 | `/dashboard/overview` / `DashboardOverviewService` | 用户总量、周期新增、日活、去重周期活跃、30 天趋势 | M4 | `00055`、`GET /dashboard/overview` 和 GVA Dashboard 已实现；按 Kuala Lumpur 自然日采集，30 天趋势零填充，真实 PostgreSQL 与双库迁移测试通过 |
| 邀请码 | `reader/inviteCode` / `ReaderInviteCodeController` | 生成、编辑、删除、状态 | M4 | `00057`、`/reader/inviteCodes` 和管理页面已实现生成、编辑、删除、启停；人工码可编辑邀请码、最大次数、过期时间和备注，读者自动码仅允许编辑状态与备注且禁止删除，真实 PostgreSQL 与浏览器验收通过 |
| 签到奖励规则 | `reader/checkinReward` / `ReaderCheckinRewardRuleController` | CRUD、启停 | M4 | `00025`、`/reader/checkinRules`、管理页面已实现；fixed/random 校验及启用唯一约束有集成测试 |
| 钱包与流水 | `reader/wallet` / `ReaderWalletAdminController` | 余额、流水、调整、签到、邀请奖励 | M4 | 余额、不可变流水和人工调账已实现；请求 ID 幂等、钱包锁和余额不足保护有真实 PostgreSQL 集成测试；签到和邀请奖励已有配置/流水事实，但旧 API 定义的两个只读记录查询尚未实现为可见审计页签 |
| 消费商品 | `reader/product` / `ReaderProductController` | 书籍、章节、会员、免广告商品及上下架 | M4 | 管理端分页筛选、CRUD、在售状态、目标存在性校验和历史订单删除保护已实现；真实 PostgreSQL 集成测试通过 |
| 消费订单 | `reader/order` / `ReaderOrderAdminController` | 查询、人工充值、确认 | M4 | 管理端列表/详情和筛选已实现；模拟充值按“创建 pending 订单、单独确认入账”两阶段执行，请求/确认流水幂等且操作受审计；退款仍需独立财务审批设计 |
| 会员人工发放 | `reader/user` / `ReaderMembershipGrantController` | 限时或永久会员发放、幂等请求、备注审计 | M4 | `POST /reader/users/:id/membership` 已实现；事务锁定读者账号，会员授予独立叠加且不修改购买订单 |
| 充值产品与设置 | `reader/payment/product` / `ReaderRechargeProductAdminController` | 预设档位、自定义兑换范围 | M4 | 充值档位 CRUD、自定义充值开关/汇率/最小最大范围管理已实现；真实 PostgreSQL 集成测试覆盖非法规则 |
| 支付渠道 | `reader/payment/channel` / `ReaderPaymentChannelAdminController` | 配置、启停、连通性检查、凭据保护 | M4 | 渠道查询、启停、凭据状态和脱敏连通性检查已实现；真实生产网关连通性仍需切换前使用受控凭据验收 |
| 充值订单与回调 | `reader/payment/order` / `ReaderRechargeOrderAdminController` | 创建、详情、回调日志、主动同步、异常状态 | M4 | 列表、筛选、详情、EPUSDT 同步创建、失败分类、活动订单替换、过期释放、历史凭据回调、防重放、人工补单和主动同步已实现；16 路并发和隔离 HTTP 替身冒烟通过。回调全尝试审计、真实最小金额支付和财务全域核对仍待完成，详见 `docs/verification/m4-exit-audit.md` |

## 内容生产（M5）

| 功能 | 旧页面/Controller | 关键操作 | 新系统证据 | 状态 |
| --- | --- | --- | --- | --- |
| 论坛来源 | `crawl/forumSource` / `CrawlForumSourceController` | CRUD、启停、连接与规则配置 | M5 | 来源 CRUD、URL/节流校验、Cookie 脱敏、权限和真实 PostgreSQL 测试已实现；连接检查待补 |
| 论坛板块 | `crawl/forumBoard` / `CrawlForumBoardController` | CRUD、增量游标、抓取策略 | M5 | 板块 CRUD、来源关联、同来源名称唯一、URL 域名和跟进策略校验已实现；导入任务引用保护待任务模块补齐 |
| 线程候选 | `crawl/threadCandidate` / `CrawlThreadCandidateController` | 筛选、状态、加入导入 | M5 | 候选列表/详情、来源板块筛选、跳过/恢复/删除状态管理已实现；新增受控板块发现 API，按来源与外部帖子 ID 幂等写入候选 |
| 导入任务 | `crawl/importTask` / `CrawlImportTaskController` | 创建、执行、暂停/恢复、重试、日志 | M5 | 导入任务创建、队列入库、同源多页抓取、状态/质量字段、取消、有限重试和人工恢复已实现；真实 PostgreSQL + MinIO 集成测试覆盖成功及重试耗尽终态 |
| 运行任务 | `CrawlRuntimeTaskController` | 调度、租约、恢复、取消 | `platform_jobs`、各内容 Worker、自动发现调度 | 论坛导入及 AI/TXT Worker 已有租约恢复和人工停止/重试证据；旧 `forumDiscover` 持续自动发现/自动跟进没有等价调度器，`autoFollowEnabled` 当前只保存配置，仍待补齐或经用户确认移除 |
| 抓取日志 | `crawl/fetchLog` / `CrawlFetchLogController` | 检索、错误详情、重试线索 | M5 | 抓取日志表、阶段/结果约束、任务/状态筛选和详情管理页面已实现；Worker 逐页记录 URL、状态码、字节数、耗时与失败线索，后页失败保留已成功页面证据 |
| TXT 导入 | `novel/txtImport` / `TxtImportController` | 上传、解析、预览、导入、失败文件修复 | `novel_txt_import_task`、`/novel/txtImports`、`view/novel/txtImports` | 上传、MinIO 校验、持久化队列、章节导入、受限预览和失败任务文件替换已实现 |
| 书籍合并 | `novel/bookMerge` / `NovelBookMergeController` | 候选、章节映射、执行、审计 | `novel_book_merge_*`、`/novel/bookMerges`、`view/novel/bookMerges` | 论坛导入来源筛选、时间排序、重复标记、人工排除、MinIO 正文复制、事务执行、源书下架和血缘审计已实现（迁移至 `00047`）；active AI 清洗结果优先策略待 AI 切片接入 |
| AI 配置与模型 | `novel/aiConfig` / `NovelAiConfigController` | OpenAI 兼容地址、有序模型、启停、失败切换、秘密保护 | `novel_ai_config*`、`/novel/aiConfigs`、`view/novel/aiConfigs` | 配置 CRUD、启用选项、流式模式、有序模型、连续失败循环切换、状态版本和人工重置已实现（迁移 `00048`）；API Key 改为环境变量 Secret 引用，连通性随 AI 执行器切片验证 |
| 章节清洗 | `novel/chapterClean` / `NovelChapterCleanController` | 配置、任务、结果、采用、失败重试 | `novel_chapter_clean_*`、`/novel/chapterClean`、`view/novel/chapterClean` | 配置、OpenAI 兼容流式/非流式传输、持久化任务、模型失败切换、自动采用、人工采用/丢弃、停止/续跑和失败重洗已实现（迁移 `00049`）；`00051` 前向修复清洗稿对象类型约束 |
| 章节摘要 | `NovelChapterSummaryController` | 配置、批量回填、失败重试 | `novel_chapter_summary_*`、`/novel/chapterSummary`、`view/novel/chapterClean` 简介补全页签 | 单例配置、状态/任务分页及详情、手动与自动批处理、停止/续跑、租约恢复、MinIO 清洗稿读取、JSON/SSE、拒答备用 AI、模型轮换和 7 天诊断清理已实现（迁移 `00050`、`00052`） |
| 书籍画像 | `novel/bookProfile` / `NovelBookProfileController` | 配置、建议、采用、自动应用、重试 | `novel_book_profile_*`、`/novel/bookProfile`、`view/novel/bookProfile` | 单例配置、正文/简介输入降级、持久化 Worker、JSON/SSE、拒答备用 AI、人工审核、自动应用、批量重试和陈旧快照保护已实现（迁移 `00053`） |

## 当前有效集成候选

| 集成 | 代码证据 | M0 结论 |
| --- | --- | --- |
| EPUSDT | 充值、回调 Controller 及 Flyway 表 | 当前业务实现存在；生产是否启用待脱敏环境确认 |
| OpenAI 兼容 AI | AI 配置、模型、清洗、摘要、画像模块 | 当前业务实现存在；具体供应商与模型由新配置迁移 |
| SMTP 邮件 | RuoYi mail 配置和验证码能力 | 框架能力存在；Moonbook 生产是否启用待确认 |
| HTTP 代理 | 采集和 AI 外部请求配置 | 当前业务实现存在；只迁移实际启用配置 |
| 对象存储 | RuoYi OSS 与历史文件字段 | 新系统统一为 MinIO，不迁移旧框架 OSS 配置 |
| Redis | 登录、缓存、任务协调 | 新系统继续使用，但不迁移缓存数据 |

## 完整性检查规则

- 每个旧业务页面必须映射到本矩阵或明确记录“框架示例/未使用/不迁移”理由。
- 每个旧业务 Controller 必须映射到管理能力、读者契约或第三方回调。
- 每个业务 SQL 表必须进入 `docs/migration/mapping.md`，不能只靠页面矩阵推断。
- M7 只有在所有“待补”替换为可定位证据且不存在未解释旧入口时才可通过。
