# Reader 分类契约修复设计

## 1. 问题

生产 PostgreSQL 已包含主分类和子分类，Reader 分类接口也能查到非空分类名称，但接口把分类序列化为 `code`、`name`。冻结版 `reader-ui` 的 `ReaderCategory` 契约读取 `categoryCode`、`categoryName`，导致分类和标签按钮数量存在但文本为空。

同一个分类序列化函数还用于书籍摘要和详情的 `subCategories`，因此书籍标签也存在相同契约风险。

## 2. 修复边界

- 只修改 Reader 兼容 API 的分类 DTO，把字段统一为 `categoryCode`、`categoryName`。
- 分类列表、子分类列表及书籍 `subCategories` 复用同一序列化函数，避免字段再次分叉。
- 不修改冻结版 `reader-ui`，不重新迁移分类，不修改 PostgreSQL、Redis 或 R2 数据。
- 不同时保留错误的 `code`、`name` 别名，避免形成双重契约。

## 3. 验证与发布

- 单元测试断言分类 DTO 仅输出 `categoryCode`、`categoryName`。
- Reader HTTP 集成测试验证主分类、子分类和书籍详情标签字段。
- 运行 Reader public 模块测试和 Go 后端构建。
- 构建新的 `linux/amd64` Server 镜像，完成漏洞扫描后传输到生产服务器。
- 仅更新 `migrate`、`admin-bootstrap` 和 `server` 使用的 Server 镜像引用；迁移应幂等显示无待执行版本。
- 重建并重启生产 `server`，验证健康检查、两个分类接口、公网页面文字、MySQL 双只读和其他应用容器状态。

## 4. 回退

保留当前 `moonbook/server:c921a90` 镜像和生产 Compose 文件备份。若新镜像健康检查或 Reader 契约验收失败，将 Server 镜像引用恢复为 `c921a90` 并重建 `server`；不执行数据库回滚。
