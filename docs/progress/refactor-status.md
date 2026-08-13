# Moonbook 重构进度

更新时间：2026-08-14

## 总览

| 里程碑 | 状态 | 当前证据 |
| --- | --- | --- |
| M0 冻结、基线与盘点 | 已完成 | GVA 与旧仓库基线已锁定；reader-ui 原样迁入并通过 536 个基线测试；功能、API、数据和容量盘点范围已建立 |
| M1 工程与本地基础设施 | 已完成 | 完整 Compose 应用栈、空库迁移、CI、一键验收、健康/指标/任务/迁移骨架及秘密扫描均有真实运行证据 |
| M2 小说核心与对象存储 | 未开始 | - |
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

进入 M2 小说核心与对象存储，先从旧系统事实和功能矩阵提取分类、作者、书籍、章节、标签和发布状态的模型与行为，再纵向完成 PostgreSQL 模型、MinIO 对象服务、管理 API/页面、迁移映射和测试。

## 下一步

1. 审计旧小说域实体、管理接口、管理页面和 Reader 调用语义，确定 M2 第一条分类/作者纵向切片。
2. 新增 M2 PostgreSQL 迁移和旧表转换映射，保持旧 `bigint` ID 并补齐 JavaScript 字符串边界测试。
3. 建立 MinIO 版本化对象服务，再完成书籍/章节正文闭环及孤立对象回收。
