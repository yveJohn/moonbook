# Moonbook 签到与邀请奖励运营审计页签设计

## 1. 文档状态

- 日期：2026-08-20
- 状态：设计已获用户批准，书面规格待用户复核
- 所属里程碑：M4 交易、支付与运营
- 目标：补齐签到记录与邀请奖励记录两个只读运营审计页签，不改变 Reader 业务行为或资金事实

本规格是 M4 的独立纵向切片。它只关闭 `GET /reader/checkin/list` 与 `GET /reader/inviteReward/list` 所代表的管理审计能力缺口，不包含财务全域核对器或真实 EPUSDT 最小金额支付。完成本规格后，M4 仍须单独完成其余退出项。

## 2. 事实来源与现状

冻结旧仓库的 `ReaderWalletAdminController` 定义两个只读管理入口：

- `GET /reader/checkin/list`：按读者 ID 查询签到记录，按签到日期、记录 ID 倒序；
- `GET /reader/inviteReward/list`：按邀请人 ID、被邀请人 ID和奖励阶段查询奖励记录，按发放时间、记录 ID 倒序。

旧接口返回记录 ID、读者或关系 ID、奖励数值、幂等键、业务状态和时间。旧管理前端未形成对应可见页面，但用户已明确选择在新管理端补齐这两个审计页签。

当前新系统已经具备：

- `reader_checkin_records` 签到事实及 `reader_checkin_records_reader_date_idx`；
- `reader_invite_reward_records` 邀请注册与首充奖励事实；
- Commerce 自有 `commerce_reader_search_projection` 读者展示投影；
- 签到规则、邀请奖励配置、钱包和不可变流水管理能力；
- GVA 管理响应、Casbin、Long ID 字符串工具和真实 PostgreSQL 集成测试模式。

当前缺少记录查询 API、权限注册、管理页面入口、面向大数据量的补充查询索引和浏览器验收证据。

## 3. 方案选择

采用现有“钱包与流水”页面内三页签方案：

1. 钱包概览；
2. 签到记录；
3. 邀请奖励。

该方案把余额、流水和奖励事实集中在同一运营入口，避免继续扩张侧边栏菜单。两个独立菜单会增加导航长度；从单个钱包行进入抽屉则无法自然表达同时涉及邀请人与被邀请人的全局奖励查询，因此不采用。

页面路由和菜单仍使用现有 `ReaderWallet`，不新增菜单记录。三个页签拥有独立筛选、分页和加载状态，切换页签时按需加载，切回时保留本页会话内的筛选条件。

## 4. 模块边界

签到和邀请奖励事实均归 Commerce。实现放在 Commerce 管理只读边界内，禁止读取 `reader_accounts` 或调用 Reader 内部实现。

读者账号和昵称只通过 `commerce_reader_search_projection` 补齐：

- 签到记录左连接一次投影；
- 邀请奖励分别按邀请人和被邀请人左连接两次投影；
- 投影缺失时仍返回业务事实，账号和昵称为空，由页面显示 `-`；
- 不允许用内连接或应用层过滤隐藏缺失展示投影的签到、奖励或财务事实。

仓储只执行查询，不开启写事务，不修改签到、邀请、钱包或奖励记录。服务层负责输入规范化、枚举/日期/分页校验和稳定错误映射；HTTP 层只负责绑定、输出及 GVA 管理响应。

## 5. 管理 API

### 5.1 签到记录

```text
GET /reader/checkins
```

查询参数：

- `readerKeyword`：可选，匹配精确十进制读者 ID 或投影中的账号/昵称；
- `startDate`：可选，`YYYY-MM-DD`，包含边界；
- `endDate`：可选，`YYYY-MM-DD`，包含边界；
- `page`：默认 1；
- `pageSize`：默认 20，最大 100。

每行字段：

- `id`、`readerId`：十进制字符串；
- `readerUsername`、`readerNickname`：字符串，投影缺失时为空字符串；
- `checkinDate`：`YYYY-MM-DD`；
- `continuousDays`：整数；
- `baseRewardCoin`、`milestoneRewardCoin`、`totalRewardCoin`：十进制字符串；
- `idempotencyKey`、`sourceType`：字符串；
- `createdAt`：RFC3339Nano 字符串。

默认排序固定为 `checkin_date DESC, id DESC`。

### 5.2 邀请奖励记录

```text
GET /reader/inviteRewards
```

查询参数：

- `inviterKeyword`：可选，匹配邀请人精确 ID 或账号/昵称；
- `inviteeKeyword`：可选，匹配被邀请人精确 ID 或账号/昵称；
- `rewardStage`：可选，只允许 `register`、`first_recharge`；
- `status`：可选，只允许 `granted`、`skipped`、`failed`；
- `startTime`、`endTime`：可选 RFC3339 时间，包含边界且开始不得晚于结束；
- `page`：默认 1；
- `pageSize`：默认 20，最大 100。

每行字段：

- `id`、`relationId`、`inviterReaderId`、`inviteeReaderId`：十进制字符串；
- `inviterUsername`、`inviterNickname`、`inviteeUsername`、`inviteeNickname`：字符串；
- `rewardStage`、`status`、`idempotencyKey`、`sourceType`、`remark`：字符串；
- `rewardCoin`：十进制字符串；
- `grantedAt`：未发放时为 `null`，否则为 RFC3339Nano 字符串；
- `createdAt`：RFC3339Nano 字符串。

默认排序固定为 `granted_at DESC NULLS LAST, id DESC`。

两个接口均使用当前 `managementresponse.Page` 的 `list/total/page/pageSize` 结构，不兼容旧 RuoYi `TableDataInfo` 包装。所有 ID 在 Go 内部可使用 `int64`，但 JSON 和 Vue 全程使用字符串。

## 6. 输入、错误与安全

- 关键词去除首尾空白，最大 128 个 Unicode 字符；全数字关键词按精确 ID 匹配，同时允许账号或昵称文本匹配；
- 日期、时间、枚举、页码和页大小严格校验，不接受部分解析或静默回退；
- 非法参数返回稳定的管理端参数错误；数据库错误通过既有 `apperror` 脱敏并保留 Request ID/Trace ID；
- 空结果固定返回非空 `list: []` 和 `total: 0`；
- 不返回 Reader Token、密码摘要、来源 IP、支付凭据或内部数据库错误；
- 幂等键用于受控管理查询，但不得写入日志或 Prometheus 标签；
- 页面严格只读，不提供补签、补发、删除、重算、导出、钱包跳转或其他资金操作；
- 不新增 Reader-facing 路由，不修改冻结 `reader-ui`，不改变签到和邀请奖励事务。

## 7. PostgreSQL 前向迁移

创建当前最高版本之后的 `00062_commerce_operations_audit.sql`。迁移只执行以下非破坏性操作：

1. 注册 `GET /reader/checkins` 和 `GET /reader/inviteRewards` 两个 GVA API；
2. 为超级管理员 authority `888` 增加两条 Casbin 读取权限；
3. 为全局签到日期倒序查询增加 `(checkin_date DESC, id DESC)` 索引；
4. 为邀请奖励全局时间排序增加 `(granted_at DESC NULLS LAST, id DESC)` 索引；
5. 为被邀请人查询增加 `(invitee_reader_id, granted_at DESC NULLS LAST, id DESC)` 索引；
6. 为奖励阶段查询增加 `(reward_stage, granted_at DESC NULLS LAST, id DESC)` 索引；
7. 为奖励状态查询增加 `(status, granted_at DESC NULLS LAST, id DESC)` 索引。

迁移不新增或修改业务列，不更新既有业务行，不修改、改名或删除 `00001` 至 `00061`。必须验证空库从零、`00061 -> 00062`、重复执行 `applied=0`，并证明签到、邀请奖励、钱包流水和充值订单行数不减少。

旧 MySQL 到 PostgreSQL 的 `reader-finance` stage 已迁移这两类事实，本切片不新增迁移 stage 或映射规则，只在映射与验收文档中补充可查询证据。

## 8. 管理页面

现有 `web/src/view/reader/wallet/index.vue` 改为三个顶层 `el-tabs`：钱包概览、签到记录、邀请奖励。页面区域保持单层、不在卡片内嵌套卡片。

签到页签提供读者关键词、日期范围、查询、重置和刷新；表格优先展示记录 ID、读者、日期、连续天数和总奖励，基础/里程碑奖励、幂等键、来源和创建时间作为宽屏列或详情抽屉字段。

邀请奖励页签提供邀请人、被邀请人、阶段、状态、发放时间范围、查询、重置和刷新；表格优先展示记录 ID、双方读者、阶段、奖励、状态和发放时间，关系 ID、幂等键、来源、备注和创建时间作为宽屏列或详情抽屉字段。

桌面端完整展示可扫描表格。`390x844` 移动视口隐藏次要列，保留关键事实与图标详情入口；工具栏纵向排列，分页可水平滚动但不得撑宽页面或与其他元素重叠。抽屉宽度使用视口约束，长 ID、幂等键和备注允许自然换行。

Vue API 必须使用仓库 Long ID 字符串工具或原字符串，不得使用 `Number`、`parseInt`、算术或 `el-input-number` 处理任何 ID。

## 9. 测试与验收

### 9.1 后端自动化

- 迁移静态测试：版本、API/Casbin、索引、无破坏语句；
- 真实 PostgreSQL：空库、升级、重复执行和关键事实行数保护；
- 仓储集成：全部筛选、组合筛选、稳定排序、分页、`NULL granted_at`、缺失投影仍返回事实；
- Long ID：记录、关系和双方读者 ID 均覆盖大于 `Number.MAX_SAFE_INTEGER` 的值；
- HTTP 合同：成功分页、空列表、非法 ID/枚举/日期/分页、数据库错误脱敏和权限注册；
- 模块边界与 SQL 所有权：运行时代码不读取 Reader 表。

### 9.2 管理前端自动化

- API URL、参数和 Long ID 字符串合同；
- 三页签按需加载、独立分页与筛选保留；
- 空状态、投影缺失、未发放时间和错误状态展示；
- 详情抽屉不包含写操作；
- Node 测试、ESLint、生产构建和供应链检查全部通过。

### 9.3 真实浏览器

使用项目专用 PostgreSQL、Redis 和 MinIO 及本轮隔离管理员/夹具，在 `1440x1000` 和 `390x844` 验证：

- 三页签可达且切换不丢失筛选；
- 两类记录、筛选、分页和详情正确；
- Long ID 保持字符串且无科学计数或精度变化；
- 缺失投影事实仍可见；
- 页面无补发、补签、删除、重算或其他写入口；
- 页面和抽屉无重叠、无被裁切的关键操作。

验收结束必须删除本轮专用管理员、签到记录、邀请关系/奖励、钱包流水、投影和其他夹具，核对各专用前缀或 ID 范围为 0。不得删除共享环境中的未知历史夹具。

## 10. 证据与退出边界

完成后更新：

- `docs/inventory/admin-feature-matrix.md`；
- `docs/migration/mapping.md`；
- `docs/progress/refactor-status.md`；
- `docs/verification/m4-exit-audit.md`；
- `docs/verification/final-report.md`。

只将 M4 的“签到和邀请奖励记录审计页签”标记为完成。以下项目继续为 No-Go：

- 订单、支付、钱包、奖励、会员和权益财务全域核对器；
- 受控真实 EPUSDT 连通性和最小金额支付验收；
- 任何正式生产迁移、流量切换或生产数据操作。

## 11. 服务影响

实施后影响 `moonbook-server` 和 `moonbook-web`。本地或部署环境需要先执行 `00062` 前向迁移，再替换或重启 Server 与 Web；冻结 Reader UI、Redis 和 MinIO 不因本切片单独重启或迁移。

规格文档本身不影响服务，无需重启。
