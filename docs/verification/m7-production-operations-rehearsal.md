# M7 生产运维合同与维护模式演练

演练日期：2026-08-21。范围仅为本机隔离 Compose、配置解析和定向测试，未连接生产、未调用真实外部服务、未执行 DNS/TLS 或支付变更。

## 配置与代码验证

- `CGO_ENABLED=0 go test ./internal/platform/runtimeconfig ./initialize` 通过；验证未配置代理时禁用转发头信任、逗号分隔 IP/CIDR 解析、空项和非法网络拒绝。macOS arm64 继续使用既有规避路径，原因是第三方 `go-m1cpu` CGO 初始化 SIGSEGV。
- 基础 `docker compose --env-file .env.example config --quiet` 通过。
- 生产 override 经固定格式的伪 digest 解析，`scripts/verify-production-config.mjs` 确认 9 个核心服务无本地 build、镜像均为 `@sha256:`、常驻服务有 CPU/内存/PID 限制、日志轮转完整、宿主端口只绑定回环且可信代理非空。
- 正常 Gateway 在完整 Compose 网络中启动并 healthy；维护 Gateway 独立 `nginx -t` 通过。Gateway 覆盖客户端 `X-Forwarded-For`，仅接受外层代理清洗后的 `https` 协议标记；Server 只信任显式 Gateway 网段。

## 隔离维护演练

执行 `scripts/verify-maintenance-mode.sh`，隔离项目为 `moonbook_verify_maintenance`，宿主端口动态选择。首次使用固定端口时发现本机端口冲突，脚本随后改为动态空闲端口并重跑通过；失败尝试和成功尝试都由 trap 清理专用容器、网络和卷。

通过结果：

1. 全新 PostgreSQL、Redis、MinIO、迁移、Server、Web、Reader 和 Gateway 启动健康。
2. 维护脚本先仅重建 Gateway；管理首页、Reader 首页和 API readiness 均返回 503 与 `Retry-After: 300`，`/gateway-health` 保持 200。
3. Gateway 验收后才优雅停止 Server、Reader、Web；PostgreSQL readiness、Redis 认证 PONG、MinIO readiness 均继续通过。
4. 退出维护时先恢复应用，再重建正常 Gateway；管理 API readiness 和 Reader health 通过，状态从 `maintenance` 回到 `active`。
5. 最终退出码为 0，隔离资源全部清理。

## 结论与剩余外部项

本次关闭维护入口不可执行、生产 Compose 无资源/日志合同、默认信任所有代理和停写顺序无自动验收的问题。生产仍为 No-Go：8 GB 副本只实测了 64 秒源恢复，业务迁移/核对被 16 个 TXT 原文件清单阻断；真实 TLS、DNS、EPUSDT、AI、论坛/代理、SMTP、商业授权和生产容量必须按 `external-validation.md` 单独授权验收。
