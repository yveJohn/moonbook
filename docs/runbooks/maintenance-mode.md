# Compose 维护模式与协调停写

本 Runbook 用于单机 Compose 的可验证停写。它只操作显式环境文件对应的项目；生产执行仍需用户批准。维护模式不等于备份：只有网关已返回 503、应用写入方全部停止且外部回调处置已确认后，才能创建 PostgreSQL 与 MinIO 协调快照。

## 前置条件

- 记录发布 commit、环境文件绝对路径、项目名、操作人、开始时间和批准单。
- 确认 EPUSDT 对非 2xx 回调会重试，并记录重试上限；不能证明时必须暂停支付入口或准备按 `epusdt-callback.md` 人工核对，禁止进入正式停写。
- 确认没有 Compose 之外的 Server、Worker、迁移命令或管理员脚本连接同一 PostgreSQL/MinIO。
- 确认 PostgreSQL、Redis、MinIO healthy，当前不存在迁移和长事务。

先只读确认目标项目：

```bash
export MOONBOOK_ROOT=/path/to/moonbook
export MOONBOOK_ENV_FILE=/secure/path/moonbook.env
docker compose --project-directory "$MOONBOOK_ROOT" \
  --env-file "$MOONBOOK_ENV_FILE" -f "$MOONBOOK_ROOT/compose.yaml" \
  config --format json | node -e '
    let s="";
    process.stdin.on("data", d => s += d);
    process.stdin.on("end", () => process.stdout.write(JSON.parse(s).name));
  '
```

不得启用 `set -x`。把输出项目名作为显式确认值，进入维护：

```bash
"$MOONBOOK_ROOT/scripts/maintenance-mode.sh" on \
  --env-file "$MOONBOOK_ENV_FILE" \
  --confirm-project '<上一步项目名>'
```

脚本先仅重建 Gateway 为维护配置，验证管理端、Reader 和 API 均返回 `503` 且带 `Retry-After: 300`，随后优雅停止 `server`、`reader-ui`、`web`。`/gateway-health` 始终返回 200，便于负载均衡器区分“已维护”与“网关故障”。PostgreSQL、Redis、MinIO 不停止，支付回调也返回 503，不能在维护期间写库。

## 停写验收

```bash
"$MOONBOOK_ROOT/scripts/maintenance-mode.sh" status --env-file "$MOONBOOK_ENV_FILE"
docker compose --project-directory "$MOONBOOK_ROOT" \
  --env-file "$MOONBOOK_ENV_FILE" -f "$MOONBOOK_ROOT/compose.yaml" ps
```

必须同时满足：状态为 `maintenance`；`server`、`reader-ui`、`web` 为 stopped；Gateway healthy；PostgreSQL、Redis、MinIO healthy；无 `migrate`、`admin-bootstrap` 或外置 Worker 在运行。记录最后成功支付回调时间、待处理回调数、任务队列和数据库活动快照。然后才按 `backup-restore.md` 依次执行 PostgreSQL dump、MinIO mirror 和联合校验。

## 退出维护

只有迁移、核对和 Go/No-Go 负责人批准后才能恢复：

```bash
"$MOONBOOK_ROOT/scripts/maintenance-mode.sh" off \
  --env-file "$MOONBOOK_ENV_FILE" \
  --confirm-project '<项目名>'
```

脚本先启动并等待 Server、管理端和 Reader，再用正常配置重建 Gateway，最后验证两个入口。恢复后检查支付回调重投、Worker 租约、失败任务、财务核对和告警。退出失败时保持维护配置，不要绕过脚本直接开放入口。

## 失败与取证

- 切换维护配置失败：应用仍运行，不得开始备份；修复 Gateway 后重试。
- Gateway 已维护但应用停止失败：保持 503，检查容器和外置进程，直到所有写入方停止。
- 退出维护失败：保持数据服务和现场，不得恢复流量；按 `incidents/application-unavailable.md` 排查。
- 所有操作记录 commit、镜像 digest、容器状态、HTTP 状态、停写起止、支付/任务积压和操作人，但不得记录 Secret。

本地独立复现使用 `scripts/verify-maintenance-mode.sh`。脚本只接受 `moonbook_verify_` 项目，动态选择回环端口，验证进入/退出与数据服务健康，并在退出时清理专用容器、网络和卷。
