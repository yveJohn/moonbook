# 应用不可用与延迟事件

1. 在 Prometheus 确认失败实例和首次时间，检查 Alertmanager 是否正在抑制或静默。
2. 依次检查 Gateway、Reader SSR、Server health 和 Compose 容器状态；不要因单个探针失败直接重启全部服务。
3. 对 Server 核对 5xx、p95、在途请求、Go 内存和依赖 readiness；对 Reader 核对 `/health`、Node 进程和上游 Server。
4. 若最近发布后出现，停止继续发布并按升级 Runbook 评估应用镜像回退；数据库迁移不得向下回退。
5. 恢复后连续观察至少 15 分钟，确认黑盒、错误率、延迟和资源告警均 resolved。
