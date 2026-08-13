# Docker Compose 本地基础设施

## 范围

根目录 `compose.yaml` 当前只管理 Moonbook 专用 PostgreSQL、Redis、MinIO 和一次性 MinIO 建桶任务。应用服务将在 M1 后续单元加入。项目名、网络、容器关联资源和命名卷均使用 `moonbook` 前缀，不得操作其他 Compose 项目。

## 首次启动

```bash
cp .env.example .env
scripts/render-local-config.sh
docker compose --env-file .env up -d postgres redis minio minio-init
scripts/verify-infrastructure.sh
```

生成的 `server/config.moonbook.local.yaml` 权限为 `0600`，被 Git 忽略。启动 Go 服务时使用：

```bash
cd server
GVA_CONFIG=config.moonbook.local.yaml go run .
```

## 本地端口

| 服务 | 缺省地址 |
| --- | --- |
| PostgreSQL | `127.0.0.1:15432` |
| Redis | `127.0.0.1:16379` |
| MinIO API | `http://127.0.0.1:19000` |
| MinIO Console | `http://127.0.0.1:19001` |
| Go API（后续） | `http://127.0.0.1:18888` |

所有端口只绑定回环地址。Bucket `moonbook-content` 由 `minio-init` 幂等创建，并强制为私有访问。

## 日常命令

```bash
docker compose --env-file .env ps
docker compose --env-file .env logs --tail=200 postgres redis minio
docker compose --env-file .env stop
docker compose --env-file .env start
```

`stop` 和普通 `down` 不删除数据卷。禁止使用 `down -v`，除非已经确认 `moonbook_postgres_data`、`moonbook_redis_data` 和 `moonbook_minio_data` 只含可丢弃的本地数据，并得到与当前操作风险相符的授权。

## 配置与秘密

- `.env.example` 只含本地开发占位值，不能用于生产。
- `.env` 和生成的 `server/config.moonbook.local.yaml` 不提交。
- GVA 通过 `GVA_CONFIG` 选择生成配置文件；数据库、Redis、MinIO 和 JWT 密钥均由 `.env` 注入模板。
- 生产部署必须使用 Secret 管理，不得复制本地 `.env`。

## 验证

`scripts/verify-infrastructure.sh` 使用容器内客户端验证：

- PostgreSQL 接受指定数据库和用户连接；
- Redis 密码认证后返回 `PONG`；
- MinIO readiness 健康；
- 内容 Bucket 可幂等创建且保持私有。
