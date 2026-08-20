# M7 CI 与统一验收覆盖审计

检查日期：2026-08-21

## 结论

Moonbook 已建立 M0-M7 统一自动化入口。根 `make verify` 默认按 quality、management、reader、integration、migration、e2e、compose、monitoring、security 九阶段执行；任一阶段失败时仍生成 JSON 摘要，并以非零状态结束。GitHub Actions 使用九个同名独立 job，最终 `required` job 显式要求全部为 `success`，没有 `continue-on-error`。

九个阶段已在当前 Task 10 候选工作树本地逐项通过。由于本轮未执行 `git push`，远程 GitHub Actions 尚未运行；工作流只完成 YAML 解析、Shell 语法、Make 目标和本地等价入口验证。本机没有 `actionlint`，不得将本报告解释为 actionlint 已通过。固定候选 commit 的全量重跑和证据索引归 Task 13。

## 统一入口

| 命令 | 阶段 | 主要覆盖 |
| --- | --- | --- |
| `make verify-quality` | quality | gofmt、模块校验、vet、模块边界、后端测试；Linux CI 强制 race，macOS 因 `go-m1cpu` CGO/M1 探测崩溃使用 `CGO_ENABLED=0` |
| `make verify-management` | management | 冻结锁安装、管理端测试、ESLint、生产构建、供应链产物检查 |
| `make verify-reader` | reader | 冻结 Reader 源码差异、536 项基线、树外 SSR/SEO 契约、生产构建 |
| `make verify-integration` | integration | 真实 PostgreSQL、Redis、MinIO 下的 Reader/Commerce 集成测试 |
| `make verify-migration` | migration | 迁移/legacy 单元与 PostgreSQL 集成、指定版本重放、M6 全量复制脚本合同 |
| `make verify-e2e` | e2e | 隔离管理员登录、动态菜单、平台任务、刷新、移动端溢出、console/page error、安全响应头 |
| `make verify-compose` | compose | 空库迁移、完整基础栈、路由、CORS、指标鉴权、管理员引导、优雅停机、Secret 扫描 |
| `make verify-monitoring` | monitoring | Compose 监控模型、Prometheus 配置与 21 条规则、Alertmanager 配置 |
| `make verify-security` | security | Secret、安全行为、三镜像、锁文件、SBOM、High/Critical 漏洞和网关配置 |

`scripts/verify-all.sh` 支持 `MOONBOOK_VERIFY_STAGES` 选择阶段。每次运行创建唯一输出目录，保存 `tools.json`、每阶段 JSON 和总 `summary.json`。摘要记录 commit、UTC 开始/结束时间、耗时、退出状态和结果；安全扫描和 E2E 不把凭据写入摘要，Playwright 源码回显经过用户名和密码脱敏。

## CI 结构

`.github/workflows/ci.yml` 的普通 push/PR 工作流包含：

| Job | 隔离与失败语义 |
| --- | --- |
| `quality` | Ubuntu 24.04，Go 版本来自 `server/go.mod`，执行 Linux race |
| `management` | 固定 Node 22.23.2、pnpm 10.15.1 和冻结锁 |
| `reader` | 固定 Node、npm 缓存和冻结 Reader 锁 |
| `integration` | 独立 PostgreSQL/Redis/MinIO 容器，显式最终 readiness 断言后迁移和测试 |
| `migration` | 独立 PostgreSQL 管理库，测试自行创建和强制删除唯一临时数据库 |
| `e2e` | 唯一 Compose 项目，隔离管理员，运行后 `always()` 删除容器和卷 |
| `compose` | `verify-m1.sh` 自建唯一空环境并由 trap 清理 |
| `monitoring` | Runner 临时 env/Token，只执行配置门，不保存 Secret |
| `security` | 构建 Server/Web/Reader 最终镜像并上传摘要、SBOM 与扫描结果 |

所有阶段都用 `if: always()` 上传结构化 artifact；上传步骤不会改变前一验证步骤的退出状态。最终 `required` job 对九个 `needs.*.result` 逐项比较 `success`，取消、跳过或失败都会阻止最终门通过。

## 本地验证结果

2026-08-21 在 macOS arm64 当前候选工作树执行结果：

| 阶段 | 结果 | 摘要耗时 |
| --- | --- | ---: |
| quality | 通过 | 3 秒（缓存命中；另确认 macOS race 在第三方 `go-m1cpu` 初始化前崩溃，已限定为 Linux CI 门） |
| management | 通过 | 9 秒 |
| reader | 通过 | 12 秒 |
| integration | 通过 | 28 秒 |
| migration | 通过 | 8 秒 |
| e2e | 通过 | 8 秒 |
| compose | 通过 | 37 秒 |
| monitoring | 通过 | 1 秒 |
| security | 通过 | 31 秒 |

本轮门禁实际发现并修复了三类验收可靠性问题：模拟充值集成夹具未清理导致全库对账污染；历史迁移重放测试把 `Up()` 误当作指定版本重放；E2E 的 CORS Origin、验证码、Hash 路由和折叠菜单假设与真实 UI 不一致。修复后对应完整阶段均重跑通过。

静态验证包括：全部新增/修改 Shell 的 `bash -n`、CI YAML 解析、`make -n` 九阶段展开和 `git diff --check`。`shellcheck`、`actionlint` 和远程 GitHub Actions 本轮未执行。

## 普通 PR CI 的边界

以下高成本或外部动作不放入普通 PR CI：

- 真实 8 GB 数据副本的 MySQL 到 PostgreSQL/MinIO 全量迁移；普通 CI 只执行迁移集成和全量复制脚本合同；
- 生产/真实支付、AI、论坛、代理、SMTP 验收；普通 CI 使用本地替身、配置合同和安全行为测试；
- 生产 TLS/HSTS、DNS、反向代理和流量切换；
- 生产数据备份恢复、升级回退和正式 12 小时停机演练。

这些边界不允许静默跳过：Task 11 的 Runbook、Task 13 的固定提交最终报告和外部验收清单必须记录输入、命令、通过标准、风险和证据位置。真实副本、商业授权或生产权限缺失继续作为外部 Go/No-Go 项，而不是伪造为 CI 成功。

## 后续固定证据

Task 13 应在所有 R1-R4 修改提交且工作区干净后执行 `make verify`，保存统一输出目录，并在 `docs/verification/artifacts/README.md` 索引 commit、工具版本、九阶段摘要、完整副本报告、财务/对象核对和外部项。只有该固定候选提交的全部必需门通过，才能在最终报告中声明本地生产切换就绪。
