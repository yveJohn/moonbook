# Moonbook 重构最终验证报告

本文件是 M7 证据索引模板。只有每项都有可定位命令输出、测试报告、迁移报告或运行时证据时，才能将对应状态标记为通过；不能用意图或空白占位替代证据。

## 版本与边界

- 新仓库 commit：待最终验收记录
- Gin-Vue-Admin 基线：见 `docs/baselines/gin-vue-admin.md`
- 旧仓库冻结基线：见 `docs/baselines/legacy-moonbook.md`
- Reader 源/目标差异：Git tree `ffbe7bb4c56792e94e160de86cc71bce0a6e0e5e`，`make verify-m3` 自动校验工作树无差异

## 证据索引

| 领域 | 证据位置 | 状态 |
| --- | --- | --- |
| M0 基线与功能矩阵 | `docs/baselines/`、`docs/inventory/admin-feature-matrix.md` | 待最终审计 |
| M1 Compose/迁移/健康检查/CI | `scripts/verify-m1.sh`、CI 运行记录 | 待最终审计 |
| M2 小说与 MinIO | `docs/progress/refactor-status.md`、对象集成测试 | 待最终审计 |
| M3 Reader 契约、SSR、浏览器 | `make verify-m3`、`docs/contracts/reader-api.md`、`docs/migration/m3-reader-commerce.md` | 通过：逐接口矩阵、真实依赖、536 个冻结用例、6 个树外 SSR/SEO 用例、生产构建和浏览器关键旅程均有证据；8GB 副本演练归属 M6 |
| M4 财务核对与支付 | `moonbook-finance-reconcile`、`docs/verification/m4-exit-audit.md` | 实施中：EPUSDT 创建/失败/并发/过期/历史凭据回调及每次回调尝试审计、两个运营页签和 Full 财务核对器已通过本地证据；真实最小金额支付未执行 |
| M5 长任务恢复与第三方替身 | 导入 Worker、TXT/AI/采集测试 | 未完成 |
| M6 全量迁移演练 | `docs/migration/`、副本演练报告 | 未完成 |
| M7 备份、切换、回退 | 本目录及 `docs/runbooks/` | 未完成；管理端三个恶意/破坏性构建依赖已关闭，但其他安全、完整浏览器、恢复和切换门仍未关闭 |

首次 M7 证据审计及当前不可执行项见 `docs/verification/m7-readiness-audit.md`。该审计用于阻止过早标记通过，不替代修复后的恢复、性能、安全和切换演练报告。

## 管理端供应链专项证据

固定代码提交 `8fd98ac525664ae9e5a45597ab82018cd34d3ae7` 已删除 `vite-vue-path-map@1.0.2`、`vite-auto-import-svg@2.9.8` 和 `vite-check-multiple-dom@0.2.1`，并以仓库内可审计实现替代路径映射和 SVG sprite。管理端 CI 等价入口 28 个测试、ESLint、生产构建和供应链检查通过；最终容器产物清单 SHA-256 为 `23833749b4a73ad6f704a6416dd41e2c85a2fa9bd93b1783201d7ea3c8801b61`，镜像 ID 为 `sha256:ec3121befb42366fac6fe1b704dfe1cf88de574ecd2ca91ccb9f72c9e91f5656`。

该项只证明三个确定的构建注入/破坏入口已关闭。Trivy 对最终 Web 镜像仍报告 Alpine `3.21.5` 有 30 high、2 critical；管理端其余依赖、Server/Reader 漏洞、SBOM、许可证和 GVA 商业授权仍是 No-Go。本轮当前代码基线的管理自动门增至 29 项并通过；Playwright 使用隔离管理员补充了登录后动态菜单及回调日志桌面/移动验收，但其他 Moonbook 管理旅程、刷新和 keep-alive 仍需最终全量证据。详见 `docs/verification/m7-security-supply-chain-audit.md` 和 `docs/verification/m7-ci-coverage-audit.md`。

## M4 EPUSDT 创建专项证据

固定实现基线 `eafe9de8ea309fd0144c60085584bff7651b871d` 已通过 `00060` 空库/升级/重跑与数据保护验证、Commerce/Reader 真实依赖回归、16 路并发创建、失败分类、活动订单替换、过期释放、历史凭据和晚到回调、防重放、冻结 Reader 全门、Compose/配置/Shell/秘密扫描及隔离 HTTP 替身冒烟。替身订单创建、Reader 查询和管理订单数据源可见性一致，临时业务数据与凭据已清理。

本证据没有执行真实 EPUSDT 付款。签到/邀请奖励运营页签及订单/支付/会员/权益全域核对仍为 M4 No-Go 项。

## M4 EPUSDT 回调审计专项证据

固定代码基线 `2658a44b` 已让每次 EPUSDT HTTP 回调在资金处理前建立独立脱敏审计，并覆盖成功、合法幂等、格式/签名拒绝、未知订单、快照不匹配、跨订单重放、依赖失败和 panic。隔离替身第一次失败后重投，2 次尝试形成 1 条 `failed` 和 1 条 `success` 审计，订单、钱包和首充奖励只存在一套事实；审计持久化失败统一返回 `503 fail`，Server 不实现第二套内部重试队列。

迁移 `00060 -> 00061`、空库 61 个迁移、幂等重跑和财务事实保护，真实 PostgreSQL/Redis/MinIO、Commerce/Reader 全包、官方 Go 1.24.2 容器 race、冻结 Reader 536 项、树外 SSR/SEO 6 项、Reader 生产构建、管理端 29 项测试/ESLint/构建/供应链门均通过。Playwright 在 `1440x1000` 与 `390x844` 验证回调列表、滞留状态、Long ID 字符串、脱敏详情和无危险操作，全部临时数据库记录、管理员及 2 个 MinIO 对象已清理。

本专项没有连接生产 EPUSDT 或执行真实付款，也不替代两个运营审计页签和财务全域核对器。M4 继续为实施中。

## 生产动作

尚未执行生产迁移、生产切换、DNS/反向代理变更、真实支付操作或 `git push`。这些动作必须在本报告全部证据通过并由用户单独授权后执行。
