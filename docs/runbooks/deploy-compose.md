# Docker Compose 部署与本地运行

## 范围

根目录 `compose.yaml` 管理 Moonbook Go API、管理前端、冻结版读者 SSR、Nginx 网关、PostgreSQL、Redis、MinIO，以及一次性数据库迁移和建桶任务。项目名、网络、容器关联资源和命名卷均使用 `moonbook` 前缀，不得操作其他 Compose 项目。

## 首次启动

```bash
cp .env.example .env
scripts/render-local-config.sh
docker compose --env-file .env up -d
docker compose --env-file .env ps
MOONBOOK_ADMIN_USERNAME=admin \
MOONBOOK_ADMIN_PASSWORD='<至少 12 位的本地初始密码>' \
MOONBOOK_ADMIN_NICKNAME='Moonbook Admin' \
docker compose --env-file .env --profile bootstrap run --rm admin-bootstrap
```

`migrate` 会在 API 前运行并只执行待处理迁移；`minio-init` 幂等创建私有 Bucket。API 容器以非 root 用户运行，启动时从环境变量安全生成权限为 `0600` 的配置文件。API、管理静态站和读者 SSR 不直接绑定公网地址，统一经 `gateway` 暴露。

只在宿主机直接运行 Go 服务时，才需要生成本地配置：

```bash
cd server
GVA_CONFIG=config.moonbook.local.yaml go run .
```

Prometheus 指标经管理网关位于 `GET /api/metrics`，必须使用 `.env` 中的 `MOONBOOK_METRICS_TOKEN`：

```bash
curl -H "Authorization: Bearer $MOONBOOK_METRICS_TOKEN" \
  http://127.0.0.1:18080/api/metrics
```

执行手工抓取时不得启用 shell `set -x`，也不得分享包含请求头的调试输出。未配置 Token 时端点返回 503，未授权请求返回 401。生产环境必须通过 Secret 注入独立高熵 Token，并在 Prometheus `bearer_token_file` 中引用挂载的 Secret，不得使用 `.env.example` 的本地占位值。指标使用路由模板而不是原始路径，不包含用户 ID、对象 ID、错误文本或请求正文。

应用启动前必须先执行版本化迁移。`scripts/migrate-local.sh status` 显示当前版本、目标版本和是否存在待执行迁移；`up` 使用事务和 PostgreSQL 会话锁执行所有待执行迁移，重复执行不会重复创建业务对象。迁移事实记录在 `moonbook_schema_version`，不得通过 GORM `AutoMigrate` 替代 Moonbook 业务迁移。

数据库迁移不会写入默认管理员或示例用户。首次部署必须通过 Compose `bootstrap` profile 显式引导管理员；用户名和密码仅从进程环境变量读取，密码至少 12 位。命令在事务和 PostgreSQL advisory lock 下执行，只允许空的 `sys_users` 表引导一次，重复执行会拒绝覆盖。新管理员首次登录后必须修改初始密码。Moonbook 不提供 GVA 的 HTTP `/init/initdb` 建库入口。

## 本地端口

| 服务 | 缺省地址 |
| --- | --- |
| PostgreSQL | `127.0.0.1:15432` |
| Redis | `127.0.0.1:16379` |
| MinIO API | `http://127.0.0.1:19000` |
| MinIO Console | `http://127.0.0.1:19001` |
| 管理端网关 | `http://127.0.0.1:18080` |
| 读者端网关 | `http://127.0.0.1:18081` |

所有宿主端口只绑定回环地址。Go API 不绑定宿主端口，管理端 `/api/` 和读者生产路径 `/prod-api/` 由网关转发；Reader 镜像默认以 `/prod-api` 构建，网关同时保留 `/dev-api/` 供本地开发兼容。Bucket `moonbook-content` 由 `minio-init` 幂等创建，并强制为私有访问。

## 日常命令

```bash
docker compose --env-file .env ps
docker compose --env-file .env logs --tail=200 gateway server reader-ui web
docker compose --env-file .env stop
docker compose --env-file .env start
```

`stop` 和普通 `down` 不删除数据卷。禁止使用 `down -v`，除非已经确认 `moonbook_postgres_data`、`moonbook_redis_data` 和 `moonbook_minio_data` 只含可丢弃的本地数据，并得到与当前操作风险相符的授权。

## 配置与秘密

- `.env.example` 只含本地开发占位值，不能用于生产。
- `.env` 和生成的 `server/config.moonbook.local.yaml` 不提交。
- GVA 通过 `GVA_CONFIG` 选择生成配置文件；数据库、Redis、MinIO 和 JWT 密钥均由 `.env` 注入模板。
- AI 配置表只保存形如 `MOONBOOK_AI_*_API_KEY` 的环境变量引用。Compose 示例透传 `MOONBOOK_AI_EXAMPLE_API_KEY`；新增供应商时必须在部署清单中显式透传对应 Secret，管理 API 只显示是否已注入，禁止把密钥值填入 PostgreSQL。
- 生产部署必须使用 Secret 管理，不得复制本地 `.env`。

## 验证

完整 M1 空环境验收使用：

```bash
make verify-m1
```

命令只允许 `moonbook_verify_` 前缀的独立 Compose 项目，使用全新命名卷验证镜像构建、迁移、基础设施、网关、指标鉴权、管理员引导和登录、非 root 运行、优雅停机及 Gitleaks 秘密扫描。默认在结束时删除测试容器、网络和卷，不操作日常 `moonbook` 项目。端口冲突时可通过 `MOONBOOK_VERIFY_*_PORT` 环境变量指定专用端口。

`scripts/verify-infrastructure.sh` 使用容器内客户端验证：

- PostgreSQL 接受指定数据库和用户连接；
- Redis 密码认证后返回 `PONG`；
- MinIO readiness 健康；
- 内容 Bucket 可幂等创建且保持私有。
