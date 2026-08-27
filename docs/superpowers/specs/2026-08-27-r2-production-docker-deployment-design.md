# Moonbook R2 生产 Docker 部署设计

## 1. 目标与边界

将当前仓库固定提交在本地构建为 Go 后端、管理前端和 Reader 前端三个应用镜像，传输到 `101.32.210.229` 后使用 Docker Compose 部署。部署复用服务器上已经完成数据迁移的 PostgreSQL 和 Redis 容器，章节正文与清洗正文继续使用 Cloudflare R2 私有桶。

本次不重建或删除 PostgreSQL、Redis、MySQL、临时 MinIO 及其数据，不执行 DNS、证书或宿主反向代理配置。MySQL 保持双只读，临时 MinIO 保持停止和数据保留状态。

## 2. 入口与网络

- Reader 网关只绑定宿主 `127.0.0.1:18081`，由 `https://ybsc.me` 反向代理。
- 管理网关只绑定宿主 `127.0.0.1:18080`，由 `https://fmjzzkco9vlc.ybsc.me` 反向代理。
- Go API、管理静态站和 Reader SSR 不直接发布宿主端口，只通过应用网关访问。
- 新建独立应用 Compose 网络，并将现有 `moonbook-postgres`、`moonbook-redis` 接入该网络。应用通过容器 DNS 名访问数据库和 Redis，不扩大数据库宿主端口暴露范围。
- 网关重写转发头；Go 服务只信任应用网关所在 Docker 子网，不使用全网代理信任配置。

## 3. 镜像与发布物

本地使用仓库 Dockerfile 构建以下镜像，并以 Git 短提交号作为不可变版本标签：

- `moonbook/server:{git-short-sha}`
- `moonbook/web:{git-short-sha}`
- `moonbook/reader-ui:{git-short-sha}`

镜像在本地导出为压缩归档，计算 SHA-256 后传输到服务器。服务器校验归档哈希后使用 `docker load` 导入，不在生产服务器编译源码。发布清单记录 Git 提交、镜像 ID、镜像摘要和配置版本。

## 4. R2 私有对象存储

现有对象存储实现继续通过 S3 兼容的 MinIO SDK 访问 R2。生产配置使用 R2 S3 API 端点、私有桶、Access Key 和 Secret Key，并显式启用 TLS。浏览器和 Reader 不获得 R2 凭据或私有对象 URL；章节正文仍由 Go API 完成权限校验、读取并返回。

当前配置模板把对象存储 TLS 固定为关闭，部署前需增加 `MOONBOOK_MINIO_USE_SSL` 配置项并在生产设置为 `true`。该修改只改变对象存储传输协议配置，不改变对象键、数据库引用和接口契约。

## 5. Secret 与运行配置

生产运行文件位于 `/opt/moonbook-app/`：

- Compose 文件和非敏感网关配置权限不高于 `0644`；
- 环境 Secret 文件权限为 `0600`，归 root 所有；
- 数据库、Redis、JWT、指标、R2、论坛 Cookie 和第三方密钥仅通过环境注入；
- Secret 不进入 Git、镜像、镜像归档名称、日志或发布清单。

CORS 固定允许 `https://ybsc.me` 和 `https://fmjzzkco9vlc.ybsc.me`。Reader 构建时 API 前缀保持 `/prod-api`。

## 6. 启动顺序

1. 检查宿主容量、现有容器健康状态、MySQL 双只读和 PostgreSQL 版本。
2. 构建、导出、传输并校验三个应用镜像。
3. 创建应用目录、网络、Secret 文件和 Compose 文件。
4. 将现有 PostgreSQL、Redis 接入应用网络。
5. 运行一次性 `moonbook-migrate up`；当前数据库已在版本 72 时应为幂等无变更。
6. 启动 Go API，等待 readiness 通过。
7. 启动管理前端、Reader SSR 和网关，等待全部健康检查通过。
8. 执行容器重启恢复与 HTTP 冒烟验收。

## 7. 验收

必须验证：

- PostgreSQL、Redis、Go API、管理前端、Reader 和网关容器健康；
- `127.0.0.1:18080` 管理入口和 `127.0.0.1:18081` Reader 入口返回成功；
- 管理登录页、Reader 首页和后端健康接口可访问；
- 选取已迁移的有效章节，通过 Reader API 读取正文成功，且响应不泄漏 R2 对象键或凭据；
- MySQL 仍为 `read_only=1`、`super_read_only=1`；
- R2 对象数量不因部署发生变化；
- 所有应用容器重启后重新达到健康状态。

反向代理和域名证书由用户配置后，再从公网域名验证 HTTPS、CORS、转发头和登录流程。

## 8. 失败与回退

这是目标应用的首次部署，失败时停止并移除本次应用 Compose 容器即可。数据库迁移为前向迁移，不执行数据库降级；当前版本无待执行迁移时不会改变结构。不得删除数据库、Redis、MinIO 数据或 R2 对象，不得解除 MySQL 只读。

修复后使用新的不可变镜像标签重新发布。旧应用镜像至少保留到域名验收和观察期结束。
