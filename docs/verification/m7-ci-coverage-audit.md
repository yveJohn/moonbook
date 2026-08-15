# M7 CI 与统一验收覆盖审计

检查日期：2026-08-15

## 结论

当前 GitHub Actions 不能作为 Moonbook 生产切换就绪门。它覆盖 Go 基础质量、管理前端测试/构建、冻结 Reader 测试/构建、Gitleaks 和 M1 空环境 Compose，但没有把 M3-M7 的真实依赖、迁移、浏览器、性能、安全和恢复证据统一固定到同一 commit。

当前流水线还存在两个确定阻断：`TestModuleDependencyRules` 会因 15 处跨模块实现引用而失败；管理前端构建会执行已确认恶意的 `vite-vue-path-map@1.0.2`。在模块边界和供应链整改完成前，不能把现有 CI 视为可通过或可信任的发布入口。

## 当前自动覆盖

| Job/步骤 | 自动执行内容 | 当前结论 |
| --- | --- | --- |
| `quality / Verify Go formatting and modules` | Moonbook 目录 gofmt、`go mod verify`、指定后端包 `go vet` | 固定提交 `c88052f` 本地同范围通过 |
| `quality / Test Moonbook foundation` | 指定后端包 `go test -race` | 模块边界测试确定失败；本机只完成非 race 普通测试基线，Linux race 尚无通过证据 |
| `quality / Build management frontend` | `pnpm install --frozen-lockfile`、`test:moonbook`、生产构建 | 锁文件确定；但生产构建会执行 critical 恶意包，当前为安全 No-Go；未运行 ESLint |
| `quality / Test and build frozen reader frontend` | `npm ci`、Vitest、生产构建 | 覆盖冻结 Reader 536 个基线用例和构建；不覆盖树外契约、真实 SSR/浏览器或依赖漏洞门 |
| `quality / Scan repository secrets` | 固定 Gitleaks 镜像扫描仓库 | 已有精确误报指纹门；不等于运行日志、镜像层或外部 Secret 配置审计 |
| `m1-compose` | `scripts/verify-m1.sh` | 覆盖空库迁移、完整基础栈、健康、路由、指标鉴权、管理员引导、优雅停机和秘密扫描 |

工作流使用锁文件安装，GitHub job 均有 30 分钟超时，权限为 `contents: read`。当前没有上传测试报告、日志摘要、SBOM、镜像摘要或验收清单的 artifact 步骤。

## 未进入 CI 的已有入口

| 入口 | 已有能力 | CI 缺口 |
| --- | --- | --- |
| `make verify-m3` / `scripts/verify-m3.sh` | Reader 源树差异、Go Reader/Commerce 测试、真实 PostgreSQL/Redis/MinIO 集成、冻结 Reader 测试、树外契约和生产构建 | 未被任何 job 调用；浏览器关键旅程仍是独立运行证据 |
| `scripts/test-reader-integration.sh` | 带 integration tag 的 Reader/Commerce/Novel 真实依赖测试 | 只被 `verify-m3` 间接调用，CI 不执行 |
| `moonbook-legacy-migrate` 及迁移集成测试 | 只读 MySQL 到 PostgreSQL/MinIO 的分域 stage、checkpoint 和幂等证据 | CI 没有 MySQL 服务、全 stage 编排、完整对象转换或差异报告 |
| `moonbook-finance-reconcile` | 当前钱包与不可变流水核对 | CI 不执行；订单、支付和权益全域核对器尚未完成 |
| 备份/恢复与 Compose 升级 Runbook | 已有隔离合成演练报告 | 没有可重复 CI job，也没有固定制品和真实数据容量证据 |
| 本地性能基线 | 四个公开读取入口短时基线 | 没有版本化负载脚本、资源限额、阈值或 CI 回归门 |

## 完全缺失的统一门

- 管理功能矩阵与 API/菜单/Casbin 对应关系的机器可读审计；
- M4 支付状态机、并发回调、财务全域核对和真实替身验收；
- M5 全部 Worker 恢复、第三方替身和内容生产迁移的统一入口；
- M6 完整副本、所有 stage、MinIO 对象、关系、财务、错误清单和 12 小时窗口验收；
- 管理端与 Reader 浏览器 E2E、SSR 故障和核心鉴权旅程；
- 依赖/镜像漏洞、恶意产物特征、SBOM、许可证和镜像 digest 门；
- 监控规则语法、告警触发/恢复、Webhook 替身和故障 Runbook 演练；
- 备份新鲜度、恢复、升级、回退和 Go/No-Go 报告生成；
- 固定验收 commit、环境/工具版本和全部证据 artifact 的统一索引。

## 根验证入口

根 `make verify` 当前只依赖 `verify-m1`，名称容易被误解为全项目验收。M7 关闭前需要一个明确的最终入口，按可并行且有独立超时的阶段组织 M0-M7 自动化证据；耗时或需要受控副本的演练可以由显式 job/命令触发，但必须固定同一 commit、输入摘要、结果和制品位置，不能用文档说明代替执行。

## 关闭条件

1. 先关闭恶意构建依赖和模块边界失败，保证基础质量门本身可信且可通过；
2. CI 执行所有适合自动化的单元、race、真实依赖、契约、SSR、管理构建和 Reader 构建检查；
3. 为 MySQL/PostgreSQL/Redis/MinIO 集成、浏览器 E2E、安全扫描和镜像构建拆分明确 job 与超时；
4. 受控完整副本、备份恢复、性能和切换演练以同一 commit 的人工触发工作流或可复现本地命令生成版本化报告；
5. 所有 job 保存结构化摘要、工具版本、镜像 digest、关键日志和失败制品，并由 `final-report.md` 精确索引；
6. 任一必需门失败时最终验收失败，不使用 `continue-on-error`、跳过或缩小测试范围掩盖缺口。

本文只记录覆盖事实和关闭条件，不批准 CI 架构或实现计划。
