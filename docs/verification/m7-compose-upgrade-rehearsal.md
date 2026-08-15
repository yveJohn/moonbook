# M7 Docker Compose 升级与应用回退演练报告

演练时间：2026-08-15 01:06-01:10 UTC

## 范围

本次演练验证固定提交 Server 镜像构建、旧版本全栈启动、持久化夹具、目标镜像一次性迁移、应用层替换、健康冒烟、目标能力断言，以及数据库不降级时的旧应用镜像回退。项目、端口、网络和数据卷均与日常环境隔离，未连接生产资源。

- 隔离项目：`moonbook_verify_upgrade`
- 升级基线：`3058fee0fcf85ec12ae60852e96443f37a4173ea`
- 升级目标：`a8c209d630e918dd780c7e7c7544cbdfdb320fb7`
- 旧 Server 镜像：`moonbook/server:upgrade-3058fee`，image ID `848073ca161e`
- 目标 Server 镜像：`moonbook/server:upgrade-a8c209d`，image ID `42bc19d2ee9a`
- 应用镜像：管理端 `9622ab4d21b8`，Reader `56409bf67f9c`
- 基础设施：PostgreSQL `17.6-alpine`、Redis `7.4.5-alpine`、MinIO `RELEASE.2025-07-23T15-54-02Z`

两个提交之间没有数据库迁移差异；变更只涉及备份恢复文档、验证证据和 Server 镜像增加 `moonbook-finance-reconcile` 命令。

## 基线结果

- 从 `3058fee` 的 Git archive 构建旧 Server 镜像，没有切换或重置工作树。
- 旧版本完整 Compose 栈全部 healthy；管理网关 `/gateway-health`、API `/api/health/ready` 和 Reader `/health` 均返回成功。
- 空库迁移后 `moonbook_schema_version` 有 59 条已应用记录，最高版本为 `58`。
- 在 `sys_params` 写入无害夹具：ID `1`、key `m7.compose.upgrade.fixture`、value `baseline-3058fee`。
- 旧镜像执行 `moonbook-finance-reconcile` 返回退出码 `127` 和 `not found`，证明目标提交新增能力尚不存在。

首次使用的 PostgreSQL 回环端口已被本机其他进程占用，Compose 在数据库启动前失败。仅清理新建且为空的 `moonbook_verify_upgrade` 容器、网络和卷后，改用已确认空闲的高位端口重新执行；未停止或修改占用端口的其他项目。

## 升级结果

1. 只把隔离环境中的 `MOONBOOK_SERVER_IMAGE` 从旧 tag 改为目标 tag。
2. 使用目标镜像运行一次性迁移，输出 `applied=0`。
3. 强制重建 `server`、`reader-ui` 和 `gateway`；Compose 同时按新镜像重建一次性 `migrate` 服务。
4. `server` 和 `migrate` 均运行 image ID `42bc19d2ee9a`。
5. 迁移仍为 59 条已应用记录、最高版本 `58`；夹具 ID、key 和 value 均未变化。
6. 网关、API readiness 和 Reader health 再次通过，readiness 的 migrations、MinIO、PostgreSQL、Redis 全部为 `ok`。
7. 目标镜像执行财务核对成功，输出 `checked=0 mismatches=0`。

## 应用镜像回退结果

在不执行任何数据库向下迁移或数据恢复的前提下，将 Server tag 改回 `3058fee` 并重建应用层：

- `server` 和 `migrate` 均回到 image ID `848073ca161e`；
- 数据库仍为 59 条已应用记录、最高版本 `58`；
- `sys_params` 夹具仍为 ID `1`、value `baseline-3058fee`；
- 网关、API readiness 和 Reader health 全部通过。

该回退只证明这两个迁移完全一致的提交可以回退应用镜像，不证明任意数据库迁移后的旧应用兼容性。通用 Runbook 已把“明确的迁移向后兼容证据”设为应用镜像回退硬门槛；不满足时必须保持停写并恢复升级前协调备份。

## 清理与结论

演练完成后，`moonbook_verify_upgrade` 的容器、网络和三个专用命名卷全部删除；临时环境文件和 Git archive 位于仓库外，不进入提交。日常 `moonbook` Compose 项目和冻结旧仓库均未操作。

本次证据关闭 Compose 升级顺序、固定 Server 镜像替换、无迁移升级、基础冒烟和特定兼容版本应用回退的命令级缺口。它没有证明真实业务升级、非零财务核对、认证业务旅程、含数据库迁移的向后兼容、外部 TLS/可信代理、资源限制或生产观察阈值，因此 M7 仍不能标记完成。
