# 旧 MySQL 迁移程序

入口：`server/cmd/moonbook-legacy-migrate`。旧库 DSN 使用 `MOONBOOK_LEGACY_MYSQL_DSN`，目标 PostgreSQL 使用 `MOONBOOK_DATABASE_DSN`，均只能通过运行环境或 Secret 注入，禁止写入仓库、日志和报告。

首个只读预检：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate preflight
```

运行前必须由 Secret 管理器或受控进程环境注入两个 DSN；不要把 DSN 直接写在 shell 命令、历史记录或脚本中。MySQL DSN 必须包含 `charset=utf8mb4&parseTime=true`。程序首先读取 `@@global.read_only`、`@@global.super_read_only` 和 `@@character_set_connection`；源库可写或连接字符集不是 `utf8mb4` 时立即拒绝迁移，不执行任何目标写入。`preflight` 只读取 MySQL 版本、当前数据库和 `information_schema` 业务表数量，并把完成证据写入 `migration_checkpoints`。

后续业务 stage 必须实现 `legacymigrate.Stage`：以 checkpoint cursor 和批量上限读取，不得一次载入整表；通过 Runner 提供的 PostgreSQL `*sql.Tx` 写入目标数据，并返回下一游标、处理数、显式错误清单和完成状态。Runner 在同一个 PostgreSQL 事务中提交业务目标、`migration_errors` 和 checkpoint，任一步失败全部回滚；重跑已完成 stage 时不会重复执行。具体转换规则和业务核对仍需在 M2-M6 逐域实现，不能仅依赖本骨架宣称迁移完成。

## 全量编排入口

已实现的迁移域可以用一次命令按依赖顺序执行：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate all
```

`all` 固定执行 `preflight`、小说分类/作者、书籍/副分类/封面、章节正文、Reader SEO、Reader 身份和 Reader Commerce。小说对象阶段需要额外注入 MinIO 端点、凭据和 Bucket；所有 DSN、凭据和允许的封面主机只能通过 Secret/环境变量注入。每个 stage 仍由同一个 Runner 使用独立 checkpoint，失败后重新执行会从最近提交的游标继续。该入口只编排当前已实现的 stage，不代表尚未实现的业务表或 8GB 生产副本演练已经完成。

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

M2 章节与正文迁移：

```bash
cd server
go run ./cmd/moonbook-legacy-migrate novel-chapters
```

`novel-chapters` 包含 `novel-books` 的全部前置阶段，随后按 `novel_chapter.id` 游标联表读取当前 `novel_chapter` 与 `novel_chapter_content`。它需要与书籍迁移相同的 PostgreSQL、MySQL 和 MinIO 环境变量；源 MySQL 只读和 `utf8mb4` 检查仍是所有 stage 的硬门槛。

每章正文限制为有效 UTF-8 且不超过 `(16 MiB)-1`，按 Unicode code point 排除空白重算字数。对象写入 MinIO 并以 `StatObject` 校验大小和 SHA-256 后，章节元数据、对象引用和书籍统计才在同一 PostgreSQL 事务提交。缺书、缺正文、非法状态、非法字段、上传失败和激活失败均写入 `migration_errors`；字数修正使用非重试错误码 `WORD_COUNT_RECALCULATED` 留下可核对证据。

章节来源指纹不依赖 migration name。已完成 checkpoint 会正常跳过；即使检查点丢失并使用新的 migration name 重跑，已有 legacy 正文对象也会被复用，不会生成重复版本。目标中同 ID 的人工或其他非 legacy 章节只记录 `CHAPTER_ID_CONFLICT`，不会覆盖。

整个命令默认最多运行 12 小时，可用 `MOONBOOK_MIGRATION_TIMEOUT` 在 `1m` 至 `24h` 间调整，例如 `6h`。超时只会中断当前批次；已提交批次的 checkpoint 保留，重新执行同一命令会从游标继续。正式演练必须根据容量和耗时报告设置小于停机窗口且留有核对、冒烟和回退余量的值。
