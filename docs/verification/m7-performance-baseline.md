# M7 本地性能基线报告

测试时间：2026-08-15 01:28-01:33 UTC

## 结论

本次测试在本机 ARM64 Docker 的隔离空库环境中建立了四个公开读取入口的短时性能基线。所有纳入报告的 ApacheBench 请求均成功，测试后完整应用栈仍为 healthy，API readiness 的 migrations、MinIO、PostgreSQL 和 Redis 均为 `ok`。

这不是生产容量证明，也不能关闭 M7 性能退出门。环境没有业务数据、没有配置 CPU/内存限制，测试没有覆盖鉴权旅程、非空书库、MinIO 章节正文、支付回调、Worker 吞吐、持续压测或约 8 GB 数据副本。

## 环境

- 验证提交：`5ff8b8a337cdd0d38525f47f24d1046d67f8851a`
- 隔离 Compose 项目：`moonbook_verify_performance`
- 主机：Darwin ARM64；受限环境未取得硬件 CPU/内存规格
- 容器运行时：Docker `29.4.0`，Linux ARM64
- 压测工具：ApacheBench `2.3`
- 数据状态：全新 PostgreSQL、Redis 和 MinIO 命名卷，数据库为空
- 资源限制：Compose 未配置 CPU 或内存限制
- 入口：管理网关 `127.0.0.1:48080`，Reader 网关 `127.0.0.1:48081`

本次项目使用独立高位端口和命名卷，没有修改日常 `moonbook` 项目，也没有连接生产资源。

## 镜像

| 服务 | 镜像 | Image ID |
| --- | --- | --- |
| Server | `moonbook/server:local` | `c94e0a20b692` |
| 管理端 | `moonbook/web:local` | `9622ab4d21b8` |
| Reader | `moonbook/reader-ui:local` | `56409bf67f9c` |
| PostgreSQL | `postgres:17.6-alpine` | `29e645e20571` |
| Redis | `redis:7.4.5-alpine` | `ff193672aa2e` |
| MinIO | `minio/minio:RELEASE.2025-07-23T15-54-02Z` | `b9292014cabd` |
| Nginx | `nginx:1.28.0-alpine3.21` | `5a91d90f47dd` |

## 场景与结果

每个场景仅执行一次纳入报告的短时固定请求量测试，延迟分位数取 ApacheBench 输出。它们适合后续提交在相同环境中做回归比较，不应外推为持续负载上限。

| 场景 | 请求数 / 并发 | RPS | p50 | p95 | p99 | 最大值 | 失败 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 网关 `/gateway-health` | 10000 / 100 | 9859.95 | 1 ms | 2 ms | 15 ms | 1011 ms | 0 |
| API `/api/health/ready` | 2000 / 20 | 3924.92 | 4 ms | 11 ms | 22 ms | 58 ms | 0 |
| Reader `/prod-api/reader/books?pageNum=1&pageSize=20` | 5000 / 50 | 7984.91 | 3 ms | 24 ms | 36 ms | 65 ms | 0 |
| Reader SSR `/` | 1000 / 20 | 546.54 | 34 ms | 49 ms | 65 ms | 88 ms | 0 |

Reader 书籍列表首次测试把查询分隔符 `&` 错误编码成 `%26`，该轮结果已舍弃。表中只记录使用独立 `pageNum=1` 和 `pageSize=20` 查询参数的修正结果。Prometheus 累计值包含这轮被舍弃的 5000 次请求，因此不能用 `/reader/books` 的累计值反推表中请求量。

## 应用指标与健康

负载结束后抓取受保护的 Prometheus 指标：

- `/health/ready`：2032 次，全部为 HTTP 200；
- `/reader/books`：10001 次，全部为 HTTP 200，其中包含被舍弃的错误查询轮次；
- Reader SSR 触发 `/reader/books/categories`、`/reader/books/featured`、`/reader/books/sub-categories` 和 `/reader/seo/config` 各 1001 次，全部为 HTTP 200；
- `moonbook_platform_job_metrics_collection_success 1`；
- `moonbook_platform_job_overdue_leases 0`。

负载后再次请求 `/api/health/ready` 返回 200，migrations、MinIO、PostgreSQL 和 Redis 全部为 `ok`；所有 Compose 容器均保持 healthy。

## 资源快照

以下是负载结束后 `docker stats --no-stream` 的一次性内存快照，不是测试期间峰值，也不包含 CPU 峰值、磁盘 I/O 或网络带宽：

| 服务 | 内存使用 |
| --- | ---: |
| Nginx 网关 | 12.94 MiB |
| Go Server | 119.8 MiB |
| Reader UI | 84.25 MiB |
| PostgreSQL | 55.98 MiB |
| Redis | 12.04 MiB |
| MinIO | 433.6 MiB |

## 未覆盖项与后续门槛

本基线没有覆盖以下生产关键路径：

- 管理员登录、RBAC 和带鉴权管理操作；
- 非空目录、搜索、书架、阅读历史及完整 Reader 用户旅程；
- PostgreSQL 查询真实数据和 MinIO 章节正文读取、大小与哈希校验；
- 订单创建、支付回调验签、幂等及财务核对并发；
- 论坛、TXT、清洗、摘要、画像和对象回收 Worker 吞吐及积压恢复；
- 长时间稳定性、连接池耗尽、Redis/MinIO 故障和恢复；
- 资源限额下的 CPU、内存、磁盘、网络、数据库连接和 p95/p99 峰值；
- 约 8 GB 完整数据副本迁移、对象核对、恢复与 12 小时窗口余量。

关闭 M7 性能门前，必须在固定验收提交上建立可重复场景和阈值，使用具有代表性的脱敏数据或完整副本执行持续负载与迁移容量测试，并记录峰值资源、错误率、延迟、Worker 吞吐、数据库连接、MinIO 带宽和 12 小时窗口余量。

## 清理

报告提交前已删除 `moonbook_verify_performance` 的容器、网络、专用命名卷和仓库外临时文件。日常 `moonbook` Compose 项目和冻结旧仓库未操作。
