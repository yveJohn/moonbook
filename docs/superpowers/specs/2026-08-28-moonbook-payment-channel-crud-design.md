# Moonbook 支付渠道安全 CRUD 设计

## 1. 文档状态

- 日期：2026-08-28
- 状态：设计已获用户确认并完成自检，待用户书面审阅
- 适用仓库：`/Users/yve/code/ai-project/moonbook`
- 目标：让管理后台能够安全地新增、查看、修改、归档和检查 EPUSDT 支付渠道，配置保存后立即生效

本规格替代既有“支付凭据只能由 EPUSDT 专属环境变量注入”的运行方式。当前处于开发阶段，没有真实支付订单或需要兼容的历史凭据；实现不得导入旧环境变量中的 EPUSDT 配置，也不建设历史凭据兼容表。首次发布后由管理员在页面创建全新配置，PostgreSQL 成为支付渠道配置的唯一业务事实来源。

## 2. 已确认决策

1. 管理页面支持支付渠道完整 CRUD，而不只是启停和连通性检查。
2. 本期只支持一个未归档的 EPUSDT 渠道，不允许创建未实现协议的提供商。
3. PID、Secret 等秘密允许由管理页面写入，但数据库只能保存认证加密密文，接口永不回显明文。
4. 全项目只使用一个应用数据加密主密钥 `MOONBOOK_APP_MASTER_KEY`；后续功能不得新增模块专属主密钥。
5. 配置按业务操作实时从 PostgreSQL 读取，保存后立即生效，不依赖应用重启或进程内缓存失效。
6. 删除采用安全归档，不物理删除渠道事实。当前不存在历史负担，因此不迁移、不保留旧环境变量凭据。
7. 存在进行中订单时禁止替换 PID/Secret 或归档，防止已创建订单失去验签能力。

## 3. 范围

### 3.1 包含

- 通用应用数据加密服务及唯一主密钥配置合同；
- EPUSDT 渠道数据库字段和前向迁移；
- 管理端列表、详情、新建、编辑、启停、归档、查看归档和连通性检查；
- 创建订单、回调验签、主动同步、健康检查和过期策略改为实时读取数据库配置；
- Casbin API 权限、GVA 操作记录脱敏和稳定业务错误；
- 管理前端表单校验、Secret 写入语义和高风险二次确认；
- 真实 PostgreSQL 集成测试、支付 HTTP 替身测试、前端测试和生产构建验证；
- 项目安全规范、环境模板和部署文档中的统一主密钥说明。

### 3.2 不包含

- 其他支付提供商、链、Token 或多渠道路由；
- 历史凭据表、旧 EPUSDT 环境变量导入或历史订单兼容；
- 在页面展示、复制、导出或下载任何秘密明文；
- 在线主密钥轮换、多主密钥 keyring 或外部 KMS；
- 真实支付、生产数据修改、生产发布或 `git push`，除非用户另行明确要求。

## 4. 统一加密服务

新增 `server/internal/platform/secretcrypto`，业务模块只能依赖其窄接口，不得自行实现 AES、派生模块密钥或读取新的主密钥变量。

### 4.1 主密钥合同

- 环境变量固定为 `MOONBOOK_APP_MASTER_KEY`；
- 值必须是标准 Base64 编码的 32 字节随机值；
- 解码失败、长度错误或存在加密数据但变量缺失时，应用必须拒绝启动支付运行路径并输出不含秘密的稳定配置错误；
- `.env.example` 只保留空值和生成说明，生产值只能由 Docker Secret 或受控环境变量注入；
- 文档和脚本不得打印生成后的真实值。

### 4.2 密文合同

- 算法固定为 AES-256-GCM；
- 每次加密生成独立随机 nonce；
- 保存带版本号的密文信封，首版为 `v1:<base64 nonce+ciphertext>`；
- Additional Authenticated Data 固定包含表名、记录 ID、字段名和密文版本；
- 密文复制到其他记录或字段后必须解密失败；
- 解密失败只返回统一配置不可用错误，不记录密文、明文或底层密码学细节；
- 明文只在一次业务调用的局部变量中存在，不进入响应、结构化日志、指标 label 或操作记录。

该服务为全项目公共能力。以后支付之外的业务秘密需要落库时复用相同主密钥和服务，并通过 AAD 用途隔离；不得添加 `MOONBOOK_<MODULE>_MASTER_KEY` 一类变量。

## 5. 数据模型与迁移

实现前重新检查 `server/internal/platform/migrate/migrations/` 的最高版本，新增严格递增的 Goose 前向迁移，不修改已执行迁移。

扩展 `reader_payment_channels`：

- `display_name varchar(100)`：页面名称；
- `merchant_pid_ciphertext text`：PID 密文；
- `secret_ciphertext text`：Secret 密文；
- `create_url varchar(2048)`；
- `notify_url varchar(2048)`；
- `redirect_url varchar(2048)`；
- `health_url varchar(2048)`；
- `sync_url varchar(2048)`；
- `connect_timeout_ms integer`，默认 3000；
- `request_timeout_ms integer`，默认 10000；
- `unknown_release_minutes integer`，默认 15；
- `archived_at timestamptz`；
- `created_at timestamptz`，为既有行补齐稳定值。

保留 `provider`、`currency`、`token`、`network`、`enabled` 和 `updated_at`。移除 `provider` 的全表唯一约束，改为 `archived_at IS NULL` 条件下唯一，使旧记录归档后可以重新创建 EPUSDT。字段约束固定：

- `provider='epusdt'`、`currency='usd'`、`token='usdt'`、`network='tron'`；
- 超时为有界正整数，总超时不得小于连接超时；
- 未归档且启用的渠道必须具备完整密文和必填 URL；
- URL 必须由应用层结构化验证为绝对 HTTP/HTTPS URL，不允许用户信息、片段或 opaque URL。

渠道 ID 使用 PostgreSQL identity 生成，所有管理 API 继续以 JSON 字符串返回和接收，管理前端不得用 JavaScript `Number` 处理。

迁移不读取环境变量、不生成密文、不自动启用既有 `id=1` 记录。既有开发记录保持禁用，管理员可编辑后使用；若没有有效记录，也可归档后新建。

## 6. 服务边界与运行数据流

### 6.1 配置仓库

Commerce 内新增只读运行配置 Provider，由管理 CRUD 和支付业务共享同一 PostgreSQL 表：

```text
管理页面 -> adminpayment.Service -> ChannelRepository -> PostgreSQL
                                      -> secretcrypto

Reader 创建订单 -> RuntimePaymentConfigProvider -> PostgreSQL -> secretcrypto
EPUSDT 回调     -> RuntimePaymentConfigProvider -> PostgreSQL -> secretcrypto
管理同步/检查   -> RuntimePaymentConfigProvider -> PostgreSQL -> secretcrypto
```

管理 Repository 负责持久化密文和非秘密字段；运行 Provider 负责读取启用渠道、解密并构造 `epusdt.Config` 与 `CredentialProvider`。HTTP 协议客户端不直接访问数据库，支付 Repository 不执行网络请求。

不使用长生命周期配置缓存。每次创建订单、回调验签、主动同步和连通性检查读取一次一致配置快照，单次调用内复用该快照。管理修改提交后的下一次业务调用立即使用新配置。

### 6.2 写入语义

- 新建时 PID、Secret 和所有必填 URL 必须完整；
- 详情响应只返回 `pidConfigured`、`secretConfigured`，不返回掩码假值或密文；
- 编辑时 PID/Secret 字段省略或空白表示保持原值；
- 修改 PID 时必须同时提交新的 Secret，修改 Secret 可保留原 PID；
- 页面不提供普通“清空 Secret”动作；归档在同一事务中禁用渠道并清除 PID、Secret 密文，只保留非秘密渠道元数据和审计事实；
- 只有不存在进行中订单时才允许替换凭据或归档。

进行中状态固定为 `creating`、`pending`、`gateway_unknown`、`callback_exception`。保护检查和写入必须处于同一数据库事务并锁定渠道记录，不能先查后写形成竞态。当前开发数据库没有真实订单，因此首次新建和配置不受历史数据阻塞。

### 6.3 启用与错误

- 未配置完整时渠道只能保存为禁用；
- 启用前执行与创建订单相同的结构校验，但不要求健康检查成功；
- 支付运行配置缺失、密文不可解或字段非法时拒绝创建订单，不退化为本地待支付订单；
- 回调配置不可用时返回现有可重试失败合同，并记录脱敏审计；
- 数据库冲突、活动订单保护和渠道不存在分别返回稳定、可翻译的业务错误。

## 7. 管理 API

管理路由沿用 `/reader/payment` 分组：

| 方法 | 路径 | 作用 |
|---|---|---|
| `GET` | `/reader/payment/channels` | 查询渠道，支持 `includeArchived` |
| `GET` | `/reader/payment/channels/:id` | 查询脱敏详情 |
| `POST` | `/reader/payment/channels` | 新建 EPUSDT 渠道 |
| `PUT` | `/reader/payment/channels/:id` | 修改完整配置或启停 |
| `DELETE` | `/reader/payment/channels/:id` | 安全归档 |
| `POST` | `/reader/payment/channels/:id/check` | 使用当前保存配置检查连通性 |

所有 ID 必须是正整数字符串。请求 DTO 使用指针或等价显式存在性表达，区分“未提交”和“提交空值”。响应不得包含以下键及其命名变体：`pid`、`secret`、`ciphertext`、`signature`、`credential`。

Casbin 和 `sys_apis` 分别登记查看、新建、修改、归档和检查权限。默认管理员角色获得这些权限，其他角色由菜单和 API 权限管理显式授予。

## 8. 管理页面

页面继续使用现有 GVA 和 Element Plus 约定，保持业务工具风格：

- 顶部提供新建、刷新和“显示已归档”开关；
- 表格展示名称、提供商、资产/网络、配置状态、启用状态、更新时间和操作；
- 行操作包括编辑、启停、检查和归档；熟悉动作使用图标按钮并提供 tooltip；
- 新建/编辑使用单层对话框，分为基本信息、商户凭据、接口地址和超时设置；
- 提供商固定为 EPUSDT，不提供伪可用的自定义提供商选项；
- PID 和 Secret 使用密码输入控件，新建必填，编辑不预填；
- URL 使用普通文本输入并显示字段级校验错误；
- 超时使用数值输入，但渠道 ID 始终使用字符串输入和提交；
- 归档要求二次确认，存在进行中订单时展示后端返回的阻止原因；
- 检查结果展示 `reachable`、`disabled`、`not_configured`、`unreachable` 等脱敏状态。

页面不得放置解释密钥安全机制、加密算法或操作教程的可见说明文字；必要提示放在字段 label、校验信息和确认对话框中。

## 9. 审计与秘密防泄漏

写接口继续使用 `middleware.OperationRecord()`，但必须在进入通用操作记录前清洗敏感请求字段。操作记录只能保留变更字段名、渠道 ID、提供商、结果和管理员身份，不保存 PID、Secret、密文或完整认证 URL 查询。

必须覆盖以下泄漏面：

- GVA 操作记录请求体和响应体；
- 访问日志、Zap 字段和 panic 日志；
- `apperror` 公共消息；
- 连通性检查错误；
- Prometheus label；
- 单元测试失败输出、快照和集成测试夹具；
- Compose、`.env.example`、设计文档和部署命令输出。

PID 按秘密处理，不返回部分掩码。连通性检查只访问数据库保存且通过验证的 `health_url`，不发送 PID、Secret 或签名，保持两秒级有界超时或采用渠道配置中更小的连接限制。

## 10. 环境与部署

`.env.example` 新增：

```dotenv
# Base64-encoded 32-byte key shared by all database-encrypted application secrets.
MOONBOOK_APP_MASTER_KEY=
```

删除 EPUSDT 专属 PID、Secret 和端点作为运行必需配置的说明；旧变量可以在一个发布周期内保留为空模板以提示废弃，但应用不得读取它们。

部署文档必须说明：

1. 首次部署前在受控环境生成 32 个随机字节并 Base64 编码；
2. 将值注入 Server 容器，不能提交到 Git 或写进普通 Compose 文件；
3. 备份数据库时必须独立备份同一主密钥，否则密文不可恢复；
4. 所有实例必须使用同一值；
5. 丢失主密钥只能重新配置业务秘密，不能从密文恢复；
6. 未实现在线轮换前不得直接替换生产主密钥；
7. 后续模块必须复用该变量和 `secretcrypto`，不得新增专属主密钥。

发布顺序为：生成并注入主密钥、运行数据库迁移、启动后端、在管理后台创建并检查渠道、启用渠道、再开放 Reader 充值入口。回滚旧镜像前必须先禁用新渠道；旧镜像不会读取数据库密文。

## 11. 测试与验收

### 11.1 加密与配置单元测试

- 正确加解密、随机 nonce、空值和长度限制；
- 密文或 AAD 被篡改、跨记录/跨字段复制时解密失败；
- 主密钥缺失、Base64 非法和长度错误；
- 错误、日志、字符串格式化和测试输出不包含明文或密文。

### 11.2 PostgreSQL 与后端集成测试

- 空库迁移、当前最高版本升级、重复迁移 `applied=0`；
- 新建、列表、详情、修改、启停、检查、归档和查看归档；
- 同一未归档 provider 并发创建只能成功一次；
- Secret 不回显，空白编辑保持原值；
- 进行中订单阻止凭据替换和归档；
- 配置修改后的下一次创建、同步和回调使用新值；
- 合法回调仍只产生一套订单、钱包流水和奖励事实；
- 操作记录、访问日志和错误响应不含 PID、Secret 或密文；
- 所有 Long ID 均为 JSON 字符串。

### 11.3 前端和协议验证

- 管理 API 模块和页面表单测试；
- 新建、编辑留空、启停、检查、归档和显示归档交互；
- 最长 URL、错误消息和窄视口不溢出或重叠；
- ESLint 与管理前端生产构建；
- EPUSDT HTTP 替身覆盖创建、检查、同步和回调验签；
- Commerce/Reader 定向 Go 测试、race 风险路径和 `go vet`。

## 12. 完成判据

以下条件全部满足才算实现完成：

1. 管理员可在页面完成 EPUSDT 渠道新建、脱敏查看、编辑、启停、检查和归档；
2. 保存后的配置无需重启即可用于全部支付路径；
3. 数据库、API、日志、审计和前端状态中均无秘密明文；
4. 当前开发环境无需导入旧 EPUSDT 环境变量或历史凭据；
5. 进行中订单保护、数据库唯一性和支付幂等保持有效；
6. 项目规范和部署文档明确唯一主密钥规则；
7. 迁移、后端、前端、协议替身和生产构建验证全部通过；
8. 有效代码和文档按功能使用简体中文提交，工作区无遗漏的有效修改。
