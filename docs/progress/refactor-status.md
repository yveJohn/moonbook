# Moonbook 重构进度

更新时间：2026-08-14

## 总览

| 里程碑 | 状态 | 当前证据 |
| --- | --- | --- |
| M0 冻结、基线与盘点 | 已完成 | GVA 与旧仓库基线已锁定；reader-ui 原样迁入并通过 536 个基线测试；功能、API、数据和容量盘点范围已建立 |
| M1 工程与本地基础设施 | 已完成 | 完整 Compose 应用栈、空库迁移、CI、一键验收、健康/指标/任务/迁移骨架及秘密扫描均有真实运行证据 |
| M2 小说核心与对象存储 | 已完成 | 分类、作者、书籍、章节、读者 SEO 管理、PostgreSQL+MinIO 版本化对象服务及旧库迁移均已实现；整体回归、空库迁移、HTTP E2E、对象完整性、Long ID、Reader 零差异和秘密扫描通过 |
| M3 读者域与零修改兼容 | 实施中 | Reader/Commerce 实现、迁移 stage 和隔离 PostgreSQL/Redis/MinIO 真实集成测试已完成；冻结接口契约快照、SSR/浏览器旅程和 8GB 副本演练仍待完成 |
| M4 交易、支付与运营 | 实施中 | 钱包、充值、签到、购买、支付回调、财务核对、充值档位/自定义充值设置/支付渠道/渠道连通性检查/签到规则/钱包人工调账/充值订单人工补单/主动同步/邀请奖励/会员发放已实现；迁移演练和综合退出审计仍待完成 |
| M5 内容生产与长任务 | 实施中 | 论坛来源、板块、候选、导入任务和抓取日志已完成，迁移版本 `00041`；Worker 长循环已接入，TXT、AI、分页抓取和全量恢复演练仍待完成 |
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

## M2 已完成

M2 第一条“分类、二级分类、作者”纵向切片已完成：

- 前向迁移 `00004_novel_metadata.sql` 建立 `novel_categories`、`novel_authors`、约束、索引、GVA 菜单、API 元数据和超级管理员 Casbin 策略；独立空库首次 `applied=4`、重复执行 `applied=0`。
- 管理 API 挂载 GVA 私有路由，写操作带操作审计；分类和作者 ID 在 JSON、路径参数及管理前端全程按字符串处理，Go/Node 边界测试覆盖 `9223372036854775807` 和 `9007199254740993`。
- 隔离应用实例已完成真实 GVA 验证码登录、强制首次改密、重新登录、未授权拒绝及 Casbin 下分类/作者 CRUD；readiness 的 PostgreSQL、Redis、MinIO 和 migrations 均为 `ok`，所有业务响应 ID 均经 `jq type` 验证为字符串。
- 管理端新增分类和作者工作页，支持分页、搜索、一级/二级类型、排序、启停/状态、作品方向以及 CRUD；ESLint、Node 测试和生产构建通过。
- `moonbook-legacy-migrate novel-metadata` 以只读 `utf8mb4` MySQL 为硬门槛，分批迁移现行字典、历史分类、现行书籍作者、`book_author` 与 `author`，并在同事务维护 checkpoint 和错误清单。
- 隔离 MySQL 8.4 → PostgreSQL 17 真实迁移测试覆盖两条显式坏数据、最大 bigint、当前书籍作者优先、不同旧 ID 不按同名合并、幂等重跑和 identity 序列推进，测试通过。
- 默认本地开发 PostgreSQL 被只读检查发现迁移记录仅到版本 1、但已有 12 张额外 GVA 表；未改写历史或删除数据。本切片使用独立 `moonbook_m2_verify` 空库完成验证，后续默认开发栈应通过重建项目专用开发卷恢复一致状态，不能在不明来源库上手工补版本。

M2 版本化对象服务基础单元已完成：

- 前向迁移 `00005` 建立对象注册、当前引用和不可变事件表，`00006` 以复合外键保证引用的 kind/book/owner 与对象目标一致；隔离空库首次 `applied=6`，重跑 `applied=0`。
- 小说模块对象服务按批准设计生成 `chapters/{bookId}/{chapterId}/v{version}.txt` 和 `covers/{bookId}/v{version}.{ext}`，使用目标级 advisory lock 分配版本和串行化并发激活。
- 上传时先计算 SHA-256，再写入 MinIO；只有 `StatObject` 返回的对象键、字节数和 SHA-256 全部一致才标为 `verified`，此时仍不建立业务引用。
- 激活在同一 PostgreSQL 事务内切换当前引用并将旧版本标为 `orphaned`；真实集成测试证明第二版提交前仍读取第一版，并发激活后严格只有一个活动版本。
- 回收器只认领超过宽限期且无引用的上传中、失败、孤立或中断删除对象，以 `deleting` 状态和事件日志支持崩溃恢复；MinIO 删除失败恢复为可重试 `failed`，成功后收敛为 `deleted`。
- 真实 PostgreSQL 17 + MinIO 测试覆盖版本上传、哈希/大小读取校验、错误哈希故障注入、零引用保证、旧对象延迟回收、活动对象保护、事件审计和测试 Bucket 清理。

## M2 书籍与章节切片

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

M2 第四条“读者 SEO 管理与旧配置迁移”纵向切片已完成：

- 前向迁移 `00011` 建立固定 `id=1` 的 `novel_reader_seo_config` 单例、正式默认值、SEO 管理菜单、GET/PUT API 元数据和超级管理员 Casbin 策略；该迁移已经执行，后续修复只能新增更高版本。
- 独立 `novel/readerseo` 模块完成管理查询与事务更新，统一校验站点根 URL、文本长度、非空值及书库/书籍模板占位符；管理响应 ID 始终为字符串，PUT 不接收 ID 或时间字段，写操作进入现有操作审计。
- 管理端完成基础设置、页面文案、收录控制、模板即时校验和随当前站点名称更新的预览；真实浏览器在桌面及 `390x844` 视口无横向溢出或遮挡，合法 PUT、重载持久化、非法 URL 前端阻止均通过。
- `moonbook-legacy-migrate novel-reader-seo` 完成旧 MySQL 单例迁移；合法配置完整覆盖，无效配置、缺表、额外 ID 分别记录结构化错误并保留目标默认值。真实 MySQL -> PostgreSQL 验证 checkpoint 为 `processed=2,error=1,done=true`，重复执行幂等，源库前后保持 `read_only=1`、`super_read_only=1`。
- HTTP E2E 的 PUT 返回 `code=0` 和字符串 `id="1"`，请求体只含 12 个可编辑字段；PostgreSQL 保持唯一配置行，并存在状态 200、用户 ID 和 request ID 完整的 PUT 操作审计。

## M2 退出结论

- 全新隔离 PostgreSQL 验证 `current=0 target=11 pending=true`，首次执行 `applied=11`，重复执行 `applied=0`，最终 `current=11 target=11 pending=false`；6 个小说菜单、23 个小说 API、23 条超级管理员 Casbin 策略和唯一 SEO 行均正确。
- `CGO_ENABLED=0` 的 M2/关联后端测试和 `go vet` 全部通过。本机 macOS ARM 的 race 仍会在上游 `go-m1cpu v0.1.6` cgo 初始化中崩溃；同一测试集已在官方 Linux Go 1.24.2 容器中以只读源码运行 `go test -race` 并全部通过。
- 管理前端定向 ESLint、`pnpm test:moonbook` 5/5 和生产构建通过；构建仅有固定上游 Vite 配置及 `arcdash` BigInt target 警告。
- Reader 与冻结 commit 的 138 个 Git 跟踪文件 blob 和路径零差异。固定 Gitleaks 8.28.0 扫描 28.11 MB，结果 `no leaks found`。
- 管理功能矩阵中分类、作者、书籍、章节和 SEO 配置均有代码、权限、页面、迁移和测试证据；书籍画像按既定边界属于 M5，不构成 M2 缺口。
- PostgreSQL 不引用未验证 MinIO 对象，正文/封面活动引用切换与业务事务闭环，孤立对象由持久化回收任务处理；旧 ID 与 JavaScript 字符串 ID 边界均有自动化和真实 HTTP 证据。
- 隔离 API、Vite、浏览器、MySQL 容器、验证数据库和临时秘密均已清理；默认 Moonbook PostgreSQL、Redis、MinIO 保持健康运行。

M2 已满足退出条件：小说管理闭环、对象存储完整性、管理页面、权限、审计、旧数据迁移和整体回归均通过。公开 SEO、robots、sitemap 以及读者分类、书目、章节兼容接口按批准边界进入 M3。

## M3 当前进度

- 已确认 M3 方案 A 的模块所有权、Reader-only Token、邀请码原子消费和 Commerce 正式权益表边界。
- 已固化五个认证路由的请求/响应、错误、Token 隔离和字符串 ID 契约：`docs/contracts/reader-auth-api.md`。
- 已固化旧 MySQL 到 PostgreSQL 的字段映射、stage/checkpoint、核对 SQL 和 8GB 副本演练参数：`docs/migration/m3-reader-commerce.md`。
- Reader 认证已实现：BCrypt/历史 32 位 MD5 登录升级、会话摘要撤销、Redis IP+账号限流失败关闭、邀请码原子注册和邀请关系。
- Reader 公开内容已实现：书目精选/分页/随机、分类、书籍详情、章节目录、章节正文、SEO JSON、robots 和 sitemap；公开查询过滤已发布且未软删除数据，正文读取通过 MinIO 活动对象并校验大小/SHA-256/UTF-8。
- Commerce 访问合同已实现批量商品、定价、会员和权益读取；章节目录与正文访问使用统一访问判定和 46101-46105 兼容错误。
- Reader 个人数据已实现：书架、点赞、阅读历史、偏好和反馈；写操作按 reader_id 隔离，点赞在 PostgreSQL 事务内锁书并重算汇总，历史写入校验书籍/章节归属和可见状态。
- 读者管理已补充密码重置：`PUT /reader/users/:id/password` 使用 bcrypt 更新密码并在同一事务撤销所有 Reader session，管理端提供确认密码表单，真实 PostgreSQL 集成测试验证旧会话失效和新哈希可用，迁移版本推进至 `00027`。
- 当前代码证据提交为 `b908041`、`163664b`、`86fd095`、`b983227`、`4af0f25`、`daa3654`、`f5cd1bd`、`4976066`、`ce7de9d`、`97ba7fe`、`b195dae`。
- Reader/Commerce 的真实 PostgreSQL/Redis/MinIO 集成验收已通过（提交 `8a50008`，批量读者隔离修复提交 `725c5ad`）。认证 HTTP 包装契约测试提交 `cfcb2a9` 已验证 Long ID 字符串和未登录错误语义；新增 public HTTP 集成契约测试覆盖匿名书库、Bearer 章节目录和 MinIO 正文。冻结 `reader-ui` 离线测试已通过 44 个测试文件/536 个用例，SSR 生产构建通过；隔离 Go API + Reader SSR 已使用真实书籍/章节 fixture 验证书籍详情、章节目录、匿名正文 46101、Book JSON-LD 和实际内容页面。Playwright CLI 因本机无缓存且网络受限未启动，浏览器关键旅程和 8GB 副本演练仍未完成，不能作为 M3 退出证据。
- 隔离 Compose 浏览器环境已验证注册后 `/me` 所需的 `GET /reader/me/entitlements`、`GET /reader/products/membership`、`POST /reader/me/invite/code` 均经 Reader 网关返回 HTTP 200；邀请码首次生成使用 PostgreSQL 事务和 advisory lock 并保证重复请求复用同一邀请码。Reader Origin 已加入严格 CORS 白名单，注册请求从 `127.0.0.1` 页面返回 200。浏览器视觉旅程和 8GB 副本演练仍待完成。

下一步：

1. 用真实 Reader API 补充有实际书籍/章节数据的书籍详情和章节 SSR 请求，并固定 HTTP 契约快照；当前空库书库 SSR 已通过。
2. 在可用的浏览器运行环境执行冻结 `reader-ui` 登录、公开浏览、书架和阅读关键旅程，逐项记录响应字段、错误和缓存语义。
3. 完成 M3 迁移 stage 的真实 MySQL 副本演练、行数/主键/关联核对和 M3 退出审计，再进入 M4 钱包、订单和支付写流程。

## M4 当前进度

- 前向迁移 `00013_reader_wallet_foundation.sql` 建立 `reader_wallets` 钱包汇总表和 `reader_wallet_ledgers` 不可变流水表；余额、收入/支出累计值均使用 PostgreSQL `bigint`，流水支持业务幂等键和分页查询。
- 钱包流水通过数据库触发器禁止更新和删除；余额变更设计为锁定钱包行、写入前后余额并在同一事务提交，Redis 不承载余额事实。
- 已新增 `commerce/wallet` 仓储合同并接入 Reader HTTP 的 `/reader/me/wallet`、`/reader/me/wallet/ledgers`；余额和金额字段以字符串输出，充值、订单、签到及支付回调将在后续 M4 纵向切片实现。
- 前向迁移 `00014_reader_recharge_foundation.sql` 建立充值产品、充值规则、支付渠道、充值订单和支付回调日志事实表；新增充值目录、精确报价、创建订单和查询订单兼容接口。订单创建要求启用本地/外部支付渠道；回调验签和钱包入账已在支付模块完成，真实网关创建适配仍待接入。
- 前向迁移 `00015_reader_checkin_foundation.sql` 建立签到奖励规则和每日签到记录；签到使用 PostgreSQL advisory lock、幂等键，并在同一事务写入奖励记录和 bonus 钱包流水；已接入 `/reader/me/checkin/status` 与 `/reader/me/checkin`。
- 前向迁移 `00016_reader_purchase_orders.sql` 建立读者购买订单事实和权益 ID 序列；新增会员、章节、整书购买服务，订单、钱包扣款和会员/作品权益在同一事务内提交，并接入 `/reader/me/orders/membership`、`/reader/me/orders/chapter`、`/reader/me/orders/book`。支付充值回调仍待实现。
- 支付模块接入 `/reader/payment/epusdt/notify`，使用环境变量注入 PID/密钥，采用排序规范化字段和 HMAC-SHA256 验签；校验金额、USDT、状态、交易哈希唯一性后，在同一事务写入充值钱包流水、订单状态和回调日志，重复成功回调幂等返回。
- 新增 `moonbook-finance-reconcile` 命令和 `commerce/reconcile`，逐钱包按不可变流水重算余额、收入和支出累计值；输出差异明细并在存在差异时返回非零退出码，作为迁移演练和切换验收硬门。
- 新增充值档位管理 API 和管理页面（`GET/POST/PUT/DELETE /reader/payment/rechargeProducts`），充值档位 ID、钻石数量和价格在管理前端按字符串处理；数据库序列保证并发创建不发生 `max(id)+1` 冲突。
- 新增支付渠道管理 API 和管理页面（`GET/PUT /reader/payment/channels`），支持渠道启停和凭据已配置状态检查；支付 PID/密钥只从环境变量读取，管理响应不返回敏感值。前向迁移版本已推进至 `00018`。
- 新增 `POST /reader/payment/channels/:id/check` 连通性检查；禁用渠道、未配置凭据、无健康地址、网络失败和非支持渠道均返回脱敏状态，EPUSDT 仅访问环境变量提供的健康检查 URL，2 秒超时且不发送支付密钥。真实隔离 PostgreSQL + 本地 HTTP 替身集成测试通过，迁移版本推进至 `00030`。
- 新增充值订单管理 API 和页面（`GET /reader/payment/orders`、`GET /reader/payment/orders/:id`），支持订单号/读者关键词、状态筛选、详情及网关交易信息只读审计；真实 PostgreSQL 集成测试覆盖列表、分页筛选和详情，迁移版本推进至 `00019`。
- 新增支付回调日志管理 API 和页面（`GET /reader/payment/callbackLogs`、`GET /reader/payment/callbackLogs/:id`），支持商户订单号/网关交易号、处理结果筛选、签名有效性、失败原因和响应状态审计；原始 payload 不通过管理接口返回，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00020`。新增 `POST /reader/payment/orders/:id/manualPay` 人工补单，事务锁订单并写入充值钱包流水、人工回调日志和 `paid` 状态；请求 ID 幂等、已支付冲突和交易号复用保护有真实 PostgreSQL 集成测试，迁移版本推进至 `00029`。真实网关主动同步仍需外部连通性与安全替身验收。
- 新增钱包与流水管理 API 和页面（`GET /reader/wallets`、`GET /reader/wallets/:readerId/ledgers`），支持读者关键词、余额汇总和币种流水只读核对；余额、累计值、流水 ID、读者 ID均以字符串输出，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00021`。人工调整、签到/邀请奖励运营入口仍待实现。
- 新增读者用户管理 API 和页面（`GET /reader/users`、`GET /reader/users/:id`、`PUT /reader/users/:id/status`），支持账号/昵称和状态筛选、详情、启停；停用或删除状态会在同一事务撤销现有读者会话，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00022`。密码重置已在 `00027` 补齐，反馈处理和行为运营页面仍待实现。
- 新增读者反馈管理 API 和页面（`GET /reader/feedback`、`GET /reader/feedback/:id`、`PUT /reader/feedback/:id/reply`），支持关键词/状态筛选、详情和单次回复；回复状态转换使用事务锁，重复回复被拒绝并写入操作审计，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00023`。行为运营页面仍待实现。
- 新增邀请码管理 API 和页面（`GET/POST /reader/inviteCodes`、`PUT/DELETE /reader/inviteCodes/:id`），支持自动/自定义生成、使用次数、过期时间、启停和未使用码删除；已使用邀请码禁止删除，ID 生成使用事务 advisory lock，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00024`。
- 新增签到奖励规则管理 API 和页面（`GET/POST/PUT/DELETE /reader/checkinRules`），支持每日/连续签到、固定/随机奖励、启停、排序和备注；所有 ID/金额字段按字符串传输，数据库唯一索引保证同一启用规则不重复，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00025`。
- 新增自定义充值设置管理 API 和页面（`GET/PUT /reader/payment/rechargeSettings`），支持自定义充值启停、钻石/USDT 汇率及最小/最大钻石范围；汇率最多 8 位小数，范围和正数校验在服务端与页面双重执行，真实 PostgreSQL 集成测试通过，迁移版本推进至 `00026`。
- 新增钱包人工调账 API 和管理入口（`POST /reader/wallets/:readerId/adjust`），支持充值币/奖励币收入与支出、原因和请求 ID；请求 ID通过不可变流水幂等键处理，事务锁定读者和钱包，支出不足自动拒绝，真实 PostgreSQL 集成测试覆盖重复请求和余额保护，迁移版本推进至 `00028`。
- 新增消费商品管理 API 和页面（`GET/POST /reader/products`、`PUT/DELETE /reader/products/:id`），覆盖书籍、章节、会员和免广告商品；所有 ID/价格以字符串传输，目标书籍/章节存在性、商品类型约束、历史订单引用删除保护均在服务端校验，迁移版本推进至 `00031`，真实 PostgreSQL 集成测试覆盖 CRUD、筛选和引用删除保护。
- 新增消费订单管理 API 和页面（`GET /reader/orders`、`GET /reader/orders/:id`），提供订单号/读者、订单类型、状态筛选及扣款拆分只读核对；所有 Long ID/金额字符串化，真实 PostgreSQL 集成测试覆盖列表、详情和读者关联，迁移版本推进至 `00032`。退款和人工充值未在只读切片中开放，避免绕过钱包不可变流水。
- 新增会员人工发放 API 和读者用户管理入口（`POST /reader/users/:id/membership`），支持按请求 ID 幂等发放限时或永久会员；事务锁定读者账号，备注写入会员授予事实，重复请求返回原记录，迁移版本推进至 `00033`，真实 PostgreSQL 集成测试覆盖幂等和叠加授予。
- 新增邀请奖励配置管理 API 和页面（`GET/PUT /reader/inviteReward`），支持启停、邀请人/新读者奖励币和备注；注册事务在建立邀请关系后向双方写入不可变 bonus 钱包流水，使用关系级幂等键防重复发奖，迁移版本推进至 `00034`，真实 PostgreSQL 集成测试覆盖配置持久化和注册奖励余额。
- 新增充值订单受控主动同步 API（`POST /reader/payment/orders/:id/sync`）和管理入口；仅允许环境变量配置的 EPUSDT 端点，响应必须通过 HMAC、订单号和状态校验，非终态记录哈希化回调日志，成功状态复用支付入账事务并保持幂等；迁移版本推进至 `00035`，本地 HTTP 替身 + 真实 PostgreSQL 集成测试通过。

## M5 当前进度

- 新增论坛来源管理 API 和页面（`GET/POST/PUT/DELETE /novel/crawl/sources`），迁移 `00036` 同时建立来源/板块事实表和 GVA 菜单权限；来源 URL、响应编码、请求间隔和排序在服务端校验，Cookie 只返回是否配置，编辑留空不会清除既有 Cookie，真实 PostgreSQL 集成测试覆盖 CRUD、筛选和敏感字段脱敏。
- 新增论坛板块管理 API 和页面（`GET/POST/PUT/DELETE /novel/crawl/boards`），迁移 `00037` 注册菜单、API 和超级管理员权限；板块来源关联、来源启停约束、同来源名称唯一、URL 域名、分页模板和自动跟进参数在服务端校验，Long ID 全程字符串化。导入任务引用保护将在任务模块落地后补齐。
- 新增论坛帖子候选管理 API 和页面（`GET /novel/crawl/candidates`、`GET /novel/crawl/candidates/:id`、`PUT /novel/crawl/candidates/skip`、`PUT /novel/crawl/candidates/restore`、`DELETE /novel/crawl/candidates`），迁移 `00038` 建立候选事实表、状态约束、来源/板块引用和唯一外部帖子索引；列表筛选、详情、状态流转和已导入候选删除保护通过真实 PostgreSQL 集成测试。发现扫描和导入执行将在长任务模块接入。
- 新增 `POST /novel/crawl/candidates/discover` 板块候选发现入口和管理页面按钮，受控抓取启用板块（HTTP/HTTPS、来源域名、4 MiB 响应体、最多 10 页），解析 Discuz 常见帖子链接后按 `(source_id,forum_thread_id)` 幂等 upsert；重复发现真实 PostgreSQL 集成测试通过，迁移 `00041` 注册 API 权限。
- 新增论坛导入任务管理 API 和页面（`GET/POST /novel/crawl/importTasks`、`GET /novel/crawl/importTasks/:id`、`POST /novel/crawl/importTasks/:id/retry`、`POST /novel/crawl/importTasks/:id/cancel`），迁移 `00039` 建立任务快照、质量统计、状态/尝试次数约束及 `platform_jobs` 队列关联；创建任务会锁定候选并写入持久化队列，支持取消和失败/取消后重试，真实 PostgreSQL 集成测试覆盖队列入库与状态联动。实际抓取执行器、租约消费和抓取日志页面仍待后续切片。
- 新增抓取日志查询 API 和页面（`GET /novel/crawl/fetchLogs`、`GET /novel/crawl/fetchLogs/:id`），迁移 `00040` 建立请求/解析/导入阶段日志事实表、状态约束和任务索引；导入 worker 核心支持通过 `platform_jobs` claim/lease 执行、成功/失败任务状态联动及日志写入。新增受控 `HTTPExecutor`，限制 HTTP/HTTPS URL、超时、User-Agent、响应体大小和可重试状态码；新增论坛章节标题/正文解析器和 `ChaptersWriter`，复用章节服务的 MinIO 校验上传、对象激活和 PostgreSQL 写入，并在写入前按书籍/章节序号跳过已存在章节以支持幂等重跑。HTTP/解析本地 `httptest` 替身和真实 PostgreSQL + MinIO writer 集成测试已通过，覆盖首次写入、对象激活和重复执行不新增章节；完整 worker 长循环和论坛分页抓取仍待执行器接入测试环境。
- 导入 Worker 已接入应用生命周期：启动时构造受控 HTTP 执行器、MinIO 章节写入器和持久化 Job Repository，持续 claim `forum_import` 任务；执行期间按租约续期，轮询前恢复过期租约，优雅停止和配置重载均会等待 Worker 退出。真实隔离 PostgreSQL 集成测试已验证持久化任务从 pending 到 succeeded、候选同步为 imported、Job 状态联动和执行器重复安全；论坛多页抓取、TXT/AI 任务和全量重启演练仍待完成。
