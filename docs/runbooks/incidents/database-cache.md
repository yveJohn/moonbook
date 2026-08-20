# PostgreSQL 与 Redis 事件

1. 先区分 exporter 故障和依赖故障：检查 `pg_up`/`redis_up`、应用 readiness 及 exporter 日志。
2. PostgreSQL 连接高时核对活跃/空闲连接、等待事件、长事务和锁；禁止直接终止未知生产事务。
3. Redis 内存高时核对 `maxmemory`、keyspace、淘汰和阻塞客户端；Redis 不是订单、钱包或任务唯一事实源。
4. 只读监控账号异常时重新运行 `monitoring-postgres-init` 轮换密码，不授予写表或超级用户权限。
5. 恢复后确认应用 readiness、exporter 指标和业务任务消费同时正常，再关闭事件。
