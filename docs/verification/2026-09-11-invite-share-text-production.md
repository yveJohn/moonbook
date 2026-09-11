# 邀请文案参数管理生产发布

用户于本次任务明确授权本地构建并发布生产。发布在 **2026-09-11 13:31（UTC+8）** 完成，固定提交 `243df5d566f3d5c56d7e1550ec9a434ab415ed81`。

## 范围

本次镜像包含：

- 读者邀请分享文案改为读取 `sys_params` 键 `reader.invite.shareText`。
- 迁移 `00078`：仅在键不存在时插入默认文案 `邀请你加入月白书城，点击 {{link}} 注册`。
- 后台继续使用 GVA「参数管理」，未新增独立管理页。

未修改生产 Secret、DNS、证书、支付或既有参数值。

## 发布结果

| 项目 | 结果 |
| --- | --- |
| 生产目标 | `101.32.210.229:/opt/moonbook-app` |
| Server | `moonbook/server:243df5d566f3`，`sha256:b82bbd73a3a3252da8e964dc22d0d6ecb9a1fb94a0a9234550e1518c379741fb` |
| Web / Gateway | `moonbook/web:243df5d566f3`，`sha256:58498456d947f969aaddfef80fc046eb7c87f3f5a1b52a416fad57c9dc119214` |
| Reader | `moonbook/reader-ui:243df5d566f3`，镜像 ID `1764382789b3`（与既有 reader-ui 层缓存命中） |
| 归档 SHA-256 | `5289ec6e399d4c16e137a3cfbbc221c56e5140dfd4f1c04000c520f86ae80b31` |
| 数据库 | `moonbook_schema_version=78`，`sys_params.reader.invite.shareText` 已存在 |
| 旧镜像（回退） | `server:4e7954befb5c`、`web:4e7954befb5c`、`reader-ui:4e7954befb5c` |

发布脚本完成 preflight、上传校验、import、prepare、migrate、recreate、health。四个应用容器均为 healthy，restart count 为 0。

## 验证

- 源站 `http://127.0.0.1:18080/gateway-health`、`/api/health/ready`、`http://127.0.0.1:18081/health` 均为 HTTP 200。
- 公网管理首页、网关健康、API readiness、读者首页、读者健康均为 HTTP 200。
- 发布后 Server 日志精确 `error/fatal/panic` 计数为 0。

## 回退

回退前需确认迁移兼容（`00078` 为前向迁移，仅插入缺失参数）。命令：

```sh
ssh root@101.32.210.229 \
  "cd /opt/moonbook-app && docker compose --project-directory /opt/moonbook-app --env-file /opt/moonbook-app/.env -f /opt/moonbook-app/compose.yml -f /opt/moonbook-app/compose.release.previous.yml up -d --no-build --force-recreate --wait server web reader-ui gateway"
```

## 服务影响

已重建 `server`、`web`、`reader-ui`、`gateway`，并执行版本化 migrate。生产更新已完成，无需用户再重启。此后追加的发布记录属于文档变更，不影响服务，无需重启。
