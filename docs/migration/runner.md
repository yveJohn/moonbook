# 旧 MySQL 迁移程序

入口：`server/cmd/moonbook-legacy-migrate`。旧库 DSN 使用 `MOONBOOK_LEGACY_MYSQL_DSN`，目标 PostgreSQL 使用 `MOONBOOK_DATABASE_DSN`，均只能通过运行环境或 Secret 注入，禁止写入仓库、日志和报告。

首个只读预检：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate preflight
```

运行前必须由 Secret 管理器或受控进程环境注入两个 DSN；不要把 DSN 直接写在 shell 命令、历史记录或脚本中。程序首先读取 `@@global.read_only` 和 `@@global.super_read_only`。两者均未启用时立即拒绝迁移，不执行任何目标写入。`preflight` 只读取 MySQL 版本、当前数据库和 `information_schema` 业务表数量，并把完成证据写入 `migration_checkpoints`。

后续业务 stage 必须实现 `legacymigrate.Stage`：以 checkpoint cursor 和批量上限读取，不得一次载入整表；通过 Runner 提供的 PostgreSQL `*sql.Tx` 写入目标数据，并返回下一游标、处理数、显式错误清单和完成状态。Runner 在同一个 PostgreSQL 事务中提交业务目标、`migration_errors` 和 checkpoint，任一步失败全部回滚；重跑已完成 stage 时不会重复执行。具体转换规则和业务核对仍需在 M2-M6 逐域实现，不能仅依赖本骨架宣称迁移完成。
