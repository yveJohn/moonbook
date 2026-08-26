# Moonbook MySQL 到 PostgreSQL/R2 迁移设计

日期：2026-08-26

## 目标与边界

在 `101.32.210.229` 上部署仅本机和 Docker 内网可访问的 PostgreSQL，并将旧 MySQL `moonbook_admin` 迁移到新 Moonbook。所有业务事实以 MySQL 数据库为准；原始 TXT 文件不是迁移前置条件。

本次迁移不开放公网 PostgreSQL 5432，不迁移 Redis 缓存，不把 R2 凭据暴露给客户端，也不在仓库或日志中保存数据库和对象存储 Secret。

## 拓扑与存储

```text
MySQL moonbook_admin（只读快照）
             |
             v
PostgreSQL moonbook_admin <--- Moonbook server/worker
             |
             +---- R2 私有 Bucket moonbook
```

PostgreSQL 绑定 `127.0.0.1`，应用容器通过 Docker 网络访问。R2 使用 S3 API 和私有 Bucket；后端执行权限、付费和对象完整性判断后读取对象并流式返回客户端。

## 数据规则

- 书籍、分类、作者、章节、章节正文、Reader、Commerce、内容生产数据均从 MySQL 迁移。
- 章节正文以 `novel_chapter_content` 为唯一正文事实，不读取原始 TXT。
- `novel_txt_import_task` 仅迁移数据库中的任务元数据、状态、统计、错误和目标关联。
- 原始 TXT 缺失不阻断迁移；不得伪造 TXT `object_key`、SHA-256 或对象大小。
- 封面、章节正文、清洗稿和其他新系统对象写入 R2，保存对象大小和 SHA-256，并由迁移审计核对。
- 已存在但未迁移的原始 TXT 对象在报告中记录为历史对象缺失，不影响 Reader 阅读能力。

## 迁移流程

1. 对源 MySQL 创建一致性快照；迁移源账号仅授予 `SELECT` 和 `SHOW VIEW`。
2. 使用 Goose 迁移建立空目标 PostgreSQL schema，并验证重复执行为 `applied=0`。
3. 先执行小批量迁移，核对书籍、章节、正文对象和 Reader 登录读取。
4. 执行 `moonbook-legacy-migrate all` 的数据库阶段，使用 checkpoint、批量事务和幂等键恢复失败批次。
5. TXT 阶段改为数据库事实模式，不依赖 manifest 和原始文件对象。
6. 对 R2 执行对象大小、SHA-256、存在性和读取权限核对。
7. 执行行数/最大 ID、关联、财务、对象和幂等重跑审计；任一差异保持 No-Go。
8. 通过完整核对后，才切换 Moonbook 的 PostgreSQL/R2 配置和流量。

## 安全与回退

- 数据库和 R2 Secret 只通过服务器 Secret/环境变量注入。
- PostgreSQL 不发布公网端口；SSH 仅用于受控运维。
- 迁移期间保留源 MySQL 和旧应用，不执行破坏性删除。
- 目标核对失败时停止切换，保留迁移 checkpoint 和隔离目标供排查。
- 迁移完成并通过回退窗口后，再由变更负责人批准旧系统下线。

## 验收标准

- PostgreSQL 仅监听 `127.0.0.1`/Docker 内网。
- MySQL 源快照前后行数、关键表最大 ID 和源只读状态可复核。
- 书籍、章节、正文、Reader、Commerce 关键事实无缺失或断关联。
- R2 私有对象可由后端读取，客户端无法获得 R2 凭据；对象大小和 SHA-256 全部一致。
- 迁移重复执行不增加业务事实，财务核对零差异。
- Reader SSR、章节阅读、管理端和健康检查通过。
