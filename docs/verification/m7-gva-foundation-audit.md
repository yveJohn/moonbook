# M7 GVA 系统基座审计

审计日期：2026-08-15；复审完成：2026-08-21

## 结论

初审识别的配置 Secret 泄露、平台任务入口和范围不明确问题均已在 2026-08-21 收敛。GVA 管理基座的本地代码、迁移、真实 HTTP 和浏览器验收现已通过；旧通知公告、通用 OSS、SMTP 和在线会话 UI 已依据只读旧库与调用证据明确移出范围。完整证据见 `docs/verification/m7-gva-foundation-final.md`。

## 已有实现与证据

### 管理员认证与 RBAC

- 固定 GVA 提供验证码、登录、JWT、登出、管理员、角色、菜单、API 和 Casbin 管理能力。
- `00002_gva_foundation.sql` 与 `00003_gva_seed.sql` 纳入 GVA 表结构、菜单、API 和 Casbin 种子；空库迁移及种子哈希已在 M1 验证。
- `moonbook-admin bootstrap` 显式创建首管理员，强制首次改密、真实登录、非 root Server 和优雅停止已由 `make verify-m1` 验证。
- 账号锁定、密码过期和强制改密中间件有定向测试；严格 CORS 已有真实运行证据。

最终隔离栈已复跑验证码登录、首次强制改密、RBAC 越权、JWT 黑名单撤销、敏感响应和关键浏览器旅程。

### 组织、岗位、字典与参数

- 新系统包含部门、岗位、字典、字典详情和参数的后端 API、Service、管理页面、表结构、菜单及超级管理员 Casbin 权限。
- 部门和定时任务等局部模块有单元测试，M1 空库迁移证明相关表和种子可创建。
- 旧管理员、角色、组织和框架配置按批准边界不迁移，生产环境应在新 GVA 模型内重新初始化。

最终真实 PostgreSQL HTTP 验收已覆盖部门、岗位、字典、参数、角色策略和数据权限查询；浏览器动态菜单和页面切换正常。

### 审计日志与定时任务

- GVA 登录流程会向 `sys_login_logs` 写入成功和失败记录；管理端存在登录日志页面和查询路由。
- 管理写路由普遍使用 `OperationRecord`，操作记录页面、API 和表结构存在；Moonbook 多个业务 E2E 已抽样验证写操作记录。
- GVA 定时任务、执行日志和文件日志查看器存在；定时任务在启动及配置重载时恢复，相关 Runner/HTTP 安全测试存在。
- Moonbook `platform_jobs` 已实现持久化、租约、重试、恢复和 Prometheus 汇总指标。

最终 HTTP 验收覆盖登录日志、操作日志、数据权限日志和定时任务；浏览器已打开操作历史与定时任务页面。`platform_jobs` 已提供统一只读列表、筛选、详情与 attempt 审计入口，业务重试仍由各领域状态机负责。

## 已关闭的生产阻断安全问题

### 通用系统配置 API 回显并可改写秘密

固定 GVA 菜单公开“配置文件”页面，并为多个内置角色种下：

- `POST /system/getSystemConfig`
- `POST /system/setSystemConfig`

历史基线中的 `SystemConfigService.GetSystemConfig` 直接返回完整 `global.GVA_CONFIG`，曾导致 JSON 回显 JWT signing key、Redis 密码、数据库密码、邮件 Secret、MinIO `access-key-secret` 以及其他对象存储凭据；管理页面也直接把这些字段绑定到可编辑输入框。

Compose 容器启动时由环境变量把模板渲染到 `/run/moonbook/config.yaml`，应用再从该文件读取 Secret。通用配置 API 既能通过响应泄露已注入秘密，也能调用 Viper `WriteConfig` 改写运行配置文件，违反以下已批准边界：

- 密码、Token、支付凭据和第三方密钥只能由环境变量或 Secret 注入；
- 管理响应不得回显敏感值；
- 配置和 Secret 生命周期必须与镜像及业务数据分离。

该缺口已修复：`GetSystemConfig` 改为逐字段白名单，只返回非敏感运行参数和 `*-configured` 状态；`setSystemConfig` 路由已移除，服务层也拒绝写入，管理页面改为只读并保留重载入口。单元测试、真实管理员 HTTP、桌面/移动浏览器和迁移 68 的空库、66→68、67→68、幂等重放均已通过。

## 范围处置结论

### 在线管理员状态

旧 RuoYi 的在线用户列表和强制退出依赖瞬态框架会话，没有独立业务事实或历史迁移价值，明确移出范围。新系统保留 JWT 注销黑名单、账号禁用、登录日志和操作审计，不新增在线会话 UI。

### 平台任务监控

`platform_jobs` 和 `platform_job_attempts` 已提供平台级只读 API 与管理页面，支持 module、job type、status、lease owner 和时间筛选，展示脱敏失败信息、租约与 attempt 历史；Long ID 保持字符串。页面没有任何写路由，人工重跑继续由业务模块负责。

## 已移除的旧框架能力

### 通知公告

旧库 `sys_notice` 只有 2 条、创建时间完全相同，均为框架初始化公告；冻结仓库只发现框架页面，没有 Moonbook 业务调用。通知公告明确移出范围，不迁移这两条初始化数据。

### 通用 OSS 管理

旧 RuoYi 有通用 OSS 配置、上传、列表、下载和删除页面。批准设计已经决定不迁移旧框架 OSS 配置，并由固定 MinIO、业务对象引用、大小/SHA-256 校验和延迟回收替代。

旧库 `sys_oss` 为 0 行，`sys_oss_config` 5 行均为框架配置。通用 OSS 管理明确移出范围；章节、封面、TXT 和 AI 结果继续使用 MinIO 业务对象闭环及受控运维工具，管理 API 不保存或回显 MinIO Secret。

### SMTP 邮件

旧代码仅在框架登录、工作流和 Demo 中引用 SMTP，没有 Moonbook 业务调用或专用数据。邮件插件注册已删除，迁移 `00068` 精确清理两个邮件 API 及 Casbin 策略，真实路由返回 404；SMTP 明确移出范围。

## M7 退出门

1. `[x]` 关闭或白名单化通用系统配置 API，自动化测试证明响应不含任何 Secret 且 API 不能更新 Secret；当前代码见 `server/service/system/sys_system.go`、`server/router/system/sys_system.go`。
2. `[x]` 已完成管理员认证、RBAC、组织、岗位、字典、参数、登录日志、操作日志、定时任务和数据权限的真实 PostgreSQL/Redis HTTP 与浏览器验收。
3. `[x]` 已实现平台任务只读监控和 attempt 审计，业务状态变化仍走各模块服务。
4. `[x]` 在线状态、通知公告、通用 OSS 管理和 SMTP 均有旧库/代码证据及明确移除结论。
5. `[x]` 功能矩阵已链接最终报告或直接证据，不再以含糊状态描述 GVA 基座。
