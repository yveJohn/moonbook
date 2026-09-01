# Moonbook 读者邀请奖励明细修复设计

## 1. 文档状态

- 日期：2026-09-01
- 状态：设计已获用户确认，书面规格待用户复核
- 所属里程碑：M4 交易、支付与运营兼容修复
- 目标：恢复冻结版 `reader-ui` 邀请奖励明细、累计奖励和注册奖励规则的真实语义，并补齐新系统运行期注册奖励事实缺口

本规格只修复 `POST /reader/me/invite/code` 的邀请奖励读模型、邀请注册奖励事实写入和可确定的运行期历史缺口。它不新增 Reader 路由，不修改冻结 `reader-ui` 业务代码，不改变奖励金额，不补发资金，也不执行正式生产迁移或流量切换。

## 2. 问题与证据

冻结旧系统的 `ReaderCurrencyQueryService.inviteDashboard` 具有以下明确语义：

1. `registerRewardCoin` 是邀请人因好友注册获得的奖励；
2. `totalRewardCoin` 汇总当前读者作为邀请人的全部 `granted` 奖励事实；
3. `rewards` 返回当前读者作为邀请人的最近 20 条 `granted` 奖励，按发放时间、记录 ID 倒序；
4. 注册奖励和首充奖励均先形成奖励事实，再以该事实发放钱包奖励。

当前 Go 实现存在四个相互关联的缺陷：

- Reader HTTP 将 `rewards` 固定返回空数组，数据库有记录时弹窗仍显示空状态；
- `totalRewardCoin` 从钱包流水汇总，既偏离冻结契约，也可能把当前读者作为被邀请人收到的注册赠币计入邀请收益；
- `registerRewardCoin` 错读 `invitee_reward_coin`，实际应展示 `inviter_reward_coin`；
- 运行期注册发奖只写双方钱包流水，没有写 `reader_invite_reward_records`，导致事实表、管理审计和 Reader 明细不完整。

旧 MySQL `reader_invite_reward_record` 到 PostgreSQL `reader_invite_reward_records` 的迁移映射已经存在，不能因 Reader 查询缺陷而判断旧奖励数据没有迁移。实施修复时必须分别核对源奖励事实、目标奖励事实和新系统运行期缺失事实。

## 3. 方案选择

采用“奖励事实闭环修复”：

1. Commerce 从奖励事实表提供规则、累计奖励和最近奖励；
2. Reader 兼容层只负责邀请码、邀请人数和 DTO 序列化；
3. 注册发奖在现有注册事务中同时写奖励事实与钱包流水；
4. `00076` 只补建能够从现有运行期邀请人流水精确证明的注册奖励事实；
5. 无法证明的差异阻止自动补偿，进入审计和人工处置，不推测金额或关系。

不采用仅修复空数组的方案，因为它会让新注册奖励继续缺少事实。不采用从钱包流水动态拼装明细的方案，因为钱包流水不是邀请奖励明细的权威来源，且无法稳定表达奖励阶段、双方关系和管理审计状态。

## 4. 模块边界与组件

邀请奖励配置、奖励事实、钱包及流水均归 Commerce。Reader 拥有邀请码和邀请关系。保持现有组合根注入方式：

- `commerce/contract` 定义 Reader 可读取的邀请奖励摘要和明细 DTO；
- `commerce/provider.AccountSummary` 只读取 Commerce 自有奖励配置、奖励事实和统一奖励政策；
- `reader/account` 读取 Reader 自有邀请码及邀请关系数量，并把 Commerce DTO 转为冻结 Reader 响应；
- `commerce/provider.RegistrationReward` 在调用方已有 PostgreSQL 注册事务中写奖励事实和钱包流水；
- Reader 注册服务继续提供关系 ID、邀请人 ID 和被邀请人 ID，不允许 Commerce 反向查询 Reader 内部表。

Commerce Reader 查询 Provider 不读取 `reader_accounts`、`reader_invite_relations` 或其他 Reader 表。奖励事实已经保存关系与双方读者 ID，Reader 明细只需要当前邀请人 ID 即可查询。

## 5. Reader 兼容契约

路由和方法保持不变：

```text
POST /reader/me/invite/code
```

响应包装、现有字段和缺省值保持冻结契约。与奖励相关的字段定义如下：

- `registerRewardCoin`：启用配置时取 `reader_invite_reward_config.inviter_reward_coin`，禁用时为字符串 `"0"`；
- `firstRechargeRewardCoin`：取 Commerce 唯一的首充邀请奖励政策值，当前为字符串 `"100"`；
- `totalRewardCoin`：当前读者作为邀请人、状态为 `granted` 的全部奖励事实 `reward_coin` 之和，以十进制字符串输出；
- `rewards`：同一事实范围内最近 20 条记录，固定为非 `null` 数组；
- 空结果：`totalRewardCoin` 为 `"0"`，`rewards` 为 `[]`。

每条 `rewards` 只输出冻结 Reader 已使用的字段：

- `id`：奖励记录 ID，十进制字符串；
- `rewardStage`：`register` 或 `first_recharge`；
- `rewardCoin`：十进制字符串；
- `grantTime`：有值时沿用 Reader 兼容日期格式，数据库时间为空时输出 JSON `null`，不输出空字符串或伪造时间；
- `remark`：原奖励备注，允许空字符串或 `null`，不返回幂等键、关系 ID、双方读者 ID 或内部来源字段。

列表过滤固定为：

```sql
inviter_reader_id = 当前读者
AND status = 'granted'
```

排序固定为：

```sql
granted_at DESC NULLS LAST, id DESC
LIMIT 20
```

累计值汇总全部匹配事实，不受 20 条列表上限影响。`skipped`、`failed`、当前读者作为被邀请人的奖励，以及其他读者的奖励均不得进入累计值或列表。

## 6. 注册奖励写入

### 6.1 邀请人奖励事实

当邀请奖励配置启用且 `inviter_reward_coin > 0` 时，在现有 Reader 注册事务中执行：

1. 使用现有关系维度 advisory lock 串行化同一受邀人的注册奖励；
2. 向 `reader_invite_reward_records` 写入一条 `register` 事实；
3. 事实字段固定使用调用方传入的关系 ID、邀请人 ID 和被邀请人 ID；
4. `reward_coin` 使用本次事务读取的 `inviter_reward_coin` 配置快照；
5. `status='granted'`，`granted_at=now()`，备注为“邀请注册奖励”；
6. 规范幂等键固定为 `invite_reward:<inviteeId>:register`；
7. 使用奖励事实 ID 作为邀请人钱包流水的业务 ID，写入 bonus 收入；
8. 奖励事实、邀请人钱包流水、邀请人余额、被邀请人赠币流水和注册关系必须在同一事务中提交或回滚。

新邀请人钱包流水的业务类型统一为冻结旧系统已使用且 Reader 钱包页面已识别的 `invite_register_reward`。钱包流水幂等键使用同一规范奖励键。奖励事实已存在时，只允许读取并验证同一关系、双方读者、阶段和金额；字段不一致必须报稳定错误，不能覆盖或重复发奖。

### 6.2 被邀请人注册赠币

`invitee_reward_coin` 是被邀请人自己的注册赠币，不属于邀请人奖励明细。现有赠币行为和金额保持不变，但必须使用与邀请人奖励不同的幂等键和业务语义，不能被 Reader 邀请累计查询纳入。

配置禁用或邀请人奖励为 0 时，不创建 0 金额邀请奖励事实，也不创建邀请人钱包流水。被邀请人赠币是否发放仍按同一配置快照中的 `invitee_reward_coin` 决定。

### 6.3 首充奖励

首充路径已经写入 `first_recharge` 奖励事实和邀请人钱包流水，本规格不改变其金额、触发条件和跨入口幂等逻辑。实现时只把首充奖励政策值收敛到 Commerce 唯一来源，避免查询端和写入端出现两个独立的 `100` 字面值。

## 7. 前向迁移与运行期历史修复

新增 `00076_reader_invite_reward_detail_repair.sql`，不得修改既有迁移。迁移只执行非破坏性约束加固和确定性事实补建，不更新或删除钱包、流水、邀请关系及已有奖励事实。

### 7.1 迁移前置保护

迁移先检查 `reader_invite_reward_records` 是否存在同一 `(invitee_reader_id, reward_stage)` 多条事实。发现重复时抛出明确异常并回滚，不自动选择、合并或删除记录。

检查通过后增加唯一索引：

```text
(invitee_reader_id, reward_stage)
```

该约束与冻结旧系统“一名受邀读者每个奖励阶段最多一条事实”的唯一性保持一致，并与幂等键唯一约束共同防止不同键产生重复奖励事实。

### 7.2 确定性补建条件

只补建同时满足以下全部条件的既有运行期邀请人流水：

- 流水 `reader_id` 等于邀请关系的 `inviter_reader_id`；
- `biz_type='invite_reward'`、`direction='income'`、`coin_type='bonus'`；
- 流水幂等键精确等于当前 Go 旧实现的 `<relationId>:<inviteeId>:inviter`；
- 关系 ID、邀请人 ID 和被邀请人 ID 能唯一匹配；
- 同一受邀人不存在 `register` 奖励事实；
- 流水金额大于 0，余额转换满足钱包不可变流水约束。

补建事实使用流水金额、创建时间和备注，阶段固定为 `register`，状态固定为 `granted`，规范幂等键为 `invite_reward:<inviteeId>:register`，`source_type='runtime'`，`source_ref` 固定为 `registration-ledger:<ledgerId>`。该引用只包含目标不可变流水 ID，不包含账号、Token 或其他秘密。补建不新增钱包流水、不改变余额，也不把被邀请人赠币转换为邀请人奖励。

### 7.3 迁移后置保护

补建后再次检查所有符合当前旧实现邀请人流水模式的记录是否都有唯一 `register` 事实，并核对关系、双方读者和金额一致。存在无法匹配、金额不一致或多重匹配时迁移抛出异常并整体回滚。

旧 MySQL 迁移产生的 `source_type='legacy'` 奖励事实不参与运行期反推。若正式副本核对发现旧源奖励事实、目标 legacy 奖励事实和 legacy 钱包流水不一致，必须由 `reader-finance` 迁移报告阻止上线，不得由 `00076` 猜测补齐。

### 7.4 生产执行边界

仓库内完成空库、`00075 -> 00076`、重复执行和带运行期缺口夹具的验证。部署生产前先只读执行同样的前置核对并保存聚合报告。正式执行生产迁移仍需用户单独授权，本规格和本地实现不构成生产数据操作授权。

## 8. 错误、安全与一致性

- Commerce Provider 无效读者 ID 返回既有稳定参数错误；
- 查询失败通过 Commerce 合同包装并由 Reader 兼容层返回既有错误结构，不泄漏 SQL、幂等键或内部表名；
- 明细查询始终初始化为空切片，禁止输出 `null`；
- 所有 Long ID 和奖励金额只在 Go/数据库内部使用整数，对 JavaScript 一律输出字符串；
- 读路径不因缺失投影或账号展示信息隐藏奖励事实；
- 注册发奖任一步失败必须回滚账号、邀请关系、邀请码使用次数、奖励事实、钱包流水和余额；
- 迁移日志和报告只输出计数、金额聚合及必要的 ID 指纹，不输出密码、Token、支付秘密或完整敏感正文；
- 不提供 Reader 或管理端的补发、删除、重算入口。

## 9. 测试与验收

### 9.1 Commerce 单元与真实 PostgreSQL 集成

- 配置启用、禁用以及邀请人/被邀请人不同奖励金额的读取；
- `registerRewardCoin` 读取邀请人奖励，不读取被邀请人赠币；
- 最近 20 条、全量累计、`granted_at` 与 ID 稳定排序；
- 累计包含 20 条以外记录，但排除 `failed`、`skipped` 和当前读者作为被邀请人的记录；
- 大于 `Number.MAX_SAFE_INTEGER` 的奖励 ID 和金额保持 `int64` 到字符串的精度；
- 注册成功只生成一条 `register` 事实和一条邀请人奖励流水；
- 配置为 0 时不生成 0 金额邀请人事实；
- 重复、并发注册和事务重试不重复事实、流水或余额；
- 奖励事实写入失败、钱包失败、被邀请人赠币失败和投影失败均不留下部分注册或资金事实；
- 首充奖励既有幂等、回滚和并发测试继续通过。

### 9.2 迁移测试

- 空库从零应用至 `00076`；
- `00075 -> 00076` 前向升级及重复执行 `applied=0`；
- 精确匹配的旧运行期邀请人流水补建一条事实且不改变钱包、流水或余额；
- 被邀请人赠币、legacy 流水、其他业务流水和零金额异常不被补建；
- 已有事实不重复；
- 重复阶段事实、关系不匹配、金额不一致和无法解释的邀请人流水使迁移失败并整体回滚；
- 迁移前后钱包、流水、关系和已有奖励事实行数不减少。

### 9.3 Reader 合同与前端回归

- `POST /reader/me/invite/code` 返回非空奖励明细、最近 20 条、全量累计和正确规则金额；
- 空结果仍返回字符串 `"0"` 与 `[]`；
- ID、金额、日期、备注和字段缺省值符合冻结合同；
- `reader-ui` API、邀请卡、奖励弹窗、页面旅程和全部既有测试不修改业务代码通过；
- 弹窗正确展示注册奖励与首充奖励，不展示其他读者或未发放事实；
- `go test`、真实 PostgreSQL 集成测试、`go vet`、Reader 全量测试和生产构建通过。

### 9.4 数据核对

对受控副本按邀请人逐项核对：

- 源 `reader_invite_reward_record` 与目标 `source_type='legacy'` 奖励事实的行数、主键、阶段、状态和金额；
- 目标运行期邀请人奖励事实与对应邀请人钱包收入流水；
- `totalRewardCoin` 与目标 `granted` 奖励事实汇总；
- 最近 20 条接口结果与同条件 SQL 查询；
- 邀请关系数量与 `invitedCount`。

截图账号或生产账号只有在获得生产只读权限后才能执行定向核对；本地夹具结果不能替代生产结论。

## 10. 文档、提交与退出标准

实现完成后更新：

- `docs/migration/mapping.md`；
- `docs/progress/refactor-status.md`；
- `docs/verification/m4-exit-audit.md`；
- `docs/verification/final-report.md`。

退出标准：

1. Reader 奖励明细不再硬编码为空；
2. 规则金额、累计奖励和最近 20 条均与冻结旧系统语义一致；
3. 新注册奖励具有奖励事实、钱包流水和事务原子性；
4. 可确定的旧运行期缺口被幂等补建，无法确定的差异明确阻断；
5. 旧数据迁移、运行期事实和 Reader 返回均有自动核对证据；
6. 冻结 `reader-ui` 业务代码保持不变。

## 11. 服务影响

实施后影响 `moonbook-server` 和 PostgreSQL：部署环境需先执行 `00076` 前向迁移，再替换并重启 Server。`moonbook-web`、冻结 `reader-ui`、Redis 和 MinIO 不需要因本修复修改或重启。

本设计文档本身不影响服务，无需重启。
