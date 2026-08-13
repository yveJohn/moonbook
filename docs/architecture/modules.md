# Go 模块边界

Moonbook 在单个 Go 进程和单个 PostgreSQL 数据库中按业务能力划分模块，不拆分微服务。业务代码放在 `server/internal/modules/`：

| 模块 | 所有权 |
| --- | --- |
| `novel` | 分类、作者、书籍、章节、标签与发布 |
| `reader` | 读者身份、书架、阅读行为与兼容 API |
| `commerce` | 钱包、流水、订单、支付与权益 |
| `production` | 采集、导入、合并、AI 与内容任务 |

`server/internal/platform/` 只提供数据库迁移、对象存储、缓存、任务运行、可观测性等平台能力。GVA 的路由和初始化层是组合根，负责把管理基座与 Moonbook 模块装配起来。

## 依赖规则

- 业务模块拥有自己的数据访问、应用服务和领域类型，不允许通过 `global`、GVA `model` 或 GVA `service` 包读取其他模块数据。
- 模块间协作只能依赖目标模块的 `contract` 包；合同只包含稳定接口和跨模块 DTO，不暴露数据库模型。
- 管理 API 与读者兼容 API 在组合根使用不同路由组、DTO 和响应封装。
- 平台包不得依赖业务模块，业务模块可以依赖平台公开接口。
- Redis 只保存可重建数据；持久任务和迁移检查点以 PostgreSQL 为准。

`server/internal/modules/dependency_test.go` 扫描生产代码 import，并在违反上述核心依赖方向时使测试失败。新增模块必须同步更新本表和测试覆盖。
