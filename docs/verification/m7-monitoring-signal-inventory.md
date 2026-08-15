# M7 监控信号与缺口清单

检查日期：2026-08-15

## 结论

Moonbook Server 已提供受 Bearer Token 保护的 Prometheus 指标和四项依赖 readiness，但仓库尚无 Prometheus 抓取、Alertmanager、可视化、基础设施 exporter 或告警规则。现状只能支持人工瞬时检查，不能满足 M7 的持续监控、告警和故障处理要求。

用户已确认后续监控栈放入根 `compose.yaml` 的可选 `monitoring` profile，通知采用 Alertmanager UI 加通用 Webhook。该选择不代表监控设计或实现已经批准；Webhook 地址和凭据不得进入 Git。

## 已有信号

| 信号 | 当前能力 | 限制 |
| --- | --- | --- |
| `moonbook_http_requests_total` | 按 method、Gin 路由模板和 HTTP status 计数；路由标签为低基数 | 只覆盖 Go Server，Reader 兼容业务大量使用 HTTP 200，不能只按 status 判断业务失败 |
| `moonbook_http_request_duration_seconds` | 按 method、路由模板记录默认 Prometheus 延迟桶 | 没有区分业务结果、依赖耗时或外部调用阶段 |
| `moonbook_http_requests_in_flight` | 当前 Go Server 在途请求数 | 没有连接池、队列等待或容量上限对照 |
| `moonbook_platform_jobs` | 按 module、job_type、status 汇总持久化任务 | 没有最老任务年龄、重试速率、处理耗时或失败原因维度 |
| `moonbook_platform_job_overdue_leases` | 统计租约已过期的 running 任务 | 只给总量，无法区分模块和任务类型 |
| `moonbook_platform_job_metrics_collection_success` | 标记任务指标 SQL 采集是否成功 | 不能替代 PostgreSQL 自身可用性和连接池监控 |
| Go/process collectors | Go runtime、进程 CPU、内存、FD 等标准指标 | 仅 Server 进程，不覆盖 Reader SSR、Nginx、数据库、Redis、MinIO 或宿主机 |
| `/health/live` | 证明 Go HTTP 进程可响应 | 不检查依赖，也没有持续采集和历史 |
| `/health/ready` | 两秒内并发检查 PostgreSQL、Redis、MinIO Bucket 和迁移版本 | 仅返回当前 ok/unavailable；没有每项延迟、错误计数、趋势或告警状态 |
| Compose healthcheck | 覆盖 PostgreSQL、Redis、MinIO、Server、Web、Reader SSR 和 Gateway | Docker health 状态当前没有进入 Prometheus/Alertmanager |

`/metrics` 只在指标配置启用时注册；路由已注册但 Token 为空时 handler 返回 503。Server 在 Compose 中只 `expose` 到内部 backend 网络，但 Gateway 的 `/api/`、`/dev-api/` 和 `/prod-api/` 会转发任意 Server 路径，因此部署设计还需验证外部入口不会成为指标抓取主路径，并保留 Bearer 保护。

## 缺失的持续监控面

| 目标 | 当前缺口 |
| --- | --- |
| PostgreSQL | 无 exporter；无连接使用率、等待、锁、事务、慢查询、数据库/表容量和复制状态指标 |
| Redis | 无 exporter；无内存、keyspace、命中率、淘汰、阻塞客户端、持久化和连接指标 |
| MinIO | 未配置 Prometheus 抓取；无请求错误、延迟、容量、对象数、磁盘水位和离线盘指标 |
| Gateway/Web/Reader SSR | 无 Nginx、静态管理前端可用性、SSR 请求、Node 进程或外部黑盒探测指标 |
| 容器与主机 | 无 CPU、内存、磁盘、网络、重启次数、OOM 和卷水位的持续指标 |
| 业务可靠性 | 无支付创建/回调异常、对象缺失/哈希错误、钱包核对差异、迁移中断和备份新鲜度指标 |
| Worker | 无队列最老年龄、任务耗时、尝试次数、吞吐、最终失败和恢复耗时指标 |
| 日志关联 | 已有 request/trace/task 标识，但没有日志聚合或从告警跳转到对应日志证据的路径 |

## 已确认的实现边界

- 监控是根 Compose 的可选 `monitoring` profile，默认业务栈不强制启动可视化组件；
- 首期通知出口为 Alertmanager UI 与通用 Webhook，不配置 SMTP；
- 所有监控组件保持 Compose 可运行，并避免依赖宿主机固定业务路径，为后续 k3s 迁移保留 ConfigMap/Secret/PVC 对应边界；
- Prometheus 抓取凭据、Webhook URL/Token 和管理密码只能由环境变量或 Secret 注入；
- 监控不得直接修改业务数据，也不得把 Redis 当作告警或任务状态的唯一事实源。

## 设计前仍需确定

后续专项设计仍需逐段确认组件范围、保留期限、资源限制、抓取身份、指标补充、告警分级和阈值、Webhook 失败策略、可视化范围、Runbook 对应关系以及本地验收方法。在设计获用户批准前，不新增 Compose 服务、exporter、规则或业务指标。
