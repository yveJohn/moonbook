# Moonbook 模块边界整改实施计划

**目标：** 消除 Reader、Commerce、Novel 的 15 处跨模块实现引用和已审计的跨域 SQL，保持冻结 Reader 契约及注册奖励、点赞、购买、首充奖励的原子性，并恢复 M7 后端质量门。

**架构：** 三个业务域只通过各自 `contract` 包协作，`initialize` 是唯一实现组合根。Platform 提供 Context 绑定的 PostgreSQL Transactor；Commerce 使用自有最小 Reader 搜索投影支撑管理端准确筛选、计数和分页，所有鉴权和交易判断继续使用实时 Reader 合同。

**技术栈：** Go、Gin、PostgreSQL、Redis、MinIO、Gin-Vue-Admin、现有版本迁移 Runner、真实依赖集成测试、Linux race。

**批准规格：** `docs/superpowers/specs/2026-08-15-moonbook-module-boundary-remediation-design.md`。

---

## 执行规则

- [ ] 每个任务先补失败测试并记录预期失败原因，再做最小实现。
- [ ] Go 命令在 `server` 目录执行，统一使用 `CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache`；需要 race 时在 Linux 执行。
- [ ] 真实集成测试只能使用项目专用 PostgreSQL、Redis 和 MinIO，不连接生产环境。
- [ ] 不修改冻结仓库和 `reader-ui` 业务代码，不修改、删除或改名 `00001` 至 `00058`。
- [ ] 不跳过、删除或放宽 `TestModuleDependencyRules`，不为当前违规增加白名单。
- [ ] 每个逻辑任务完成验证后检查全部工作区修改，使用简体中文独立提交。
- [ ] 管理前端恶意依赖未整改前，不执行或信任生产构建；该项持续标记为 M7 安全 No-Go。

## Task 1：建立 Context 事务基础设施

**文件：**

- Create: `server/internal/platform/transaction/context.go`
- Create: `server/internal/platform/transaction/transactor.go`
- Create: `server/internal/platform/transaction/after_commit.go`
- Create: `server/internal/platform/transaction/existing.go`
- Create: `server/internal/platform/transaction/transactor_test.go`
- Create: `server/internal/platform/transaction/transactor_integration_test.go`

- [x] **Step 1：编写失败测试。** 覆盖最外层提交、回调错误回滚、panic 回滚并继续传播、嵌套加入、嵌套错误被上层吞掉后仍 rollback-only、提交失败、`AfterCommit` 仅成功提交后按注册顺序执行，以及回调失败不反转已提交事实。
- [x] **Step 2：确认失败。** 运行 `go test ./internal/platform/transaction -v`，预期因包和 API 尚不存在失败。
- [x] **Step 3：实现 Transactor。** 提供 `Within(ctx, fn)`、Context 内部事务解析和最小 `DBTX` 接口；业务合同不暴露 `*sql.Tx`。嵌套作用域复用现有事务并共享 rollback-only 状态，只有最外层提交或回滚；另提供仅供 Platform 适配既有 Runner 外层事务的 `WithExisting(ctx, tx, fn)`，该入口不提交或回滚调用方拥有的事务，但共享 rollback-only 状态。
- [x] **Step 4：真实 PostgreSQL 验证。** 使用隔离表验证嵌套错误、panic、并发调用和 AfterCommit；测试自行清理唯一 fixture，不使用无条件清表。
- [x] **Step 5：质量检查。** 运行 `gofmt`、`go test ./internal/platform/transaction -v` 和 `go vet ./internal/platform/transaction`。
- [x] **Step 6：提交。** 提交信息：`新增上下文事务协调器`。

## Task 2：建立三域窄合同与稳定错误

**文件：**

- Create: `server/internal/modules/reader/contract/account.go`
- Create: `server/internal/modules/reader/contract/invite.go`
- Create: `server/internal/modules/reader/contract/display.go`
- Create: `server/internal/modules/reader/contract/errors.go`
- Create: `server/internal/modules/commerce/contract/account.go`
- Create: `server/internal/modules/commerce/contract/catalog.go`
- Create: `server/internal/modules/commerce/contract/reward.go`
- Create: `server/internal/modules/commerce/contract/errors.go`
- Create: `server/internal/modules/novel/contract/public.go`
- Create: `server/internal/modules/novel/contract/purchase.go`
- Create: `server/internal/modules/novel/contract/likes.go`
- Create: `server/internal/modules/novel/contract/errors.go`
- Create: `server/internal/modules/reader/contract/contract_test.go`
- Create: `server/internal/modules/commerce/contract/contract_test.go`
- Create: `server/internal/modules/novel/contract/contract_test.go`

- [x] **Step 1：编写合同形状测试。** 断言 DTO 不含 Gin Context、GVA Model、Repository、表名、`*sql.DB` 或 `*sql.Tx`；Long ID 仍为 Go `int64` 领域值，HTTP 字符串化留在 Reader 兼容层。
- [x] **Step 2：编写错误分类测试。** 覆盖 not found、disabled、conflict、temporary、timeout 和 unknown，保证 `errors.Is` 可用且消息不含 SQL、表名、对象键、DSN 或凭据。
- [x] **Step 3：定义窄接口。** 按账号校验/锁定、显示快照、邀请关系、商业汇总、商品访问、奖励参与者、公开内容、购买快照和点赞汇总拆分，不建立全域巨型 Service。
- [x] **Step 4：增加合同包依赖测试。** 证明合同包只依赖标准库和同领域合同 DTO，不反向引用任何 Provider 或实现包；Provider 编译期断言随对应 Adapter 任务加入。
- [x] **Step 5：验证并提交。** 运行 `go test ./internal/modules/reader/contract ./internal/modules/commerce/contract ./internal/modules/novel/contract -v`；提交信息：`建立跨领域业务合同`。

## Task 3：新增 Commerce Reader 搜索投影

**文件：**

- Create: `server/internal/platform/migrate/migrations/00059_commerce_reader_search_projection.sql`
- Modify: `server/internal/platform/migrate/migrate_test.go`
- Create: `server/internal/modules/commerce/readersearch/types.go`
- Create: `server/internal/modules/commerce/readersearch/repository.go`
- Create: `server/internal/modules/commerce/readersearch/service.go`
- Create: `server/internal/modules/commerce/readersearch/service_test.go`
- Create: `server/internal/modules/commerce/readersearch/repository_integration_test.go`
- Create: `server/internal/modules/commerce/provider/reader_search.go`

- [x] **Step 1：确认迁移最高版本。** 再次检查迁移目录最高版本仍为 `00058`；若已有更高迁移，停止并按真实最高版本顺延，禁止版本重复或倒序。
- [x] **Step 2：编写失败测试。** 静态测试要求表字段只含 `reader_id/username/nickname/status/created_at/updated_at`，无跨域外键，具备约束、查询索引和 forward-only 保护。
- [x] **Step 3：实现 `00059`。** 创建 `commerce_reader_search_projection` 并用 `INSERT ... SELECT ... ON CONFLICT DO UPDATE` 从 `reader_accounts` 幂等回填；迁移是唯一允许直接读取源表完成初始投影的边界。
- [x] **Step 4：实现 Commerce 自有仓储与 Adapter。** 提供幂等 upsert、按 ID 删除、分页快照、字段漂移比较所需读法；`commerce/provider/reader_search.go` 实现合同并添加编译期断言，运行时代码不得查询 `reader_accounts`。
- [x] **Step 5：真实迁移验证。** 分别验证空库迁移、已有账号升级、重复执行 `applied=0`、账号行数不减少、投影字段零差异。
- [x] **Step 6：验证并提交。** 运行迁移清单测试和 `go test ./internal/modules/commerce/readersearch -v`；提交信息：`新增交易域读者搜索投影`。

## Task 4：账号写流程与旧迁移同步投影

**文件：**

- Modify: `server/internal/modules/reader/invite/repository.go`
- Modify: `server/internal/modules/reader/invite/service.go`
- Modify: `server/internal/modules/reader/adminuser/repository.go`
- Modify: `server/internal/modules/reader/adminuser/service.go`
- Modify: `server/internal/platform/legacymigrate/reader_identity.go`
- Modify: `server/internal/platform/legacymigrate/runner.go`
- Modify: `server/internal/platform/legacymigrate/runner_test.go`
- Modify: `server/internal/platform/legacymigrate/reader_migration_test.go`
- Modify: `server/internal/platform/legacymigrate/reader_migration_integration_test.go`
- Create: `server/internal/modules/reader/accountsync/service.go`
- Create: `server/internal/modules/reader/accountsync/service_test.go`
- Modify: `server/initialize/router_biz.go`

- [x] **Step 1：编写失败测试。** 覆盖现有注册和管理端状态变更同步；Reader 写失败不改投影，投影写失败使 Reader 写回滚，重复 upsert 不产生多行。密码重置和摘要升级不改变投影字段，不得为同步投影改动它们的事务语义。
- [x] **Step 2：实现同事务同步。** Reader 写流程通过 `commerce/contract` 调用投影 Provider，双方从 Context 取得同一事务；不得在 Reader 中直接写 Commerce 表。
- [x] **Step 3：改造旧账号迁移。** Runner 使用 `WithExisting` 把每批目标事务绑定进 Stage Context；账号 Stage 写入 `reader_accounts` 后通过 Commerce 合同同步投影。检查点、账号和投影由 Runner 的同一事务提交或回滚，幂等重跑保持零差异。
- [x] **Step 4：实现一致性应用服务。** Reader 分页读取源快照，通过 Commerce 合同读取投影，报告 missing/extra/mismatch；显式修复模式按 Reader 事实 upsert 或删除，日志不输出账号敏感字段。
- [x] **Step 5：真实 PostgreSQL 验证。** 注入投影故障验证回滚；验证迁移中断恢复、重复执行和一致性修复归零。
- [x] **Step 6：提交。** 提交信息：`同步读者账号搜索投影`。

## Task 5：把 Reader-facing Commerce HTTP 迁入 Reader

**文件：**

- Create: `server/internal/modules/reader/commercecompat/checkin.go`
- Create: `server/internal/modules/reader/commercecompat/purchase.go`
- Create: `server/internal/modules/reader/commercecompat/recharge.go`
- Create: `server/internal/modules/reader/commercecompat/wallet.go`
- Create: `server/internal/modules/reader/commercecompat/*_contract_test.go`
- Remove after route parity: `server/internal/modules/commerce/checkin/http.go`
- Remove after equivalent tests pass: `server/internal/modules/commerce/checkin/http_contract_test.go`
- Remove after route parity: `server/internal/modules/commerce/purchase/http.go`
- Remove after equivalent tests pass: `server/internal/modules/commerce/purchase/http_contract_test.go`
- Remove after route parity: `server/internal/modules/commerce/recharge/http.go`
- Remove after equivalent tests pass: `server/internal/modules/commerce/recharge/http_contract_test.go`
- Remove after route parity: `server/internal/modules/commerce/wallet/http.go`
- Remove after equivalent tests pass: `server/internal/modules/commerce/wallet/http_contract_test.go`
- Modify: `server/initialize/router_biz.go`

- [x] **Step 1：冻结路由和响应测试。** 复制行为断言而非实现，逐接口核对路径、方法、认证、HTTP 200 包装、业务 code/message、`data/rows/total` 空值、日期和 Long ID 字符串。
- [x] **Step 2：确认新 Handler 测试失败。** 新兼容层只依赖 Reader 内部 auth/wire 和 Commerce contract 替身。
- [x] **Step 3：迁移 HTTP 编排。** Commerce 保留签到、购买、充值、钱包规则与 Provider；Reader Handler 仅做解析、认证、合同调用和冻结错误映射。
- [x] **Step 4：切换组合根。** `initialize` 注入 Commerce Provider 并只注册一次冻结路由；用路由清单测试确认无缺失、重复或方法变化。
- [x] **Step 5：删除旧 HTTP 实现并验证。** 先证明新 Reader 合同测试逐断言覆盖四份旧测试且用例数不减少，再删除旧 HTTP 与旧位置测试；运行新合同测试、`TestModuleDependencyRules` 定向测试和 Reader 路由清单。
- [x] **Step 6：提交。** 提交信息：`归并读者交易兼容接口`。

## Task 6：通过 Novel 合同重构 Reader 公开内容

**文件：**

- Modify: `server/internal/modules/reader/public/http.go`
- Modify: `server/internal/modules/reader/public/service.go`
- Modify: `server/internal/modules/reader/public/types.go`
- Modify: `server/internal/modules/reader/public/*_test.go`
- Create: `server/internal/modules/novel/provider/public.go`
- Create: `server/internal/modules/novel/provider/public_test.go`
- Modify: `server/initialize/router_biz.go`

- [x] **Step 1：用合同替身补失败测试。** 覆盖书库分页、发布过滤、详情、章节导航、分类、SEO、sitemap、正文 MinIO 大小/SHA-256 校验、缺失对象和基础设施错误脱敏。
- [x] **Step 2：实现 Novel 批量/公开合同 Adapter。** `novel/provider/public.go` 组合现有 books、chapters、objectstore 和 readerseo 能力并添加编译期合同断言；查询只发生在 Novel 内，正文读取继续由 Novel 校验对象，批量结果保持顺序、缺失项和空值。
- [x] **Step 3：改造 Reader Service。** Reader 组合 Novel 内容与 Commerce 访问/价格合同，不引用具体 catalog、objectstore 或 readerseo 类型。
- [x] **Step 4：组合根注入。** 只有 `initialize` 构造三个实现并注入 Reader public。
- [x] **Step 5：验证并提交。** 运行 Reader public 单元/真实 PostgreSQL+Redis+MinIO 集成及冻结契约；提交信息：`重构读者公开内容边界`。

## Task 7：重构 Reader 账号商业视图与个人内容投影

**文件：**

- Modify: `server/internal/modules/reader/account/http.go`
- Modify: `server/internal/modules/reader/account/http_contract_test.go`
- Modify: `server/internal/modules/reader/me/repository.go`
- Modify: `server/internal/modules/reader/me/service.go`
- Modify: `server/internal/modules/reader/me/*_test.go`
- Create: `server/internal/modules/commerce/provider/account_summary.go`
- Create: `server/internal/modules/novel/provider/display.go`
- Modify: `server/initialize/router_biz.go`

- [x] **Step 1：补失败测试。** 固定权益、会员商品、邀请面板、书架、点赞、历史的排序、空值、日期、已发布过滤和 Long ID 字符串。
- [x] **Step 2：账号商业视图改用 Commerce 合同。** Reader 不查询权益、会员、商品、奖励配置或钱包流水表。
- [x] **Step 3：个人内容使用 Novel 批量快照。** Reader 先读自身书架/历史/点赞事实，去重收集 book/chapter ID，一次批量调用后稳定组装，缺失或未发布目标按冻结语义处理。
- [x] **Step 4：证明无 N+1。** 使用计数替身和真实数据库查询统计，断言每页最多固定次数合同调用。
- [x] **Step 5：验证并提交。** 提交信息：`重构读者跨域查询投影`。

## Task 8：重构邀请注册与奖励原子事务

**文件：**

- Modify: `server/internal/modules/reader/invite/repository.go`
- Modify: `server/internal/modules/reader/invite/service.go`
- Modify: `server/internal/modules/reader/invite/*_test.go`
- Modify: `server/internal/modules/commerce/invitereward/repository.go`
- Modify: `server/internal/modules/commerce/wallet/repository.go`
- Create: `server/internal/modules/commerce/provider/reward.go`

- [x] **Step 1：编写故障矩阵。** 在账号、投影、邀请码计数、关系、邀请人奖励、受邀人奖励各阶段注入失败，断言全部事实回滚。
- [x] **Step 2：使用 Platform Transactor。** Reader 编排锁定邀请码、创建账号/自动码/关系/投影，再调用 Commerce 奖励合同；Provider 参与现有事务，不自行提交。
- [x] **Step 3：保持幂等与锁序。** advisory lock 后依次锁 Reader 事实、Commerce 奖励事实和钱包；重复注册或奖励请求不能重复流水。
- [x] **Step 4：真实并发验证。** 覆盖同一邀请码并发、奖励冲突、嵌套错误被误吞及无半注册/单边奖励。
- [x] **Step 5：提交。** 提交信息：`重构邀请注册奖励事务`。

## Task 9：重构点赞关系与 Novel 汇总原子事务

**文件：**

- Modify: `server/internal/modules/reader/me/repository.go`
- Modify: `server/internal/modules/reader/me/service.go`
- Modify: `server/internal/modules/reader/me/service_integration_test.go`
- Create: `server/internal/modules/novel/provider/likes.go`

- [x] **Step 1：编写失败与并发测试。** 覆盖无效书籍、重复点赞、重复取消、Reader 关系失败、Novel 汇总失败、并发点赞/取消和 rollback-only。
- [x] **Step 2：实现合同事务。** Novel 合同锁定有效书籍；Reader 幂等改关系并计算精确关系数；Novel 合同更新 `like_count`，双方加入同一 Context 事务。
- [x] **Step 3：移除 Reader 对 Novel 表 SQL。** Reader 只访问点赞关系和其他 Reader 表。
- [x] **Step 4：真实 PostgreSQL 验证。** 并发完成后 `like_count` 必须等于关系行数，不接受最终一致窗口。
- [x] **Step 5：提交。** 提交信息：`重构读者点赞汇总事务`。

## Task 10：改造 Commerce 管理端 Reader 查询

**文件：**

- Modify: `server/internal/modules/commerce/adminmembership/repository.go`
- Modify: `server/internal/modules/commerce/adminmembership/service.go`
- Modify: `server/internal/modules/commerce/adminorder/repository.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/repository.go`
- Modify: `server/internal/modules/commerce/adminwallet/repository.go`
- Modify: `server/internal/modules/commerce/adminmembership/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/adminorder/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/adminwallet/repository_integration_test.go`
- Modify: `server/initialize/router_biz.go`
- Create: `server/internal/modules/reader/provider/account.go`

- [ ] **Step 1：补筛选分页失败测试。** 订单和充值订单按订单号或用户名搜索时总数、稳定排序和分页准确；钱包按用户名或昵称搜索，并显示没有钱包事实的账号。
- [ ] **Step 2：列表查询使用自有投影。** 订单/充值订单 JOIN `commerce_reader_search_projection`；钱包以投影为驱动表 LEFT JOIN `reader_wallets`。不得读取 `reader_accounts`。
- [ ] **Step 3：写操作使用实时 Reader 合同。** 会员发放、人工补单、钱包调账继续实时校验并按统一锁序锁定 Reader，不能使用投影判断账号状态。
- [ ] **Step 4：错误语义回归。** 保持不存在、禁用、余额不足、幂等冲突和管理端 GVA 错误映射。
- [ ] **Step 5：真实 PostgreSQL 验证并提交。** 提交信息：`整改交易管理读者查询`。

## Task 11：重构商品目标与购买报价

**文件：**

- Modify: `server/internal/modules/commerce/adminproduct/repository.go`
- Modify: `server/internal/modules/commerce/adminproduct/service.go`
- Modify: `server/internal/modules/commerce/purchase/repository.go`
- Modify: `server/internal/modules/commerce/purchase/service.go`
- Modify: `server/internal/modules/commerce/adminproduct/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/purchase/repository_integration_test.go`
- Create: `server/internal/modules/novel/provider/purchase.go`
- Modify: `server/initialize/router_biz.go`

- [ ] **Step 1：补合同与故障测试。** 覆盖书籍/章节不存在、未发布/禁用、免费章节、字数计价、固定价、报价变化和 Provider 故障。
- [ ] **Step 2：商品管理改用 Novel 合同。** Commerce 不查询书籍或章节表，创建/更新商品时通过目标快照校验。
- [ ] **Step 3：购买使用同一事务。** 依次取得幂等锁、实时 Reader 校验、Novel 有效购买快照，再写 Commerce 订单、扣款、流水和权益。
- [ ] **Step 4：故障矩阵验证。** 任一步失败均不留下订单、扣款、流水或权益；重复请求返回原结果。
- [ ] **Step 5：移除跨域 SQL并提交。** 提交信息：`重构商品目标与购买事务`。

## Task 12：重构首充邀请奖励

**文件：**

- Modify: `server/internal/modules/commerce/invitereward/repository.go`
- Modify: `server/internal/modules/commerce/payment/repository.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/repository.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/sync.go`
- Modify: `server/internal/modules/commerce/invitereward/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/payment/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/sync_integration_test.go`
- Create: `server/internal/modules/reader/provider/invite.go`

- [ ] **Step 1：补跨入口并发测试。** 模拟充值、支付回调和人工补单同时完成时只产生一份首充奖励事实和流水。
- [ ] **Step 2：替换邀请关系 SQL。** Commerce 通过 Reader 合同读取有效邀请关系并加入当前事务，不直接查询 `reader_invite_relations`。
- [ ] **Step 3：保持既有幂等事实。** 充值订单锁、首充奖励唯一键和钱包流水唯一键继续共同防重。
- [ ] **Step 4：故障回滚。** Reader 合同、奖励事实或钱包任一失败，订单终态和入账全部回滚。
- [ ] **Step 5：真实 PostgreSQL 验证并提交。** 提交信息：`重构首充邀请奖励事务`。

## Task 13：增加 SQL 所有权与组合根静态门

**文件：**

- Keep unchanged: `server/internal/modules/dependency_test.go`
- Create: `server/internal/modules/sql_ownership_test.go`
- Create: `server/internal/modules/composition_root_test.go`
- Create: `server/internal/modules/table_ownership_test.go`

- [ ] **Step 1：先让测试报告当前越界。** `table_ownership_test.go` 用类型化 Go map 固定 Reader/Commerce/Novel 表归属；Go AST 扫描非测试 Go 源码中的 SQL 字符串常量和可解析拼接并报告跨域访问。
- [ ] **Step 2：固定例外边界。** 只扫描业务模块运行时代码；版本迁移和测试 fixture 不属于业务运行时。动态表名只允许同领域显式枚举，禁止任意字符串逃逸。
- [ ] **Step 3：增加组合根检查。** 业务包不能构造其他领域 Provider；跨域实现装配只能出现在 `initialize`。
- [ ] **Step 4：禁止投影越权。** 静态或架构测试证明交易决策代码不依赖 `commerce_reader_search_projection` 查询结果，并禁止 Commerce 运行时读取 `reader_accounts`。
- [ ] **Step 5：运行门禁。** `go test ./internal/modules -run 'TestModuleDependencyRules|TestSQLOwnership|TestCompositionRoot' -v` 必须全部通过。
- [ ] **Step 6：提交。** 提交信息：`增加模块数据所有权检查`。

## Task 14：执行全量回归并固定验收证据

**文件：**

- Modify: `docs/verification/m7-backend-quality-baseline.md`
- Modify: `docs/verification/m7-module-boundary-workflow-inventory.md`
- Modify: `docs/progress/refactor-status.md`
- Reference without contract changes: `docs/contracts/reader-api.md`
- Modify: `docs/migration/mapping.md`

- [ ] **Step 1：普通后端质量门。** 运行 gofmt 检查、`go mod verify`、与 CI 同范围的 `go vet` 和 `go test`；不得排除失败包。
- [ ] **Step 2：真实依赖回归。** 运行涉及 Reader、Commerce、Novel、迁移和事务的真实 PostgreSQL/Redis/MinIO 集成测试，确认 fixture 清理不减少其他业务数据。
- [ ] **Step 3：M3 零修改回归。** 运行 `make verify-m3`、冻结 Reader 树差异、536 个基线用例、契约、SSR 和浏览器关键旅程；路由清单必须无变化。
- [ ] **Step 4：Linux race。** 在 Linux CI 或等价环境运行目标包及跨域并发测试的 `go test -race`，保存固定 commit 与日志定位信息。
- [ ] **Step 5：可信前端边界。** 只运行不触发恶意依赖的静态检查；生产构建继续明确 No-Go，直到供应链整改独立完成。
- [ ] **Step 6：更新证据。** 记录命令、结果、验收 commit、投影一致性统计、剩余风险和 M7 安全阻断，不把模块整改误写为 M4-M7 全部完成。
- [ ] **Step 7：最终检查与提交。** 检查 tracked/untracked 文件、秘密、构建产物和 `git diff --check`；提交信息：`补充模块边界整改验收证据`。

## 最终验收清单

- [ ] 15 处跨模块实现引用全部消除，原样 `TestModuleDependencyRules` 通过。
- [ ] Reader、Commerce、Novel 运行时代码不直接访问其他领域表，SQL 所有权测试通过。
- [ ] `initialize` 是唯一跨领域实现组合根。
- [ ] Reader 拥有全部冻结 Reader HTTP 路由、认证、DTO 和错误映射。
- [ ] Commerce 搜索投影与 Reader 账号零差异；筛选、总数、分页和无钱包账号语义正确。
- [ ] 投影不参与认证、状态锁定、调账、购买、会员授予或权益决定。
- [ ] 注册奖励、点赞、购买和首充奖励具备真实 PostgreSQL 回滚、幂等和并发证据。
- [ ] Reader 跨域查询使用批量合同，无 N+1，排序、缺失关联、空值和分页语义保持。
- [ ] 冻结 Reader 契约、Long ID 字符串、SSR、536 个基线用例和关键旅程无回归。
- [ ] Go 普通测试、vet、迁移验证和 Linux race 通过，报告固定到验收 commit。
- [ ] 管理前端恶意依赖仍作为独立 M7 安全 No-Go，未使用旧生产构建伪造证据。
- [ ] 工作区干净，所有有效修改按功能使用简体中文提交，未 push。

## 服务影响

实施代码将影响 `moonbook-admin`、应用内内容/交易 Worker、PostgreSQL schema 和旧数据迁移 CLI。Redis 与 MinIO 的协议和数据不变，但会参与全量 Reader 回归。每个后端代码任务部署后需重启 `moonbook-admin`；涉及 Worker 注入的任务需随同重启应用内 Worker；应用 `00059` 前必须先备份 PostgreSQL，并在启动新后端前完成迁移。仅执行本计划文档本身不影响服务，无需重启。
