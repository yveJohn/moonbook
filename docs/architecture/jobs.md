# 持久化任务状态机

`server/internal/platform/jobs` 以 PostgreSQL 的 `platform_jobs` 和 `platform_job_attempts` 为唯一任务事实。进程内 goroutine 只执行已取得租约的工作，不保存不可恢复的队列状态。

状态流转：

```text
pending --claim--> running --complete--> succeeded
                         \--retryable failure--> pending
                         \--terminal/exhausted--> failed
running --lease expired--> 可由另一 worker 重新 claim
```

- 入队使用 `(module, job_type, idempotency_key)` 唯一键；重复请求返回原任务，不覆盖 payload 或重试上限。
- 领取必须声明 module 和可处理的 job types，并使用事务、`FOR UPDATE SKIP LOCKED` 和 `available_at`，允许多 worker 并发且同一时刻只有一个租约持有者。
- 每次领取递增 `attempt_count` 并写入对应 `platform_job_attempts`。
- 完成、失败和续租必须同时匹配任务 ID、`running` 状态、worker ID 和未过期租约；旧 worker 返回 `ErrLeaseLost`，不得覆盖新租约结果。
- 可重试失败按 1 秒起步的指数退避重新进入 `pending`，最高 15 分钟；达到 `max_attempts` 后进入 `failed`。
- 应用启动时调用 `RecoverExpired`，将过期且仍可重试的任务恢复为 `pending`，将尝试耗尽的任务标记为 `failed`，避免永久停留在 `running`。
- worker ID 必须包含节点和进程唯一部分，不能只使用固定服务名。执行时间超过租约的任务必须周期性续租。
- payload、result 和错误消息不得包含密码、Token、支付密钥或完整章节正文；大内容和文件使用 MinIO 对象引用。
