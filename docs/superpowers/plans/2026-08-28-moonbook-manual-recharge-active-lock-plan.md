# Moonbook 人工补单活动锁修复实施计划

## 1. 约束

- 依据 `docs/superpowers/specs/2026-08-28-moonbook-manual-recharge-active-lock-design.md` 实施。
- 不直接更新生产订单、钱包、回调或审计数据。
- 不新增数据库迁移，不修改冻结的 `reader-ui` 或管理前端。
- 保持充值请求幂等、单读者 advisory lock、钱包一次入账和 GM Pay 不重复建单边界。

## 2. 实施任务

### 任务一：人工补单释放活动锁

- [x] 扩展真实 PostgreSQL 人工补单测试，先证明成功补单后 `active_reader_id` 残留。
- [x] 在补单结算事务中将 `active_reader_id` 置空。
- [x] 验证首次补单与幂等重放都保持 `paid`、单钱包流水和空活动锁。

### 任务二：建单自愈历史终态锁

- [x] 扩展真实 PostgreSQL 充值仓储测试，构造带活动锁的 `paid` 历史订单并证明建单失败。
- [x] 在读者 advisory lock 事务内清除明确终态的遗留活动锁，再继续创建新订单。
- [x] 自愈更新带订单、读者和终态条件；未知状态继续拒绝。
- [x] 验证旧订单支付事实不变，新订单创建成功，`pending` 替换及不确定订单保护不变。

### 任务三：验证、提交与发布

- [x] 执行相关 Go 单元测试、真实 PostgreSQL 集成测试、`go vet` 和 Commerce 全包回归。
- [x] 更新 `docs/progress/refactor-status.md`，记录生产根因、实现和验证证据。
- [x] 检查全部修改、敏感信息、`git diff --check` 和暂存区，使用简体中文逻辑提交。
- [x] 通过 `scripts/deploy-production.sh` 发布固定提交，核对镜像、迁移、健康、HTTPS 和日志。

## 3. 完成标准

- 人工补单不再留下 `paid` 活动订单锁。
- 当前生产遗留锁无需直接数据修复，下一次建单可在事务内自愈并成功创建订单。
- 不重复调用 GM Pay，不重复写钱包流水，不改变已支付订单事实。
- 自动化验证、生产发布和发布后健康检查全部通过。

## 4. 服务影响

- 代码仅影响 `server`。
- 既有生产脚本会重建 `server`、`web`、`reader-ui` 和 `gateway`。
- PostgreSQL、Redis、GM Pay 和对象存储无需重启。
