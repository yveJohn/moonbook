# 旧 MySQL 迁移程序

入口：`server/cmd/moonbook-legacy-migrate`。旧库 DSN 使用 `MOONBOOK_LEGACY_MYSQL_DSN`，目标 PostgreSQL 使用 `MOONBOOK_DATABASE_DSN`，均只能通过运行环境或 Secret 注入，禁止写入仓库、日志和报告。

首个只读预检：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate preflight
```

运行前必须由 Secret 管理器或受控进程环境注入两个 DSN；不要把 DSN 直接写在 shell 命令、历史记录或脚本中。MySQL DSN 必须包含 `charset=utf8mb4&parseTime=true`。程序首先读取 `@@global.read_only`、`@@global.super_read_only` 和 `@@character_set_connection`；源库可写或连接字符集不是 `utf8mb4` 时立即拒绝迁移，不执行任何目标写入。`preflight` 只读取 MySQL 版本、当前数据库和 `information_schema` 业务表数量，并把完成证据写入 `migration_checkpoints`。

后续业务 stage 必须实现 `legacymigrate.Stage`：以 checkpoint cursor 和批量上限读取，不得一次载入整表；通过 Runner 提供的 PostgreSQL `*sql.Tx` 写入目标数据，并返回下一游标、处理数、显式错误清单和完成状态。Runner 在同一个 PostgreSQL 事务中提交业务目标、`migration_errors` 和 checkpoint，任一步失败全部回滚；重跑已完成 stage 时不会重复执行。具体转换规则和业务核对仍需在 M2-M6 逐域实现，不能仅依赖本骨架宣称迁移完成。

M2 小说分类与作者迁移：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate novel-metadata
```

该命令会先执行同一个只读预检，再按顺序迁移现行分类字典、历史分类、现行书籍作者、`book_author` 和更早期的 `author`。每个阶段按 bigint 游标分批，并与 checkpoint、错误清单在同一 PostgreSQL 事务提交。`sys_dict_data` 与 `novel_book` 是当前业务必需来源，缺表会失败；更早期的补充表不存在时会在 checkpoint 元数据中明确记录 `table_not_found`。

M2 书籍、副分类和封面迁移：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate novel-books
```

`novel-books` 包含 `novel-metadata` 的全部前置阶段，然后迁移 `novel_book`、`reader_product(product_type='book')`、`novel_book_sub_category_rel` 和非空 `legacy_cover_url`。除两个数据库 DSN 外，还必须通过 Secret 注入 `MOONBOOK_MINIO_ENDPOINT`、`MINIO_ROOT_USER`、`MINIO_ROOT_PASSWORD`、`MINIO_BUCKET`；TLS 端点设置 `MOONBOOK_MINIO_USE_SSL=true`。迁移程序不会输出这些值。

封面下载默认总超时 20 秒、最多 3 次重定向、最大 10 MiB，只接受 HTTP(S) 和 JPEG/PNG/WebP/GIF 魔数。非白名单地址只能使用 80/443 端口，DNS 解析结果只要包含环回、私网、链路本地或其他非公网地址就拒绝；连接阶段会再次解析和校验，防止重定向及 DNS 重绑定绕过。旧封面确实位于受控内网时，才可通过 `MOONBOOK_LEGACY_COVER_ALLOWED_HOSTS` 显式配置逗号分隔的主机名/IP；该变量不能填写 URL、路径或凭据。`MOONBOOK_LEGACY_COVER_TIMEOUT_SECONDS` 可在 1 至 120 秒间调整单个下载超时。

封面 URL 仅以 SHA-256 指纹写入对象元数据，避免带签名参数的 URL 泄漏；实际 URL 仍只保留在 `novel_books.legacy_cover_url` 供错误排查。下载、格式、上传或激活失败会写入 `migration_errors`，不会覆盖已有活动封面。MinIO 写入经大小和 SHA-256 校验后才激活 PostgreSQL 引用；同一书籍和 URL 指纹重跑不会新建重复版本。

整个命令默认最多运行 12 小时，可用 `MOONBOOK_MIGRATION_TIMEOUT` 在 `1m` 至 `24h` 间调整，例如 `6h`。超时只会中断当前批次；已提交批次的 checkpoint 保留，重新执行同一命令会从游标继续。正式演练必须根据容量和耗时报告设置小于停机窗口且留有核对、冒烟和回退余量的值。
