# Moonbook 重构最终验证报告

本文件是 M7 证据索引模板。只有每项都有可定位命令输出、测试报告、迁移报告或运行时证据时，才能将对应状态标记为通过；不能用意图或空白占位替代证据。

## 版本与边界

- 新仓库 commit：待最终验收记录
- Gin-Vue-Admin 基线：见 `docs/baselines/gin-vue-admin.md`
- 旧仓库冻结基线：见 `docs/baselines/legacy-moonbook.md`
- Reader 源/目标差异：待最终验收记录

## 证据索引

| 领域 | 证据位置 | 状态 |
| --- | --- | --- |
| M0 基线与功能矩阵 | `docs/baselines/`、`docs/inventory/admin-feature-matrix.md` | 待最终审计 |
| M1 Compose/迁移/健康检查/CI | `scripts/verify-m1.sh`、CI 运行记录 | 待最终审计 |
| M2 小说与 MinIO | `docs/progress/refactor-status.md`、对象集成测试 | 待最终审计 |
| M3 Reader 契约、SSR、浏览器 | `docs/contracts/reader-api.md`、`docs/migration/m3-reader-commerce.md`、Reader 真实依赖测试 | 部分通过：SSR 与浏览器关键旅程通过，逐接口矩阵和 8GB 副本演练待完成 |
| M4 财务核对与支付 | `moonbook-finance-reconcile`、隔离支付报告 | 未完成 |
| M5 长任务恢复与第三方替身 | 导入 Worker、TXT/AI/采集测试 | 未完成 |
| M6 全量迁移演练 | `docs/migration/`、副本演练报告 | 未完成 |
| M7 备份、切换、回退 | 本目录及 `docs/runbooks/` | 未完成 |

## 生产动作

尚未执行生产迁移、生产切换、DNS/反向代理变更、真实支付操作或 `git push`。这些动作必须在本报告全部证据通过并由用户单独授权后执行。
