# Moonbook 读者 SEO 管理实施计划

## 1. 执行约束

- 权威规格：`docs/superpowers/specs/2026-08-14-moonbook-reader-seo-management-design.md`
- 旧仓库 `/Users/yve/Documents/moonbook` 仅供读取和提取兼容事实。
- 已执行迁移 `00001` 至 `00010` 不得修改；本切片只能新增 `00011`。
- M2 不注册 `/reader/seo/*`、robots 或 sitemap 公开路由。
- 每个逻辑单元验证后使用简体中文提交，禁止 push、reset 或 rebase。

## 2. 实施步骤

### 任务一：PostgreSQL 模型与管理后端

修改或新增：

- `server/internal/platform/migrate/migrations/00011_novel_reader_seo.sql`
- `server/internal/modules/novel/readerseo/types.go`
- `server/internal/modules/novel/readerseo/service.go`
- `server/internal/modules/novel/readerseo/http.go`
- 对应 Go 测试
- `server/initialize/router_biz.go`

步骤：

1. 用迁移建立固定 `id = 1` 的单例配置、正式默认值、菜单、GET/PUT API 元数据和超级管理员 Casbin 策略。
2. 先编写 URL、占位符、长度、空白和字符串 ID 的失败/边界测试。
3. 实现配置查询、事务更新、规范化和管理响应映射。
4. 注册私有管理路由，确认 PUT 进入现有操作审计。
5. 运行格式检查、受影响 Go 包测试、race 和 vet。

验收：空库默认配置正确；不能创建第二条配置；GET/PUT 合同、鉴权、权限、审计和错误映射有证据。

### 任务二：旧 MySQL 配置迁移

修改或新增：

- `server/internal/platform/legacymigrate/novel_reader_seo.go`
- 对应单元与集成测试、MySQL fixture
- `server/cmd/moonbook-legacy-migrate/main.go`
- `docs/migration/mapping.md`

步骤：

1. 注册 `novel-reader-seo` CLI 阶段并复用在线服务的规范化校验规则。
2. 实现固定记录读取、`0/1` 到布尔值转换、时间保留和事务覆盖。
3. 旧表缺失、固定记录缺失、额外 ID 或字段无效时保留默认配置并写入结构化错误清单。
4. 用真实 MySQL/PostgreSQL 覆盖有效迁移、错误分支、检查点和幂等重跑。
5. 验证源库只读、目标单例不增加、错误场景不发生部分覆盖。

验收：有效配置原值迁移；无效配置安全回退；相同输入重跑得到相同目标状态。

### 任务三：管理前端

修改或新增：

- `web/src/api/novel/readerSeo.js`
- `web/src/view/novel/readerSeo/index.vue`
- SEO 校验/预览辅助模块及测试
- Moonbook 前端测试入口中需要登记的测试文件

步骤：

1. 建立 GET/PUT API 封装，所有 ID 使用字符串。
2. 实现基础设置、页面文案和收录控制三个未嵌套区域。
3. 实现 URL、占位符即时校验及模板预览，服务端仍保留最终校验。
4. 覆盖加载、保存中、成功、失败和权限状态。
5. 执行相关 ESLint、`pnpm test:moonbook` 和生产构建。

验收：桌面和移动视口无文本溢出、遮挡或不可操作控件；API 载荷不发生 ID 精度转换。

### 任务四：真实闭环与 M2 退出审计

修改：

- `docs/inventory/admin-feature-matrix.md`
- `docs/migration/mapping.md`
- `docs/progress/refactor-status.md`
- 必要的验证证据文档

步骤：

1. 在隔离 PostgreSQL 从零执行到版本 11并验证重复执行 `applied=0`。
2. 启动隔离 API，完成 GVA 登录、首次改密、重新登录、未授权、Casbin、GET/PUT 和操作审计 HTTP E2E。
3. 执行真实 MySQL 到 PostgreSQL SEO 迁移集成测试。
4. 执行受影响 Go race/vet、管理前端测试/构建、Reader 零差异和秘密扫描。
5. 审计 M2 管理功能矩阵和全部退出条件；只有没有其他缺口时才标记 M2 完成。
6. 更新进度与证据，检查全部已跟踪和未跟踪文件，按逻辑提交。

验收：M2 每项交付物均有代码、测试或运行结果证据；若发现新缺口，继续补齐而不是提前进入 M3。

## 3. 提交边界

预期至少拆分为：

1. `规划读者SEO管理实施`
2. `实现读者SEO配置管理`
3. `实现旧SEO配置迁移`
4. `新增读者SEO管理页面`
5. `完成M2小说核心退出审计`

实际提交可按依赖和验证结果调整，但不得把无关修改合并为一个提交。
