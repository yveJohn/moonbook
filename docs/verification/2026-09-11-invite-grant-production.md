# 邀请码与读者发放功能生产发布

用户于本次任务明确授权本地构建并发布生产。发布在 **2026-09-11 12:23:54—12:23:55（UTC+8）** 完成，固定提交 `4e7954befb5c729ea5015f92b3f2562afb6fe0db`。

## 范围

本次镜像包含：

- 新生成读者邀请码改为 8 位大写字母或数字；已有邀请码不改写。
- 读者用户页按商品发放会员，以及直接发放钻石/金币。
- 管理后台业务枚举中文显示补全。

本次 Git 范围无新增迁移文件，未修改生产 Secret、DNS、证书、支付或业务数据。

## 发布结果

| 项目 | 结果 |
| --- | --- |
| 生产目标 | `101.32.210.229:/opt/moonbook-app` |
| Server | `moonbook/server:4e7954befb5c`，`sha256:65822c7a85a7c50a2d0aa94757434c7dafc927565e0cb0420524fd0ff9230285` |
| Web / Gateway | `moonbook/web:4e7954befb5c`，`sha256:fbe31b2e14b3131fee83e71b2af4a3d4feb7bff818eaeab059841f6c7000ecd2` |
| Reader | `moonbook/reader-ui:4e7954befb5c`，镜像 ID `1764382789b3`（与既有 reader-ui 层缓存命中） |
| 归档 SHA-256 | `9602246b39548a246ccf3839e86a9d2beea7405cbc14339638ba0476bb2322af` |
| 旧镜像（回退） | `server:0bb1fdbd9ac1`、`web:67841f1faf56`、`reader-ui:0b6b6e003b42` |

发布脚本完成 preflight、上传校验、import、prepare、migrate、recreate、health 与临时目录清理。四个应用容器均为 healthy，restart count 为 0。

## 验证

- 源站 `http://127.0.0.1:18080/gateway-health`、`/api/health/ready`、`http://127.0.0.1:18081/health` 均为 HTTP 200。
- 公网管理首页、网关健康、API readiness、读者首页、读者健康均为 HTTP 200。
- 发布后约 3 分钟内 Server 日志精确 `error/fatal/panic` 计数为 0。

## 回退

回退前需确认迁移兼容（本次无新业务迁移）。命令：

```sh
ssh root@101.32.210.229 \
  "cd /opt/moonbook-app && docker compose --project-directory /opt/moonbook-app --env-file /opt/moonbook-app/.env -f /opt/moonbook-app/compose.yml -f /opt/moonbook-app/compose.release.previous.yml up -d --no-build --force-recreate --wait server web reader-ui gateway"
```

## 服务影响

已重建 `server`、`web`、`reader-ui`、`gateway`。生产更新已完成，无需用户再重启。此后追加的发布记录属于文档变更，不影响服务，无需重启。
