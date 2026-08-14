# Moonbook 重构进度

更新时间：2026-08-14

## 总览

| 里程碑 | 状态 | 当前证据 |
| --- | --- | --- |
| M0 冻结、基线与盘点 | 已完成 | GVA 与旧仓库基线已锁定；reader-ui 原样迁入并通过 536 个基线测试；功能、API、数据和容量盘点范围已建立 |
| M1 工程与本地基础设施 | 已完成 | 完整 Compose 应用栈、空库迁移、CI、一键验收、健康/指标/任务/迁移骨架及秘密扫描均有真实运行证据 |
| M2 小说核心与对象存储 | 进行中 | 分类、作者、书籍和章节纵向切片及 PostgreSQL+MinIO 版本化对象服务已实现；真实 HTTP、双库迁移、正文/封面迁移、对象引用切换/完整性/持久化回收及 Long ID 边界验证通过，正在执行 M2 整体退出审计 |
| M3 读者域与零修改兼容 | 未开始 | - |
| M4 交易、支付与运营 | 未开始 | - |
| M5 内容生产与长任务 | 未开始 | - |
| M6 全量迁移与校验 | 未开始 | - |
| M7 生产切换就绪验收 | 未开始 | - |

## M0 已完成

- Gin-Vue-Admin 固定基线：`02f37833255e0e339c3d69199cb5a468f17de9fc`，见 `docs/baselines/gin-vue-admin.md`。
- 旧 Moonbook 冻结基线：`8e7f57316638d199d7a8d7c964a52a89e13fa281`，见 `docs/baselines/legacy-moonbook.md`。
- `reader-ui` 从冻结 commit 原样导入，138 个文件逐文件比较零差异。
- reader-ui 基线测试：44 个测试文件、536 个用例全部通过。
- 读者 API 首版契约清单：`docs/contracts/reader-api.md`。
- 管理功能矩阵：`docs/inventory/admin-feature-matrix.md`。
- 旧数据域映射总表：`docs/migration/mapping.md`。
- 只读 MySQL 容量盘点脚本：`scripts/inventory-legacy-mysql.sh`。

## 当前已知问题

- GVA BSL 1.1 规定 Production Use 需要商业许可证；生产切换前必须取得并保存授权证据。
- 冻结 reader-ui 的 npm 依赖存在 9 个 high 漏洞，必须在 M7 前升级或形成批准的缓解记录。
- 旧仓库存在约 8 GB 数据库备份，但新仓库未复制该敏感/大体积文件；完整数据副本演练将在 M6 使用受控来源执行。
- 当前有效第三方集成的生产启用状态不能仅凭仓库默认配置确定，需要后续脱敏环境清单或用户确认。

## M1 已验证单元

- `compose.yaml` 使用独立 `moonbook` 项目、网络和命名卷启动 PostgreSQL 17.6、Redis 7.4.5 与 MinIO 固定版本。
- `.env.example`、`server/config.moonbook.yaml.tpl` 和 `scripts/render-local-config.sh` 提供无生产秘密的本地配置生成流程；生成文件权限为 `0600` 且无未解析变量。
- `scripts/verify-infrastructure.sh` 已用容器内客户端证明 PostgreSQL 接受连接、Redis 密码认证成功、MinIO readiness 正常，并独立读取策略确认 `moonbook-content` Bucket 为私有。
- `server/config/config_contract_test.go` 对 Moonbook 模板执行确定性渲染和 YAML 严格字段校验；`CGO_ENABLED=0 GOCACHE=/private/tmp/moonbook-go-cache go test ./config ./core/... ./initialize/...` 通过。
- `server/internal/platform/migrate` 使用嵌入式 Goose 和 PostgreSQL 会话锁管理前向迁移，`scripts/migrate-local.sh` 提供本地 `status/up` 入口；真实 PostgreSQL 已验证 `current=0 target=1 pending=true`、首次 `applied=1`、完成后 `pending=false`、重复执行 `applied=0`。
- 首个迁移建立 `platform_jobs`、`platform_job_attempts`、`migration_checkpoints` 和 `migration_errors`，为持久化任务、租约重试和旧数据迁移检查点提供 PostgreSQL 事实表；元数据核对确认 4 张表和 43 个约束存在。
- `server/internal/modules` 建立小说、读者、交易和内容生产模块边界，依赖扫描测试禁止业务模块直接依赖 GVA 全局实现或绕过其他模块公开合同。
- `/health/live` 与 `/health/ready` 已完成运行时验证：live 返回 200，ready 在 11ms 内确认 migrations、MinIO、PostgreSQL、Redis 均为 `ok`；探测器具备 2 秒硬超时，失败响应只暴露依赖状态。两次 SIGINT 验证均记录优雅关闭完成。
- 严格 CORS 已启用并修正无 `Origin` 的反向代理/健康请求语义；运行时确认非浏览器健康请求通过、非白名单 Origin 返回 403。
- GVA PostgreSQL 基座通过前向迁移 `00002`/`00003` 纳入 44 张表、38 个序列和必要框架数据；两次生成 SHA-256 稳定，迁移后的规范化 schema 与 GVA 初始化源库逐字节一致。
- 13 张框架种子表逐表完整行 JSON 哈希与源库一致，仅明确排除已禁用的 `/init/initdb` 免鉴权元数据；用户、用户角色关联及上传示例表为空。全新真实 PostgreSQL 验证首次 `applied=3`，最终 `current=3 target=3 pending=false`，重跑 `applied=0`。
- `moonbook-admin bootstrap` 以事务、advisory lock 和环境变量显式创建首个管理员，不提交默认密码或摘要；真实数据库验证 bcrypt 摘要、3 个角色关联及强制首次改密正确，重复执行拒绝覆盖。临时 API 在完整迁移库启动无缺表错误，真实管理登录返回 `code=0` 和 `needChangePassword=true`。
- `server/internal/platform/apperror` 建立稳定错误分类、未知错误脱敏和 GVA 管理响应适配；管理失败保留 HTTP 200/`code=7`，同时返回 `errorCode`、`requestId` 和 `traceId`。读者兼容层明确延后到 M3 按冻结契约独立映射。
- 受 Bearer Token 保护的 `/metrics` 暴露 Go/进程、HTTP 请求量/延迟/在途请求以及持久化任务状态和过期租约指标；真实运行验证未授权为 401、授权抓取成功、任务三种状态和 1 条过期租约正确，HTTP 标签使用低基数路由模板。
- `server/internal/platform/jobs` 完成幂等入队、module/type 定向并发领取、租约续期、旧 worker 防护、有限重试与指数退避；启动和配置重载会事务性恢复过期租约并同步关闭 attempt 审计。真实 PostgreSQL + race 测试覆盖并发 worker、重复入队、可重试恢复和尝试耗尽终态。
- `moonbook-legacy-migrate preflight` 建立旧 MySQL 迁移 Runner、stage/batch 合同、检查点和错误清单写入；任何 stage 前强制验证源 MySQL 全局只读模式，已完成 stage 幂等跳过。具体业务转换和同事务目标写入按 M2-M6 逐域补齐。
- 完整 Compose 应用栈包含 Go API、GVA 管理前端、冻结版 Reader SSR、Nginx 双入口网关、PostgreSQL、Redis、MinIO，以及一次性迁移和管理员引导任务；API 无宿主端口且以 `uid=100(moonbook)` 非 root 用户运行，管理端 `/api/` 和读者端 `/dev-api/` 路径均通过真实 HTTP 验证。
- 后端镜像在启动时使用 `moonbook-config` 将环境变量经 YAML 编码写入 `0600` 配置，并用标准 URL 编码生成 PostgreSQL DSN；特殊字符、缺失变量和文件权限已有自动化测试。管理前端使用 Node 22、固定 pnpm 10.15.1 和提交的锁文件构建。
- `make verify-m1` 已从空命名卷完整通过九阶段：三项基础设施健康、迁移 current=target 且重跑 `applied=0`、网关双入口、readiness 四依赖、指标 401/200、首管理员引导与真实登录、强制改密、SIGTERM 优雅关闭及恢复、Gitleaks 零新增秘密；测试项目、网络和卷在结束时自动清理。
- `.github/workflows/ci.yml` 建立 Moonbook 基座质量门和真实 Docker Compose M1 门；Go 门覆盖 Moonbook 模块/平台/命令及关联配置、核心、初始化和中间件，管理端按 pnpm 锁文件构建，冻结 Reader 执行 536 个测试并构建，仓库由固定 Gitleaks 8.28.0 扫描。
- Gitleaks 对固定基线的 6 个已审计误报使用精确 `文件:规则:行号` fingerprint 放行：过期 GVA README 媒体 JWT、Reader 合成密码夹具、GVA 公共对象 Key；任意新增或行漂移会重新失败。
- 本机 Go 1.25.12 在 macOS ARM 启用 cgo 时会在上游 `github.com/shoenig/go-m1cpu v0.1.6` 初始化期间崩溃；当前以禁用 cgo 的可重复测试路径规避，不归因于 Moonbook 配置改动。
- 固定 GVA 的仓库级 `go test ./...` 仍包含不适合作为 Moonbook CI 门的上游基线测试：MCP 客户端依赖本机 8888 服务、自动代码测试依赖相对模板/全局数据库、部分插件渲染断言，以及受限沙箱中的 IPv6 `httptest` 监听。CI 不宣称这些基线用例通过；Moonbook 新增及 M1 关联包全部纳入明确测试范围，后续触及对应上游模块时必须逐项收敛。

## M1 退出结论

M1 已满足退出条件：空环境可由一条命令构建并启动，PostgreSQL 从零迁移，真实 PostgreSQL/Redis/MinIO 及完整应用栈通过集成验收，仓库秘密扫描为零新增命中。默认本地 PostgreSQL、Redis 和 MinIO 保持运行，验证期间的应用栈和测试卷均已清理。

## 当前工作

M2 第一条“分类、二级分类、作者”纵向切片已完成：

- 前向迁移 `00004_novel_metadata.sql` 建立 `novel_categories`、`novel_authors`、约束、索引、GVA 菜单、API 元数据和超级管理员 Casbin 策略；独立空库首次 `applied=4`、重复执行 `applied=0`。
- 管理 API 挂载 GVA 私有路由，写操作带操作审计；分类和作者 ID 在 JSON、路径参数及管理前端全程按字符串处理，Go/Node 边界测试覆盖 `9223372036854775807` 和 `9007199254740993`。
- 隔离应用实例已完成真实 GVA 验证码登录、强制首次改密、重新登录、未授权拒绝及 Casbin 下分类/作者 CRUD；readiness 的 PostgreSQL、Redis、MinIO 和 migrations 均为 `ok`，所有业务响应 ID 均经 `jq type` 验证为字符串。
- 管理端新增分类和作者工作页，支持分页、搜索、一级/二级类型、排序、启停/状态、作品方向以及 CRUD；ESLint、Node 测试和生产构建通过。
- `moonbook-legacy-migrate novel-metadata` 以只读 `utf8mb4` MySQL 为硬门槛，分批迁移现行字典、历史分类、现行书籍作者、`book_author` 与 `author`，并在同事务维护 checkpoint 和错误清单。
- 隔离 MySQL 8.4 → PostgreSQL 17 真实迁移测试覆盖两条显式坏数据、最大 bigint、当前书籍作者优先、不同旧 ID 不按同名合并、幂等重跑和 identity 序列推进，测试通过。
- 默认本地开发 PostgreSQL 被只读检查发现迁移记录仅到版本 1、但已有 12 张额外 GVA 表；未改写历史或删除数据。本切片使用独立 `moonbook_m2_verify` 空库完成验证，后续默认开发栈应通过重建项目专用开发卷恢复一致状态，不能在不明来源库上手工补版本。

当前继续 M2 书籍、章节与对象存储闭环。

M2 版本化对象服务基础单元已完成：

- 前向迁移 `00005` 建立对象注册、当前引用和不可变事件表，`00006` 以复合外键保证引用的 kind/book/owner 与对象目标一致；隔离空库首次 `applied=6`，重跑 `applied=0`。
- 小说模块对象服务按批准设计生成 `chapters/{bookId}/{chapterId}/v{version}.txt` 和 `covers/{bookId}/v{version}.{ext}`，使用目标级 advisory lock 分配版本和串行化并发激活。
- 上传时先计算 SHA-256，再写入 MinIO；只有 `StatObject` 返回的对象键、字节数和 SHA-256 全部一致才标为 `verified`，此时仍不建立业务引用。
- 激活在同一 PostgreSQL 事务内切换当前引用并将旧版本标为 `orphaned`；真实集成测试证明第二版提交前仍读取第一版，并发激活后严格只有一个活动版本。
- 回收器只认领超过宽限期且无引用的上传中、失败、孤立或中断删除对象，以 `deleting` 状态和事件日志支持崩溃恢复；MinIO 删除失败恢复为可重试 `failed`，成功后收敛为 `deleted`。
- 真实 PostgreSQL 17 + MinIO 测试覆盖版本上传、哈希/大小读取校验、错误哈希故障注入、零引用保证、旧对象延迟回收、活动对象保护、事件审计和测试 Bucket 清理。

## 下一步

M2 第二条“书籍、标签、发布状态与封面”纵向切片已完成：

- 前向迁移 `00007` 建立 `novel_books`、副分类和标签关系以及 GVA 菜单/API/Casbin；`00008` 纳入鉴权封面上传/读取 API；`00009` 增加旧来源 SHA-256 指纹及幂等唯一约束。隔离 PostgreSQL 从版本 8 正向执行到 9，重跑 `applied=0`。
- 管理 API 完成书籍 CRUD、详情、分页筛选、发布/作品/来源/收费状态、分类、作者、标签和软删除；列表以三次批量查询装载副分类和标签，不产生逐书 N+1。管理端完整加载分类、远程检索作者、字符串处理所有 bigint ID/整书价格，并支持鉴权封面预览和独立上传。
- 封面 API 服务端以魔数接受 JPEG/PNG/WebP/GIF，限制 10 MiB，经 MinIO 大小/SHA-256 校验后才激活引用；读取再次校验字节数和哈希。真实 HTTP E2E 覆盖未授权、首次强制改密、分类/作者/书籍创建、详情、更新、筛选、非法分页、删除、双版本封面逐字节读取、Casbin 和操作审计，全部通过。
- `moonbook-legacy-migrate novel-books` 分批迁移当前 `novel_book` 全字段、整书 `reader_product` 定价和副分类关系，保留旧 bigint ID，支持 checkpoint、错误清单和幂等重跑。真实 MySQL 8.4 -> PostgreSQL 17 测试覆盖最大 bigint、坏分类/状态/关系、缺书和序列推进。
- 旧封面迁移使用默认拒绝私网的安全 HTTP 下载器，限制协议、端口、重定向、超时、大小和魔数；显式主机白名单仅用于受控内网。URL 以 SHA-256 指纹实现幂等，不在对象元数据或错误信息中暴露完整 URL。真实 PostgreSQL + MinIO + 本地 HTTP fixture 验证有效封面激活、错误内容清单及重复运行对象数不增长。
- 小说模块通过平台管理响应适配器保持 GVA 成功响应合同，不再直接依赖 GVA `model` 实现；模块依赖规则、响应合同、Go race/vet、管理端 ESLint、`pnpm test:moonbook` 和生产构建通过。构建仍只有固定上游 `arcdash` BigInt 目标警告。
- 固定 Gitleaks 8.28.0 扫描约 27.88 MB，结果 `no leaks found`；Reader 从冻结 commit 重新归档比较，138 个源文件零差异。本轮隔离 API、PostgreSQL/MySQL 容器、测试卷/网络及临时秘密均已清理，默认 Moonbook PostgreSQL、Redis、MinIO 保持健康运行。

M2 第三条“章节、正文迁移与持久化对象回收”纵向切片已完成：

- 前向迁移 `00010` 建立 `novel_chapters`、目录索引以及 GVA 菜单、6 个 API 元数据和超级管理员 Casbin 策略；真实隔离 PostgreSQL 首次执行到版本 10，重跑 `applied=0`。该迁移已经执行，后续修复只能新增更高版本。
- 管理 API 和页面完成章节分页/书籍/关键词/状态筛选、详情、正文读取、创建、编辑和软删除。正文必须是 UTF-8 且小于 16 MiB，字数按 Unicode code point 排除空白统计；章节变更同步维护书籍总字数和末章信息，禁止直接跨书移动。
- 章节正文使用 `chapters/{bookId}/{chapterId}/v{version}.txt`。MinIO 大小/SHA-256 校验、章节元数据写入、活动引用切换和书籍统计更新形成原子闭环；软删除同时解除引用，所有正文版本进入宽限期 orphan 状态。
- 对象服务新增带业务事务的激活/停用能力。激活事务失败时已验证对象安全转为无引用 orphan，版本号保持不可变；持久化 `object_gc` worker 使用平台 Job 的租约、续租、有限重试和小时幂等键，每批最多 500 个对象，默认宽限期 24 小时，并接入启动、配置热重载和优雅停止。
- `moonbook-legacy-migrate novel-chapters` 按章节 ID 联表迁移当前 `novel_chapter` 与 `novel_chapter_content`，保留 bigint ID/价格，正文校验后原子写元数据和引用。来源指纹保证 checkpoint 丢失式重跑不增加对象；目标非 legacy 同 ID 记录不会被覆盖。
- 真实 HTTP E2E 覆盖 readiness、未授权 401、首次强制改密、重新登录、章节创建/两版正文读取/更新/复合筛选/非法分页/软删除、最大 bigint 价格、JavaScript ID 字符串、Casbin 和操作审计。删除后书籍统计清零、两版正文均 orphan、活动引用为零。
- 真实 MySQL 8.4 -> PostgreSQL 17 + MinIO 测试覆盖两条成功章节、2 条字数重算、坏状态、缺书、缺正文、最大 bigint、幂等重跑、对象不重复和 identity 序列推进。源库在验证前后均为 `read_only=1`、`super_read_only=1`。
- 真实 PostgreSQL 17 + MinIO 测试覆盖章节生命周期、对象激活事务回滚、不可变版本、读后哈希、并发激活、软删除停用、宽限期回收、Job 成功结果和同小时幂等。管理端 ESLint、Long ID 测试和生产构建通过；Go race/vet 和受影响包回归通过。
- Reader 从冻结 commit 重新归档，排除本地 `node_modules`/`build` 后 138 个 Git 跟踪源文件零差异。临时 API 已优雅停止；章节隔离 PostgreSQL/MySQL 容器待提交完成后清理，默认 Moonbook PostgreSQL、Redis、MinIO 保持运行。

下一步：

1. 执行 M2 小说核心整体回归和里程碑退出审计，核对所有交付物及矩阵缺口。
2. 若 M2 退出条件全部满足，将里程碑标记完成并提交审计证据。
3. 接入 M3 读者分类、书目和章节兼容接口。
