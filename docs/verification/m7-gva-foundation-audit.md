# M7 GVA 系统基座审计

审计日期：2026-08-15

## 结论

Gin-Vue-Admin 固定基线已经完整导入，管理员认证、RBAC、组织、字典、参数、日志和定时任务等大部分源码、页面、路由、表结构及超级管理员种子权限均存在；但这不能直接证明旧 RuoYi 系统基座能力已经完成生产验收。

当前至少存在一个生产阻断安全问题、两个确定的管理能力缺口，以及多项仅有源码或分散测试而没有固定验收 commit 上运行证据的基座能力。管理功能矩阵中的六条 `基线已盘点` 因此不能直接改为“已完成”。

## 已有实现与证据

### 管理员认证与 RBAC

- 固定 GVA 提供验证码、登录、JWT、登出、管理员、角色、菜单、API 和 Casbin 管理能力。
- `00002_gva_foundation.sql` 与 `00003_gva_seed.sql` 纳入 GVA 表结构、菜单、API 和 Casbin 种子；空库迁移及种子哈希已在 M1 验证。
- `moonbook-admin bootstrap` 显式创建首管理员，强制首次改密、真实登录、非 root Server 和优雅停止已由 `make verify-m1` 验证。
- 账号锁定、密码过期和强制改密中间件有定向测试；严格 CORS 已有真实运行证据。

这些证据证明认证基座可运行，但 M7 仍需在最终验收 commit 上复跑 RBAC 越权、登录限流、会话撤销、敏感响应和浏览器关键管理旅程。

### 组织、岗位、字典与参数

- 新系统包含部门、岗位、字典、字典详情和参数的后端 API、Service、管理页面、表结构、菜单及超级管理员 Casbin 权限。
- 部门和定时任务等局部模块有单元测试，M1 空库迁移证明相关表和种子可创建。
- 旧管理员、角色、组织和框架配置按批准边界不迁移，生产环境应在新 GVA 模型内重新初始化。

当前没有覆盖部门、岗位、字典和参数 CRUD、角色授权及数据权限的统一真实 PostgreSQL HTTP/浏览器验收报告，不能只凭文件存在标记通过。

### 审计日志与定时任务

- GVA 登录流程会向 `sys_login_logs` 写入成功和失败记录；管理端存在登录日志页面和查询路由。
- 管理写路由普遍使用 `OperationRecord`，操作记录页面、API 和表结构存在；Moonbook 多个业务 E2E 已抽样验证写操作记录。
- GVA 定时任务、执行日志和文件日志查看器存在；定时任务在启动及配置重载时恢复，相关 Runner/HTTP 安全测试存在。
- Moonbook `platform_jobs` 已实现持久化、租约、重试、恢复和 Prometheus 汇总指标。

当前没有跨登录日志、操作日志、文件日志和定时任务页面的最终浏览器验收，也没有通用 `platform_jobs` 管理查询页面或 API。业务任务页面只能查看各自任务，不能替代平台级积压、失败、租约和 attempt 审计入口。

## 生产阻断安全问题

### 通用系统配置 API 回显并可改写秘密

固定 GVA 菜单公开“配置文件”页面，并为多个内置角色种下：

- `POST /system/getSystemConfig`
- `POST /system/setSystemConfig`

历史基线中的 `SystemConfigService.GetSystemConfig` 直接返回完整 `global.GVA_CONFIG`，曾导致 JSON 回显 JWT signing key、Redis 密码、数据库密码、邮件 Secret、MinIO `access-key-secret` 以及其他对象存储凭据；管理页面也直接把这些字段绑定到可编辑输入框。

Compose 容器启动时由环境变量把模板渲染到 `/run/moonbook/config.yaml`，应用再从该文件读取 Secret。通用配置 API 既能通过响应泄露已注入秘密，也能调用 Viper `WriteConfig` 改写运行配置文件，违反以下已批准边界：

- 密码、Token、支付凭据和第三方密钥只能由环境变量或 Secret 注入；
- 管理响应不得回显敏感值；
- 配置和 Secret 生命周期必须与镜像及业务数据分离。

该缺口已在当前实现修复：`GetSystemConfig` 改为逐字段白名单，只返回非敏感运行参数和 `*-configured` 状态；`setSystemConfig` 路由已移除，服务层也拒绝写入，管理页面改为只读并保留重载入口。新增 `TestPublicSystemConfigDoesNotExposeSecrets` 和 `TestSetSystemConfigIsRejected` 覆盖值泄漏与写入拒绝，`00066_disable_system_config_write.sql` 清理历史 API/策略元数据。隔离本地 PostgreSQL 已完成 `current=0 target=66` 的首次迁移，重复执行 `applied=0`、`current=66 target=66 pending=false`，残留 API/策略均为 0。固定验收 commit 上仍需补充真实管理员 HTTP/浏览器旅程，不能仅凭单元测试关闭整个 M7。

## 确定缺口

### 在线管理员状态

旧 RuoYi 提供在线用户列表、当前设备列表和强制退出接口。新系统没有等价管理页面或路由；SSE Hub 的在线计数不是管理员会话清单，也不能执行会话撤销。

若该旧入口被确认属于当前有效运维能力，需要基于管理员 JWT/黑名单事实实现只读会话审计和受审计撤销；若没有实际使用证据，应在功能矩阵中明确批准移除，不能继续保持含糊状态。

### 平台任务监控

`platform_jobs` 和 `platform_job_attempts` 是 Moonbook 长任务的统一事实表，但当前没有平台级管理 API 或页面。M7 的任务监控要求至少需要按 module、job type、status、lease owner 和时间筛选，并能查看有限脱敏的失败代码、失败消息、尝试次数、租约及 attempt 历史。人工重跑继续由各业务模块负责，平台页不应绕过业务状态机直接修改任务。

## 需要范围确认的旧框架能力

### 通知公告

旧 RuoYi 有通知公告 CRUD 页面和 Controller。冻结仓库搜索只发现框架系统页面，没有 Moonbook 业务模块调用或专门设计证据；固定 GVA 基线没有等价通知公告模块。

该能力应由旧系统实际使用数据或用户确认决定：有有效公告数据或运营使用证据则迁移为 GVA 管理能力；否则记录为未使用框架能力并明确移出范围。

### 通用 OSS 管理

旧 RuoYi 有通用 OSS 配置、上传、列表、下载和删除页面。批准设计已经决定不迁移旧框架 OSS 配置，并由固定 MinIO、业务对象引用、大小/SHA-256 校验和延迟回收替代。

新系统已经完成章节、封面、TXT 和 AI 结果等业务对象闭环，但没有通用对象浏览器。M7 需要确认生产运维是否只使用 MinIO Console/受控脚本，还是需要只读对象审计页；无论选择哪种方式，都不能恢复在数据库中保存或通过管理 API 回显 MinIO Secret 的旧配置模式。

### SMTP 邮件

旧框架存在邮件配置及相关插件线索，但当前仓库没有证明 Moonbook 生产正在使用 SMTP 的脱敏环境或业务调用证据。该项仍属于第三方集成待确认清单，不能因 GVA 配置结构含有 email 字段而视为已经迁移。

## M7 退出门

1. `[x]` 关闭或白名单化通用系统配置 API，自动化测试证明响应不含任何 Secret 且 API 不能更新 Secret；当前代码见 `server/service/system/sys_system.go`、`server/router/system/sys_system.go`。
2. 在固定验收 commit 上执行管理员认证、RBAC、组织、岗位、字典、参数、登录日志、操作日志、定时任务和数据权限的真实 PostgreSQL HTTP/浏览器验收。
3. 实现平台任务只读监控和 attempt 审计，或提供同等可复现的受控运维入口；业务状态变化仍走各模块服务。
4. 对在线状态、通知公告、通用 OSS 管理和 SMTP 分别补充实际使用证据及“实现/明确移除”结论。
5. 功能矩阵每一条都链接到代码、测试、运行报告或批准的移除记录，不再保留 `基线已盘点`。
