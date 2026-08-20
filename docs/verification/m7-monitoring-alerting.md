# M7 生产监控与告警栈验收

检查日期：2026-08-21

## 结论

根 Compose `monitoring` profile 已在唯一隔离项目 `moonbook_monitoring_verify_20260821` 完成真实依赖验收。默认 profile 渲染不包含监控服务；启用后 8 个 Prometheus target 全部 `up=1`，21 条规则通过 `promtool`，Alertmanager 配置通过 `amtool`，Reader 停止/恢复产生且仅产生预期 firing/resolved 通知。验收结束时没有 firing 告警。

## 固定组件

| 组件 | 固定镜像摘要 |
| --- | --- |
| Prometheus 3.5.0 | `sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996` |
| Alertmanager 0.28.1 | `sha256:27c475db5fb156cab31d5c18a4251ac7ed567746a2483ff264516437a39b15ba` |
| postgres_exporter 0.17.1 | `sha256:38606faa38c54787525fb0ff2fd6b41b4cfb75d455c1df294927c5f611699b17` |
| redis_exporter 1.74.0 | `sha256:88862b6fc5004ead3495c16d5ba548b14e898fe6d22e7c5b93bf3df83823a9a5` |
| Blackbox Exporter 0.27.0 | `sha256:a50c4c0eda297baa1678cd4dc4712a67fdea713b832d43ce7fcc5f9bea05094d` |
| cAdvisor 0.52.1 | `sha256:f40e65878e25c2e78ea037f73a449527a0fb994e303dc3e34cb6b187b4b91435` |

仓库默认和 `.env.example` 同时固定 `tag@sha256`。Webhook sink 是本仓库最小 Go 镜像，默认只保存计数；生产 URL 和 Bearer Token 由运行环境注入。

## 最小权限

- Server 指标继续要求 Bearer Token；Prometheus 只从仓库外 `0600` 文件读取同值 Secret。
- 一次性初始化服务创建固定 `moonbook_monitor`，真实查询结果为 `rolsuper=false`、`rolcreaterole=false`、`rolcreatedb=false`、`rolcanlogin=true`、`pg_monitor member=true`。
- Redis exporter 使用现有 Redis 密码，不写业务数据。
- MinIO 原生指标设为 public 仅用于内部 backend 网络抓取；宿主端口仍仅绑定 `127.0.0.1`，生产不得把 9000 直接暴露公网。
- Prometheus、Alertmanager 和本地 sink 的 UI/HTTP 入口均只绑定宿主回环。

## 信号与规则

抓取目标：Server 指标、PostgreSQL、Redis、MinIO、cAdvisor，以及 Gateway/Reader/Server 三个 Blackbox 目标，共 8 条 `up=1`。

21 条规则覆盖：入口不可用、API 5xx/p95/在途请求、PostgreSQL/Redis 可用性和容量、MinIO 指标/离线盘、容器内存、支付滞留与失败、任务失败/积压/过期租约、业务指标采集失败、未解决迁移错误、异常对象、财务核对差异和运营核对过期。每条规则都有固定 `severity`、低基数 `service` 和 Runbook 路径。

Server 新增五分钟缓存的只读运营核对。它异步执行，单次最多两分钟，不阻塞 Prometheus scrape；失败保留上次值并把 collection success 设为 0。真实隔离空库结果：迁移错误 0、异常对象 0、财务差异 0、采集成功 1，且记录了最近成功时间。

## 故障注入

`scripts/verify-monitoring.sh --inject` 暂停隔离 Reader SSR。持续 15 秒后 `MoonbookEndpointDown` 进入 firing，本地 sink 计数从 0 增至 1；脚本恢复 Reader 后收到 resolved，计数同样增至 1。第二次在加入运营核对指标后重跑，累计结果为 received 4、firing 2、resolved 2。脚本异常退出时 trap 仍会恢复 Reader。

验收末态：业务、exporter、Prometheus、Alertmanager、Blackbox、cAdvisor 和 sink 容器均运行，业务容器 healthy，一次性迁移/初始化容器退出码为 0，Prometheus 当前 firing 集合为空。

## 验证命令

```bash
GOCACHE=/private/tmp/moonbook-go-build CGO_ENABLED=0 \
  go test -race ./internal/platform/metrics
GOCACHE=/private/tmp/moonbook-go-build CGO_ENABLED=0 \
  go test ./... # deploy/monitoring/webhook
scripts/verify-monitoring.sh --config /absolute/path/to/env
scripts/verify-monitoring.sh --inject /absolute/path/to/env
```

验收后已按精确 Compose 项目名执行 `down --volumes --remove-orphans`，再次检查容器、卷和网络数量均为 0。该清理未连接或修改 M6 保留源库；验收环境不包含生产凭据或外部 Webhook 地址。
