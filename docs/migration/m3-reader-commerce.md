# M3 Reader 与 Commerce 迁移及验收记录

## 迁移边界

目标数据库固定为 PostgreSQL `moonbook_admin`，迁移通过 `moonbook-legacy-migrate` 的只读预检、stage、批量事务和 checkpoint 执行。旧 MySQL 必须满足 `@@global.read_only=1` 或 `@@global.super_read_only=1`，连接字符集必须为 `utf8mb4`。M3 不迁移钱包、订单、支付回调或邀请奖励发放写流程。

## 表字段映射

| 旧 MySQL | 新 PostgreSQL | 规则 |
| --- | --- | --- |
| `reader_user.id,username,nickname,password_hash,status,token_version,invite_code_id,last_login_time,create_time,update_time` | `reader_accounts.id,username,nickname,password_digest,password_algorithm,status,used_invite_code_id,last_login_at,created_at,updated_at` | ID 原值 `bigint`；BCrypt 标记 `bcrypt`；32 位历史摘要标记 `md5_legacy`，禁止日志输出摘要 |
| `reader_invite_code.id,code,inviter_reader_id,status,max_use_count,used_count,expire_time,remark,create_time,update_time` | `reader_invite_codes.id,code,inviter_reader_id,status,max_use_count,used_count,expires_at,remark,created_at,updated_at` | `code` 唯一；消费在注册事务中锁定并检查上限/过期 |
| `reader_invite_relation.id,inviter_reader_id,invitee_reader_id,invite_code,status,create_time` | `reader_invite_relations.id,inviter_reader_id,invitee_reader_id,invite_code,status,created_at` | 被邀请人唯一；M3 只建立关系事实 |
| `reader_bookshelf.id,reader_id,book_id,last_chapter_id,last_read_time,create_time,update_time` | `reader_bookshelf_entries.*` | `(reader_id,book_id)` 冲突时保留目标事实并按 stage 规则更新进度 |
| `reader_book_like.id,reader_id,book_id,create_time` | `reader_book_likes.*` | `(reader_id,book_id)` 幂等，计数由业务事务维护 |
| `reader_reading_history.*` | `reader_reading_history.*` | `(reader_id,book_id)` 唯一；章节归属、位置和百分比重新校验 |
| `reader_reading_preference.*` | `reader_reading_preferences.*` | 每读者单例；非法枚举写错误清单，不静默改值 |
| `reader_feedback.*` | `reader_feedback.*` | 读者隔离；原状态映射为 `pending/replied` |
| `reader_product.id,product_type,target_id,product_name,price_coin,allow_bonus_coin,duration_days,sale_status,sort_order` | `commerce_products.*` | 商品正式事实表；M3 只读访问，不创建购买写流程 |
| `novel_chapter_word_pricing`（旧章节计价配置） | `commerce_chapter_pricing_config.*` | 保留字数单位/币值单位；无效配置写错误清单 |
| `reader_membership_grant.*` | `commerce_membership_grants.*` | 保留来源引用、永久标记和有效期 |
| `reader_entitlement.id,reader_id,entitlement_type,target_id,status,start_time,expire_time,source_order_no` | `commerce_entitlements.*` | `source_order_no` 可为空并保留旧来源类型；不复制到临时投影表 |

所有时间转换为 `timestamptz`；所有旧 ID 保留为 PostgreSQL `bigint`，迁移结束后序列推进到最大值。冲突采用唯一键 + `ON CONFLICT` 幂等策略；目标存在非 legacy 同 ID 时记录不可重试 `*_ID_CONFLICT`，不覆盖人工数据。

## Stage 与 checkpoint

固定顺序：`reader-identity`、`reader-invites`、`reader-commerce`、`reader-bookshelf-likes`、`reader-history-preferences`、`reader-feedback`。每批最多 1,000 行（可由命令配置但不得一次载入整表）；Runner 在同一 PostgreSQL 事务写入目标事实、`migration_errors` 和 `migration_checkpoints`。`cursor_value` 使用源表 bigint ID，`metadata.done=true` 表示完成；重复运行已完成 stage 直接跳过，丢失 checkpoint 时依靠来源指纹和唯一键避免重复对象/事实。

核对 checkpoint：

```sql
SELECT migration_name, stage, cursor_value, processed_count, error_count,
       metadata->>'done' AS done, updated_at
FROM migration_checkpoints
WHERE migration_name = 'm3-reader-commerce'
ORDER BY stage;
```

## 核对 SQL

```sql
-- 行数与错误汇总
SELECT 'reader_accounts' AS table_name, count(*) FROM reader_accounts
UNION ALL SELECT 'reader_invite_codes', count(*) FROM reader_invite_codes
UNION ALL SELECT 'reader_invite_relations', count(*) FROM reader_invite_relations
UNION ALL SELECT 'commerce_products', count(*) FROM commerce_products
UNION ALL SELECT 'commerce_entitlements', count(*) FROM commerce_entitlements;

SELECT stage, count(*) AS errors,
       count(*) FILTER (WHERE retryable) AS retryable_errors
FROM migration_errors
WHERE migration_name='m3-reader-commerce'
GROUP BY stage ORDER BY stage;

-- 关键唯一性、越界与敏感信息检查
SELECT username, count(*) FROM reader_accounts GROUP BY username HAVING count(*) > 1;
SELECT reader_id, book_id, count(*) FROM reader_bookshelf_entries
GROUP BY reader_id, book_id HAVING count(*) > 1;
SELECT id FROM reader_accounts WHERE id < 0;
SELECT error_message FROM migration_errors
WHERE migration_name='m3-reader-commerce'
  AND (error_message ILIKE '%password%' OR error_message ILIKE '%digest%');
```

最后一条必须返回 0 行；任何密码或摘要泄漏均判定迁移失败。还必须对照源库快照逐表记录源行数、目标行数、错误数和差异样本，不能仅凭总数宣称完成。

## 8GB 副本演练参数

演练只使用脱敏、受控副本，不把备份文件、DSN 或密钥提交仓库。建议先恢复到独立 MySQL 容器，启用 `read_only`/`super_read_only`，连接使用 `charset=utf8mb4&parseTime=true`；目标使用独立项目 PostgreSQL 卷，Redis/MinIO 使用项目专用资源。按 1,000 行批次运行，设置 `MOONBOOK_MIGRATION_TIMEOUT` 为停机窗口的 70%（上限 12 小时），预留核对、冒烟和回退时间；每批保留 checkpoint，失败从最近提交批次继续。演练报告至少包含：源/目标版本、开始/结束时间、吞吐、峰值连接和磁盘、每 stage processed/error、抽样哈希、重跑结果及回退决定。未取得脱敏 8GB 副本前，不得声称容量演练通过。

## 当前验证状态

### 真实依赖集成测试

测试代码位于以下包，并使用迁移后的真实表和真实依赖：

- `internal/modules/reader/auth`：PostgreSQL 账号/会话、历史 MD5 登录升级、Redis 双维度限流和 Token 撤销。
- `internal/modules/reader/public`：发布书籍与章节、MinIO 正文上传/激活/读取、Reader 权益判定。
- `internal/modules/reader/me`：书架、点赞、阅读历史、偏好、反馈，以及两个 `reader_id` 的隔离。
- `internal/modules/commerce/catalog`：商品/章节价格、会员授予、章节权益和批量访问判定隔离。

一键执行：

```bash
export MOONBOOK_READER_TEST_DSN='postgres://moonbook:password@127.0.0.1:5432/moonbook_admin?sslmode=disable'
export MOONBOOK_READER_TEST_REDIS_ADDR='127.0.0.1:6379'
export MOONBOOK_READER_TEST_REDIS_PASSWORD=''
export MOONBOOK_READER_TEST_MINIO_ENDPOINT='127.0.0.1:9000'
export MOONBOOK_READER_TEST_MINIO_ACCESS_KEY='minioadmin'
export MOONBOOK_READER_TEST_MINIO_SECRET_KEY='minioadmin'
make test-reader-integration
```

测试仅使用上述连接指向的本地项目资源；每次运行创建随机 MinIO bucket，并在清理阶段删除 bucket、对象及带测试 ID 的数据库行。缺少任一依赖变量时测试会 `Skip`，不能将该结果作为真实集成验收证据。

- M3 Reader/Commerce 代码、真实迁移 stage 和五路由契约测试已完成；本次补充的真实依赖测试覆盖上列四个模块及注册事务。
- 2026-08-14 在隔离 Compose 项目 `moonbook_m3_verify` 上完成真实验证：Flyway `current=0 target=12 pending=true`，执行后 `applied=12`，复查 `current=12 target=12 pending=false`；使用 PostgreSQL `127.0.0.1:25432`、Redis `127.0.0.1:26379`、MinIO `127.0.0.1:29000`，命令以 `CGO_ENABLED=0 -tags=integration -count=1 -v` 运行，`reader/auth`、`reader/invite`、`reader/me`、`reader/public`、`commerce/catalog` 全部 PASS。
- 本次验证同时覆盖章节正文实际 MinIO 写入/读取和不同 reader 的批量权益隔离；测试完成后仅清理随机测试数据及 bucket，未触碰默认 `moonbook` 资源。
- 冻结 `reader-ui` 使用本地缓存离线执行 `npm run test -- --run`，44 个测试文件、536 个用例全部通过；`npm run build` 的 SSR 生产构建通过。临时 mock API SSR 验证首页、登录页、robots 均返回预期结果，但书库页面因 mock 未提供完整数据响应返回 503，真实 API 书库/书籍/章节 SSR 和浏览器旅程仍是 M3 未关闭项。
- 2026-08-14 在隔离 Go API `127.0.0.1:4188` + Reader SSR `127.0.0.1:4189` 上完成真实上游验证：`/reader/seo/config`、精选/随机/分页书库、分类、robots、sitemap 均返回预期 200；SSR `/`、`/books`、`/auth/login`、`/robots.txt`、`/sitemap.xml` 均返回 200，书库空状态、SEO title/canonical/JSON-LD 和登录页 `noindex,nofollow` 正确。该临时 API readiness 因隔离 MinIO bucket 未创建返回 503，未影响本次只读路由验证。
- 新增 `reader/public/http_integration_test.go` 后，完整五包真实集成套件再次通过；该测试直接注册 Gin Reader public 路由，验证匿名分页书库、Bearer Reader 章节目录和 MinIO 正文响应，Long ID 为字符串。
- Playwright CLI 试运行因 `@playwright/cli` 未在本机 npm 缓存且当前网络受限而未启动，浏览器级关键旅程仍未验收；不能将 curl/SSR 结果替代浏览器证据。
- `CGO_ENABLED=0 -tags=integration` 编译和无环境变量路径已验证：缺少 `MOONBOOK_READER_TEST_*` 时对应测试明确 `Skip`，不会伪造通过；配置完整依赖后必须保留 verbose 原始输出作为验收证据。
- 当前仓库已知本机 macOS ARM cgo 会在上游 `go-m1cpu` 初始化时崩溃；计划中的后端回归应使用 `CGO_ENABLED=0` 可重复路径，race 需在官方 Linux Go 容器执行。该限制不能被记录为 M3 通过证据。
- 计划命令 `go test ./internal/... ./initialize/... ./cmd/moonbook-legacy-migrate/...` 和 Docker 集成测试须在实现后运行并保存原始输出摘要。
