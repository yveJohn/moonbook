# M3 读者身份与 Commerce 基础实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** 建立读者兼容 API 的身份基础、邀请码关系和 Commerce 只读权益事实，使冻结版 reader-ui 能在新 PostgreSQL/Redis/MinIO 环境完成注册、登录、会话和访问上下文准备。

**Architecture:** 在模块化单体中由 \`reader\` 拥有账号、会话、邀请码、邀请关系及兼容 HTTP DTO；由 \`commerce\` 拥有商品、章节定价、会员授予和权益表，并只暴露读取/判定合同。旧 MySQL 通过现有可恢复 Runner 分阶段写入 PostgreSQL，Token 只使用 Reader 专用会话摘要，管理员 Token 与 Reader 身份完全隔离。

**Tech Stack:** Go 1.24、Gin、GORM、PostgreSQL、Redis、MinIO、golang-jwt、bcrypt、现有 legacy-migrate Runner、Go test、Docker Compose。

---

## 文件范围

- Create: \`server/internal/platform/migrate/migrations/00012_reader_commerce_foundation.sql\` - Reader 与 Commerce 正式事实表、约束、索引和序列推进。
- Create: \`server/internal/modules/reader/auth/types.go\`, \`repository.go\`, \`service.go\`, \`http.go\`, \`middleware.go\`, \`response.go\` - Reader 身份、兼容响应和 Reader-only JWT 会话。
- Create: \`server/internal/modules/reader/auth/*_test.go\` - 密码兼容、邀请码原子消费、会话撤销、响应和鉴权测试。
- Create: \`server/internal/modules/commerce/catalog/types.go\`, \`repository.go\`, \`service.go\`、对应测试 - 商品、章节定价、会员授予、权益读取和批量 AccessReader 判定。
- Modify: \`server/initialize/router_biz.go\` - 注册 Reader 兼容公开/私有路由并注入 PostgreSQL、Redis。
- Modify: \`server/internal/modules/reader/doc.go\`, \`server/internal/modules/commerce/doc.go\` - 固化模块所有权与公开合同说明。
- Create: \`server/internal/platform/legacymigrate/reader_identity.go\`, \`reader_commerce.go\` 及测试/fixture - 账号、邀请码、邀请关系、商品、定价、会员和权益迁移 stage。
- Modify: \`server/cmd/moonbook-legacy-migrate/main.go\` - 按固定顺序装配 M3 stages。
- Create: \`docs/contracts/reader-auth-api.md\`、\`docs/migration/m3-reader-commerce.md\` - 接口与映射证据。

## Task 1: 编写数据库迁移与约束

**Files:** Create \`server/internal/platform/migrate/migrations/00012_reader_commerce_foundation.sql\`; Test \`server/internal/platform/migrate/migrations/00012_reader_commerce_foundation_test.go\`。

- [ ] **Step 1: 写迁移静态检查测试**：断言版本为 00012、目标表齐全，包含 \`bigint\` 旧 ID、\`timestamptz\` 时间、唯一约束、邀请码消费约束、权益状态约束，并禁止 \`DROP DATABASE\`、\`TRUNCATE\`、无条件 \`DELETE\`。
- [ ] **Step 2: 运行失败测试**：\`go test ./internal/platform/migrate/migrations -run TestReaderCommerceMigration -v\`；预期因迁移文件不存在失败。
- [ ] **Step 3: 实现 SQL**：创建 \`reader_accounts\`、\`reader_sessions\`、\`reader_invite_codes\`、\`reader_invite_relations\`、\`reader_bookshelf_entries\`、\`reader_book_likes\`、\`reader_reading_history\`、\`reader_reading_preferences\`、\`reader_feedback\`、\`commerce_products\`、\`commerce_chapter_pricing_config\`、\`commerce_membership_grants\`、\`commerce_entitlements\`；为旧 ID 建唯一索引，为关系和状态建立检查约束；不要创建钱包、订单或支付写模型。
- [ ] **Step 4: 使用本地 Flyway/迁移 Runner 验证**：启动项目专用 PostgreSQL，执行 \`go test ./internal/platform/migrate/... -run TestMigrations -v\`，检查 schema history、重复执行幂等及关键表行数不减少。
- [ ] **Step 5: 提交**：\`git add server/internal/platform/migrate/migrations/00012_reader_commerce_foundation.sql server/internal/platform/migrate/migrations/00012_reader_commerce_foundation_test.go && git commit -m "新增读者与商品基础表"\`。

## Task 2: 实现 Reader 密码与会话核心

**Files:** Create \`server/internal/modules/reader/auth/types.go\`, \`repository.go\`, \`service.go\`, \`middleware.go\`; Test \`server/internal/modules/reader/auth/service_test.go\`, \`middleware_test.go\`。

- [ ] **Step 1: 写失败测试**：覆盖 BCrypt 登录、历史 32 位 MD5 登录后升级 BCrypt、未知摘要拒绝且不升级、禁用账号拒绝、Redis 限流不可用时注册/登录失败关闭、退出撤销会话、改密撤销其他会话。
- [ ] **Step 2: 运行测试确认失败**：\`go test ./internal/modules/reader/auth -run 'Test(password|Login|Session|RateLimit)' -v\`。
- [ ] **Step 3: 实现服务**：定义 \`ReaderAccount\`、\`ReaderSession\`、\`TokenClaims\`；实现 bcrypt 校验、旧 MD5 校验和成功后事务升级；JWT 只含 reader ID/session ID/过期时间；数据库保存 SHA-256 会话摘要；每次请求检查账号状态、过期和撤销；Redis 只实现 IP+账号限流。
- [ ] **Step 4: 运行通过测试**：\`go test ./internal/modules/reader/auth -v\`；预期全部 PASS。
- [ ] **Step 5: 提交**：\`git add server/internal/modules/reader/auth && git commit -m "实现读者认证与会话管理"\`。

## Task 3: 实现 Reader 兼容响应和认证路由

**Files:** Create \`server/internal/modules/reader/auth/http.go\`, \`response.go\`; Modify \`server/initialize/router_biz.go\`; Test \`server/internal/modules/reader/auth/http_test.go\`。

- [ ] **Step 1: 写 Gin 契约测试**：验证 \`POST /reader/auth/register\`、\`POST /reader/auth/login\`、\`POST /reader/auth/logout\`、\`GET /reader/auth/profile\`、\`PUT /reader/auth/password\`；成功 JSON 分别为 \`code=200,msg=操作成功\` 或 \`msg=查询成功\`，错误使用 HTTP 200 业务码；无效 Token 返回 \`code=401,msg=认证失败，无法访问系统资源\`。
- [ ] **Step 2: 运行失败测试**：\`go test ./internal/modules/reader/auth -run TestHTTP -v\`。
- [ ] **Step 3: 实现兼容层**：公开路由无 Token 按匿名处理；个人路由强制 Reader 身份；管理员 Token 不得注入 Reader ID；输出 ID 为十进制字符串；绑定固定错误消息，不复用 GVA response。
- [ ] **Step 4: 接入路由并验证**：在 \`initBizRouter\` 注入现有 DB/Redis，注册独立 \`/reader\` group；运行 \`go test ./initialize ./internal/modules/reader/auth -v\`。
- [ ] **Step 5: 提交**：\`git add server/initialize/router_biz.go server/internal/modules/reader/auth && git commit -m "接入读者兼容认证接口"\`。

## Task 4: 实现邀请码和邀请关系原子流程

**Files:** Create \`server/internal/modules/reader/invite/types.go\`, \`repository.go\`, \`service.go\`; Test \`server/internal/modules/reader/invite/service_test.go\`。

- [ ] **Step 1: 写失败测试**：验证邀请码必填、过期/禁用/超限拒绝、并发注册只能成功一次、成功注册原子增加 used_count、生成新读者自动邀请码并建立唯一邀请关系；失败事务不产生账号、邀请码或关系残留。
- [ ] **Step 2: 运行失败测试**：\`go test ./internal/modules/reader/invite -v\`。
- [ ] **Step 3: 实现事务服务**：使用 \`SELECT ... FOR UPDATE\` 锁定邀请码，在同一 PostgreSQL 事务中创建账号、消费邀请码、生成幂等自动码和关系事实；不创建钱包或奖励记录。
- [ ] **Step 4: 通过并发测试**：\`go test ./internal/modules/reader/invite -race -v\`。
- [ ] **Step 5: 提交**：\`git add server/internal/modules/reader/invite && git commit -m "实现读者邀请码关系"\`。

## Task 5: 建立 Commerce 目录与访问判定只读合同

**Files:** Create \`server/internal/modules/commerce/catalog/types.go\`, \`repository.go\`, \`service.go\`; Test \`server/internal/modules/commerce/catalog/service_test.go\`, \`integration_test.go\`。

- [ ] **Step 1: 写失败测试**：覆盖 \`word_charge\`、\`membership_only\`、\`login_free\`、\`fixed_price\`；匿名统一 \`login_required\`；整书权益优先章节权益；会员、免费章节和不可用收费模式返回冻结 access reason/code；批量输入不逐章查询。
- [ ] **Step 2: 运行失败测试**：\`go test ./internal/modules/commerce/catalog -v\`。
- [ ] **Step 3: 实现合同**：定义商品、章节报价、会员授予、权益及 \`AccessReader\` 结果；提供批量 repository 查询；只读服务不实现购买、扣款、支付或奖励。
- [ ] **Step 4: 使用真实 PostgreSQL 验证**：\`docker compose -f server/deploy/compose/docker-compose.yml up -d postgres redis minio\` 后运行 \`go test ./internal/modules/commerce/catalog -tags=integration -v\`，验证 Redis 清空后结果仍由 PostgreSQL 正确重建。
- [ ] **Step 5: 提交**：\`git add server/internal/modules/commerce && git commit -m "建立商品权益访问基础"\`。

## Task 6: 增加旧 MySQL 可恢复迁移 stages

**Files:** Create \`server/internal/platform/legacymigrate/reader_identity.go\`, \`reader_commerce.go\`；Modify \`server/cmd/moonbook-legacy-migrate/main.go\`; Test \`server/internal/platform/legacymigrate/reader_identity_test.go\`, \`reader_commerce_test.go\`。

- [ ] **Step 1: 写 stage 单元测试**：使用 MySQL fixture 验证游标分页、重复执行幂等、旧 ID 保留、BCrypt/MD5 算法标记、邀请码关系、商品/定价/会员/权益映射及错误清单不泄漏密码。
- [ ] **Step 2: 运行失败测试**：\`go test ./internal/platform/legacymigrate -run 'TestReader|TestCommerce' -v\`。
- [ ] **Step 3: 实现 stage**：按顺序迁移账号密码、邀请码关系、Commerce 历史权益、书架点赞、阅读历史偏好、反馈；所有写入 \`ON CONFLICT\` 幂等；每批写 checkpoint；来源缺失关联写可重试/不可重试错误，不静默丢弃。
- [ ] **Step 4: 装配并验证**：在 CLI 固定 stage 顺序，运行 \`go test ./cmd/moonbook-legacy-migrate ./internal/platform/legacymigrate -v\`。
- [ ] **Step 5: 提交**：\`git add server/cmd/moonbook-legacy-migrate/main.go server/internal/platform/legacymigrate && git commit -m "增加读者与权益迁移阶段"\`。

## Task 7: 固化接口、映射和验收证据

**Files:** Create \`docs/contracts/reader-auth-api.md\`, \`docs/migration/m3-reader-commerce.md\`; Modify \`docs/progress/refactor-status.md\`。

- [ ] **Step 1: 记录契约**：列出五个 auth 路由的请求字段、响应 JSON、HTTP 语义、错误码、Token 隔离和 JavaScript 字符串 ID 规则。
- [ ] **Step 2: 记录迁移映射**：列出旧表/字段到新表/字段、密码算法、冲突策略、checkpoint/stage 名称、核对 SQL 和 8GB 副本演练参数。
- [ ] **Step 3: 运行完整验证**：\`JAVA_HOME=/Users/yve/.vfox/cache/java/v-17.0.2+8/java-17.0.2+8 go test ./internal/... ./initialize/... ./cmd/moonbook-legacy-migrate/...\`；再运行本地 Docker 集成测试并保存输出摘要。
- [ ] **Step 4: 自审**：检查规格每节均有对应实现/测试；扫描计划中的占位表达，预期没有命中；核对所有 ID 类型和接口名称一致。
- [ ] **Step 5: 提交**：\`git add docs/contracts/reader-auth-api.md docs/migration/m3-reader-commerce.md docs/progress/refactor-status.md && git commit -m "补充M3认证迁移验收文档"\`。

## 验收清单

- PostgreSQL 00012 迁移可从空库执行、重复执行不变且不破坏业务数据。
- Reader 注册、登录、退出、资料和改密契约测试通过；旧 MD5 登录后只升级为 BCrypt。
- Reader Token 可撤销，管理员 Token 永不获得 Reader 身份；Redis 限流失败关闭。
- 邀请码消费、自动邀请码和邀请关系在并发下保持原子且幂等。
- Commerce AccessReader 批量判定与冻结收费模式一致，M3 不产生购买/支付写入。
- MySQL stages 可断点续跑、重复执行无重复事实，密码和敏感摘要不进入日志。
- 关键服务影响：\`moonbook-admin\`（后端路由/模块）、本地 PostgreSQL、Redis；使用 MinIO 的正文访问合同仅被读取。代码变更后需重启 \`moonbook-admin\` 以加载新路由；迁移验证只需重启本地基础设施，不得重启生产服务。
