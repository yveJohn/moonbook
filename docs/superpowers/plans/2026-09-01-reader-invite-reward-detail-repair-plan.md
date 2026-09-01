# Moonbook 读者邀请奖励明细修复实施计划

## 1. 目标与约束

- 依据 `docs/superpowers/specs/2026-09-01-reader-invite-reward-detail-repair-design.md` 实施。
- 冻结 `reader-ui` 业务代码保持不变，继续使用 `POST /reader/me/invite/code` 和现有弹窗。
- 邀请奖励配置、奖励事实、钱包和流水归 Commerce；邀请码和邀请关系归 Reader。
- `totalRewardCoin` 和 `rewards` 只以 `reader_invite_reward_records` 为事实来源，不从钱包流水动态拼装。
- 旧 MySQL 奖励事实继续由 `reader-finance` 迁移；`00076` 只修复能够从当前 Go 旧实现流水精确证明的运行期注册奖励事实。
- `00001` 至 `00075` 不修改、改名或删除；结构及数据修复使用新迁移 `00076`。
- 不补发资金、不修改余额或既有不可变流水、不连接或修改生产环境、不执行 `git push`。
- 所有 JavaScript 可见 Long ID 和奖励金额均以十进制字符串输出。
- 每个逻辑单元通过定向验证后使用简体中文提交，最终检查全部已跟踪和未跟踪修改。

## 2. 实施任务

### 任务一：增加奖励阶段唯一约束和确定性历史补偿

涉及文件：

- Create: `server/internal/platform/migrate/migrations/00076_reader_invite_reward_detail_repair.sql`
- Modify: `server/internal/platform/migrate/migrate_test.go`
- Modify: `server/internal/platform/migrate/migrate_integration_test.go`

实施步骤：

- [ ] 在迁移静态清单测试中先登记 `00076_reader_invite_reward_detail_repair.sql`，写出失败断言，确认测试因迁移缺失而失败。
- [ ] 新增真实 PostgreSQL 迁移场景测试，覆盖空库、`00075 -> 00076`、重复执行和事务回滚。
- [ ] 为同一 `(invitee_reader_id, reward_stage)` 重复事实增加迁移前置检查；发现重复时抛出明确异常，不删除、不合并、不选择记录。
- [ ] 创建 `(invitee_reader_id, reward_stage)` 唯一索引，保留既有 `idempotency_key` 唯一约束。
- [ ] 只识别当前旧 Go 写入模式：邀请人 bonus 收入流水、`biz_type='invite_reward'`、幂等键为 `<relationId>:<inviteeId>:inviter`，且流水、关系和双方读者精确一致。
- [ ] 对无 `register` 事实的精确匹配流水补建奖励事实：金额取流水金额，时间取流水创建时间，备注取原备注或稳定缺省值，幂等键规范化为 `invite_reward:<inviteeId>:register`，`source_ref` 为 `registration-ledger:<ledgerId>`。
- [ ] 补建 SQL 不写钱包、不更新余额、不新增流水、不处理 `source_type='legacy'` 数据，也不处理被邀请人 `:invitee` 流水。
- [ ] 增加迁移后置检查：符合旧邀请人流水模式的每条流水必须有且只有一条关系、双方读者和金额一致的 `register` 事实；不一致时整体回滚。
- [ ] 测试重复阶段、关系不匹配、金额不一致和无法解释流水均阻止迁移；验证失败后迁移版本、事实、钱包和流水保持原状。
- [ ] 验证迁移前后钱包、流水、邀请关系和已有奖励事实行数不减少，补建场景只增加预期事实。
- [ ] 运行 `gofmt`（仅 Go 测试文件）、`go test ./internal/platform/migrate` 和带 `MOONBOOK_MIGRATION_TEST_ADMIN_DSN` 的真实 PostgreSQL 集成测试。
- [ ] 检查暂存区及 `git diff --cached --check`，提交：`补齐邀请奖励事实约束与历史补偿`。

### 任务二：建立 Commerce 邀请奖励 Reader 读模型

涉及文件：

- Modify: `server/internal/modules/commerce/contract/reward.go`
- Modify: `server/internal/modules/commerce/provider/account_summary.go`
- Modify: `server/internal/modules/commerce/provider/account_summary_integration_test.go`
- Modify: `server/internal/modules/commerce/invitereward/repository.go`
- Modify: `server/internal/modules/commerce/invitereward/repository_integration_test.go`

实施步骤：

- [ ] 先扩展 `account_summary_integration_test.go`，建立两个读者、超过 20 条混合阶段/状态/角色的奖励事实，断言当前实现无法返回正确规则、累计和列表。
- [ ] 在 `commerce/contract` 增加 Reader 专用奖励记录结构，字段只含 Go 内部 `int64` ID/金额、阶段、可空发放时间和备注；在 `InviteRewardSummary` 增加非空记录切片。
- [ ] 将首充邀请奖励 `100` 收敛为 Commerce 合同层唯一政策常量，首充写入和摘要读取共同引用，删除重复字面值。
- [ ] `AccountSummary.InviteRewardSummary` 从启用配置读取 `inviter_reward_coin`，禁用时返回 0；不得读取 `invitee_reward_coin` 作为邀请人规则。
- [ ] 从 `reader_invite_reward_records` 汇总当前邀请人全部 `granted` 奖励金额，不读取 `reader_wallet_ledgers`。
- [ ] 查询当前邀请人最近 20 条 `granted` 事实，按 `granted_at DESC NULLS LAST, id DESC` 稳定排序；初始化空切片，数据库无记录时不得返回 `nil`。
- [ ] 覆盖 `failed`、`skipped`、当前读者作为被邀请人、20 条以外仍计入累计、同时间按 ID 排序、空发放时间和超过 JavaScript 安全整数边界的 ID/金额。
- [ ] 确认 Provider 运行时 SQL 只读取 Commerce 所有表，不读取 Reader 邀请关系或账号表，并通过模块 SQL 所有权静态门。
- [ ] 运行 `gofmt`、Commerce Provider/InviteReward 定向单元与真实 PostgreSQL 集成测试、`go vet`。
- [ ] 检查暂存区及 `git diff --cached --check`，提交：`修复邀请奖励汇总查询`。

### 任务三：补齐注册奖励事实与钱包事务

涉及文件：

- Modify: `server/internal/modules/commerce/provider/registration_reward.go`
- Modify: `server/internal/modules/commerce/provider/registration_reward_integration_test.go`
- Modify: `server/internal/modules/reader/invite/service_integration_test.go`

实施步骤：

- [ ] 先扩展真实 PostgreSQL 测试，要求注册成功同时产生一条 `register` 奖励事实和一条邀请人奖励流水；确认当前只写流水的实现失败。
- [ ] 在现有 Reader 注册事务和 advisory lock 内，根据同一配置快照处理邀请人奖励和被邀请人赠币。
- [ ] 当配置启用且 `inviter_reward_coin > 0` 时，以 `invite_reward:<inviteeId>:register` 写入 `reader_invite_reward_records`，记录关系、双方读者、金额、`granted` 状态、发放时间和“邀请注册奖励”备注。
- [ ] 邀请人钱包流水使用奖励事实 ID 作为业务 ID，`biz_type='invite_register_reward'`，并复用规范奖励幂等键。
- [ ] 若奖励事实已存在，读取并严格验证关系、双方读者、阶段、金额和状态；完全一致时通过钱包幂等继续，任一字段不一致时返回稳定错误，不覆盖事实。
- [ ] 保留被邀请人现有赠币金额和 `biz_type='invite_reward'` 兼容行为，继续使用独立的 `<relationId>:<inviteeId>:invitee` 幂等键及“注册奖励”备注；该流水不创建邀请人奖励事实。
- [ ] 配置禁用时双方均不发放；邀请人金额为 0 时不创建 0 金额事实或邀请人流水，被邀请人金额仍按配置处理。
- [ ] 更新测试清理顺序，先清理奖励事实和流水，再清理邀请关系与账号；不得通过禁用约束或删除测试逃避引用完整性。
- [ ] 覆盖 16 路并发、重复调用、邀请人/被邀请人 ID 锁顺序、配置 0、事实冲突、邀请人钱包失败、被邀请人钱包失败和注册后续步骤失败。
- [ ] 断言任一失败不会留下账号、邀请关系、邀请码使用次数、奖励事实、流水或余额的部分结果。
- [ ] 运行 `gofmt`、RegistrationReward 与 Reader Invite 定向真实 PostgreSQL 集成测试、Commerce/Reader 相关 race 测试和 `go vet`。
- [ ] 检查暂存区及 `git diff --cached --check`，提交：`补齐邀请注册奖励事实写入`。

### 任务四：恢复 Reader 邀请奖励兼容输出

涉及文件：

- Modify: `server/internal/modules/reader/account/http.go`
- Modify: `server/internal/modules/reader/account/http_contract_test.go`
- Verify only: `reader-ui/src/api/reader.test.ts`
- Verify only: `reader-ui/src/components/me/InviteRewardDialog.test.tsx`
- Verify only: `reader-ui/src/pages/MePage.test.tsx`

实施步骤：

- [ ] 先把 Reader HTTP 合同测试中的固定 `rewards: []` 改为包含注册和首充奖励的高位 ID 夹具，并断言当前实现失败。
- [ ] 将 Commerce 返回的奖励记录映射到 `id`、`rewardStage`、`rewardCoin`、`grantTime`、`remark`；ID 和金额使用 `strconv.FormatInt`，不得经过浮点数。
- [ ] `grantTime` 有值时使用既有 Reader 日期工具，无值时输出 JSON `null`；不输出内部关系 ID、读者 ID、幂等键、状态或来源字段。
- [ ] `totalRewardCoin`、`registerRewardCoin`、`firstRechargeRewardCoin` 继续以字符串输出；空记录固定输出 `[]`。
- [ ] 保持邀请码生成事务、邀请码可用性、邀请人数查询、路由、方法、HTTP 状态、业务码和消息不变。
- [ ] 增加 Provider 错误映射和空结果合同测试，确保数据库错误不泄漏内部细节。
- [ ] 运行 Reader account 定向测试、Reader 接口合同测试和 `reader-ui` API/邀请卡/弹窗/MePage 既有测试。
- [ ] 使用冻结基线检查 `reader-ui` 业务树无差异；本任务不得修改上列 Verify only 文件。
- [ ] 检查暂存区及 `git diff --cached --check`，提交：`恢复读者邀请奖励明细`。

### 任务五：全量验证、数据核对与文档收尾

涉及文件：

- Modify: `docs/migration/mapping.md`
- Modify: `docs/progress/refactor-status.md`
- Modify: `docs/verification/m4-exit-audit.md`
- Modify: `docs/verification/final-report.md`

实施步骤：

- [ ] 使用项目专用 PostgreSQL 执行空库 0→76、75→76、重复迁移、补建成功和阻断回滚场景，记录迁移版本和行数保护证据。
- [ ] 执行 `make verify-migration`，确保迁移单元、真实 PostgreSQL、重放和完整副本合同测试通过。
- [ ] 执行 `make verify-quality`，覆盖 Go 格式、依赖校验、模块边界、SQL 所有权、`go vet` 和普通测试。
- [ ] 执行 `make verify-integration`，使用真实 PostgreSQL、Redis 和 MinIO 完成 Reader/Commerce 全量集成回归。
- [ ] 执行 `make verify-reader` 或 `scripts/verify-m3.sh --reader-only`，证明冻结 `reader-ui` 业务代码零差异、合同测试和生产构建通过。
- [ ] 在隔离 Reader 账号下生成注册和首充奖励，通过真实浏览器打开“我的 → 奖励明细”，验证两阶段明细、金额、时间、最近 20 条和空状态；清理本轮专用夹具并核对前缀/ID 范围为 0。
- [ ] 对受控旧数据副本核对源 `reader_invite_reward_record`、目标 legacy 奖励事实、运行期奖励事实、邀请人钱包流水、累计值和最近 20 条接口结果；任何行数或金额差异阻止完成。
- [ ] 不把本地夹具结果表述为截图账号或生产账号结论；生产定向只读核对和正式 `00076` 执行继续等待单独授权。
- [ ] 更新迁移映射、进度、M4 退出审计和最终报告，写明命令、日期、结果、剩余外部验收及服务重启要求。
- [ ] 执行秘密扫描、`git diff --check`，检查项目全部已跟踪和未跟踪文件，排除构建产物、日志、缓存和敏感信息。
- [ ] 按实际逻辑拆分任何额外有效修改，检查 `git diff --cached --check`，提交：`记录邀请奖励明细修复验证`。
- [ ] 最终执行 `git status`，除明确说明的不应提交文件外保持工作树干净。

## 3. 完成标准

- Reader 邀请奖励弹窗返回当前邀请人最近 20 条真实 `granted` 奖励，不再硬编码为空。
- `registerRewardCoin` 使用邀请人奖励配置；累计值来自全部邀请人奖励事实，不混入被邀请人赠币。
- 新注册奖励事实、邀请人钱包流水、余额、邀请关系和被邀请人赠币保持同一事务和业务幂等。
- `00076` 只补建确定性运行期缺口，不修改资金事实；重复、不一致或无法解释的数据阻止迁移。
- 旧 MySQL 奖励迁移、运行期奖励事实和 Reader 返回均有真实 PostgreSQL 自动核对证据。
- 冻结 `reader-ui` 业务代码无变化，Reader/Commerce、迁移、质量和构建验证全部通过。
- 全部有效改动按逻辑使用简体中文提交，仓库最终状态干净。

## 4. 服务影响

- `moonbook-server`：需要重新构建并重启，启动前必须成功执行 `00076`。
- PostgreSQL：新增邀请奖励阶段唯一索引，并只补建可由既有不可变流水精确证明的注册奖励事实；不修改余额和流水。
- `moonbook-web`：无代码变更，无需因本修复重新构建或重启。
- 冻结 `reader-ui`：无代码变更，无需因本修复重新构建或重启；部署流程若统一构建不改变其制品内容。
- Redis、MinIO 和网关：无数据结构或配置变更，无需因本修复重启。
