# Reader 充值金额四位小数展示实施计划

## 1. 约束

- 依据 `docs/superpowers/specs/2026-08-28-reader-recharge-four-decimal-display-design.md` 实施。
- 只修改 Reader 展示，不修改支付请求、回调、后端 DTO 或数据库金额。
- 金额全程按字符串处理，不使用 JavaScript `Number`。

## 2. 实施任务

### 任务一：金额字符串格式化

- [x] 新增 USDT 四位小数格式化工具及单元测试。
- [x] 覆盖整数、两位、四位、历史零尾八位和异常高精度值。

### 任务二：充值订单页统一展示

- [x] 顶部“需要支付”继续优先读取 `actualAmount`，并格式化为四位小数。
- [x] 明细“基础定价”使用同一格式化逻辑。
- [x] 扩展组件测试，锁定两处四位展示。

### 任务三：验证与发布

- [x] 执行 Reader 定向测试、完整测试和生产构建。
- [x] 更新 `docs/progress/refactor-status.md` 并使用简体中文提交。
- [ ] 通过现有生产脚本发布，核对镜像、迁移、健康、HTTPS 和日志。

## 3. 服务影响

- 代码仅影响 `reader-ui`。
- 生产脚本会重建 `server`、`web`、`reader-ui` 和 `gateway`。
- PostgreSQL、Redis、MinIO 和 GM Pay 无需重启。
