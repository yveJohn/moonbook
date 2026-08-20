# M7 GVA 管理基座最终验收

验收日期：2026-08-21

## 结论

GVA 管理基座的本地实现、迁移、真实 PostgreSQL/Redis HTTP 和浏览器验收通过。系统配置只读且不回显 Secret，平台任务具备统一只读审计入口；旧通知公告、通用 OSS、SMTP 和在线会话 UI 已按旧库与调用证据明确移出范围。

本报告关闭管理基座工作包，不代表 M4-M7 其他工作包或生产切换已经完成。

## 范围结论

| 能力 | 旧系统证据 | 结论 |
| --- | --- | --- |
| 通知公告 | `sys_notice` 2 行，同一初始化时间；仅框架页面和初始化公告 | 不迁移，移出范围 |
| 通用 OSS | `sys_oss` 0 行；`sys_oss_config` 5 行且均为框架配置 | 不迁移通用管理，保留业务 MinIO 对象闭环 |
| SMTP | 仅框架登录、工作流和 Demo 引用，无 Moonbook 业务调用 | 删除插件注册、API 元数据与权限，移出范围 |
| 在线会话 UI | 瞬态框架会话列表/强退，无独立业务事实 | 移出范围；保留 JWT 黑名单、账号禁用和审计 |

旧库检查为只读聚合查询，报告不记录公告正文、凭据或其他敏感值。

## 自动化证据

- 后端聚焦测试：迁移包、系统配置白名单/拒写、管理基座 integration 编译均通过。
- 前端：31 个 Node 测试、全量 ESLint、生产构建和供应链检查通过。
- 管理 HTTP：真实验证码登录、首次强制改密、重新登录、角色/Casbin、部门、岗位、字典、参数、登录日志、操作日志、定时任务、数据权限、配置白名单、配置拒写、受限角色拒绝、注销和 JWT 黑名单撤销全部通过。
- HTTP 测试使用隔离账号、短 TTL 一次性验证码和测试 Redis；账号、关联、登录/操作/数据权限日志及测试 Token 在清理阶段删除。

## 迁移与路由

- `00068_remove_unused_email_plugin.sql` 只删除 `/email/emailTest`、`/email/sendEmail` 的 API 元数据和 POST Casbin 策略。
- 真实 PostgreSQL 17 覆盖空库 0→68、66→68、67→68 和重复执行；三个场景均通过，重放应用数为 0。
- 最终库版本为 68，邮件 API 元数据 0、Casbin 策略 0；两个邮件路由真实请求均返回 HTTP 404。

## 浏览器证据

- 动态菜单可打开系统配置、操作历史、定时任务和平台任务；定时任务页显示 2 条种子任务。
- 系统配置页只显示应用、认证/指标、PostgreSQL、Redis/MinIO、网络/日志的安全值及“已配置”状态，正文区域没有输入框；重载操作有二次确认。
- 桌面 `1200x900` 与移动 `390x844` 均无横向溢出；移动端 `innerWidth`、document 和 body scroll width 均为 390。
- 页面正文包含 6 个“已配置”状态，不包含注入的测试 Secret；控制台 0 error、0 warning。
- 截图保存在 Git 忽略目录 `output/playwright/gva-foundation-config-desktop.png` 和 `output/playwright/gva-foundation-config-mobile.png`。

## 服务影响

- `server`：配置响应范围和插件注册变化，需要重启。
- `migrate`：新增迁移 68，部署时必须先运行一次迁移任务。
- `web`：系统配置页改为安全只读页面，需要重新构建并替换静态资源。
- PostgreSQL、Redis、MinIO、Reader UI 和 Gateway 无代码变更；仅为加载新应用镜像或依赖顺序按正常部署流程重建，无需单独配置变更。
