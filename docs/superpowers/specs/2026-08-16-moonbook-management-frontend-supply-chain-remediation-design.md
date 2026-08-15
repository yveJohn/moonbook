# Moonbook 管理前端恶意构建依赖整改设计

## 1. 文档状态

- 日期：2026-08-16
- 状态：实施前书面规格
- 适用仓库：`/Users/yve/code/ai-project/moonbook`
- 目标：删除 `vite-vue-path-map@1.0.2` 及其生产构建注入入口，用仓库内可审计的确定性实现保留管理前端路径映射能力

本规格是 M7 供应链整改的第一个独立切片。它关闭已确认的 critical 恶意直接依赖，但不把管理前端其余 high 风险依赖、Server/Reader 依赖漏洞或三个基础镜像漏洞误记为已完成。

## 2. 已确认事实

当前管理前端存在以下确定风险：

1. `web/package.json` 直接依赖 `vite-vue-path-map@^1.0.2`，`web/pnpm-lock.yaml` 固定解析为 `1.0.2`；
2. `web/vite.config.js` 在每次启动和生产构建时调用该插件；
3. `web/vitePlugin/secret/index.js` 通过 `AddSecret()` 设置 `gva-project-name` 和 `gva-secret` 隐藏全局变量，仅供该插件的授权/注入逻辑读取；
4. 插件合法职责只是扫描 `src/view` 与 `src/plugin` 下的 Vue 文件并生成 `src/pathInfo.json`；
5. 插件的 `generateBundle` 会向生产 JavaScript 插入远程请求、`document.open()` 和 `document.write()` 逻辑；
6. 当前 `web/dist` 已命中这些注入特征，因此旧产物和旧管理端镜像不可信；
7. 当前仓库有 133 个目标 Vue 文件，`src/pathInfo.json` 恰好有 133 个映射，无缺失和多余项。

`pathInfo.json` 被动态路由 keep-alive、菜单组件名推导和组件级联选择器使用。整改必须保留这三个运行时调用面的路径与组件名语义，不能简单删除插件或改为空映射。

## 3. 范围

### 3.1 本切片包含

- 仓库内确定性路径映射生成器；
- Vite 启动、构建和开发期文件监听适配；
- 删除恶意包、锁文件条目、`AddSecret()` 调用和专用隐藏全局变量实现；
- 生成器、Vite 适配和当前仓库映射一致性测试；
- 源码、依赖声明、锁文件、安装树和生产产物的恶意特征扫描；
- CI 管理端构建后的供应链检查；
- 重新安装管理端依赖、生成可信生产产物和管理端镜像；
- 管理端现有单元测试、lint、生产构建和关键浏览器旅程回归；
- M7 安全、CI、进度和最终证据更新。

### 3.2 本切片不包含

- 批量升级管理端其余依赖；
- 修改冻结 Reader 业务代码或依赖；
- 升级 Go 工具链、Go 模块或基础镜像；
- 关闭 GVA 商业授权、真实第三方凭据或生产环境验收项；
- 连接生产环境、执行生产迁移、切流或 `git push`。

这些事项继续按 M7 独立切片处理。本切片完成后，生产发布仍保持 No-Go，直到全部安全退出门关闭。

## 4. 方案选择

采用仓库内 Vite 插件方案，而不是构建前脚本或运行时 `import.meta.glob`：

- 构建前脚本无法自然保持开发期新增、删除、重命名 Vue 文件后的即时映射更新；
- 运行时 glob 需要改动路由、菜单和 keep-alive 消费者，扩大业务回归面；
- 本地 Vite 插件可以保持现有 `pathInfo.json` 合同，同时把扫描、解析和写入逻辑放在仓库内接受代码审查和测试。

生成器与 Vite 生命周期适配器必须分离。生成器不依赖 Vite 全局状态，输入为项目根目录与输出路径，输出为确定性映射和是否写入；Vite 适配器只负责调用时机与开发期监听。

## 5. 组件设计

### 5.1 路径映射生成器

新增 `web/vitePlugin/pathMap/generator.js`，职责固定为：

1. 递归扫描 `<root>/src/view` 和 `<root>/src/plugin`；
2. 只接受普通 `.vue` 文件，不跟随目录符号链接；
3. 使用 `@vue/compiler-sfc` 解析单文件组件边界，再使用 `@babel/parser` 将 `script setup` 和普通 `script` 解析为 JavaScript/TypeScript AST；
4. 从 AST 中优先提取静态字符串形式的 `defineOptions({ name: '...' })`；
5. 未声明静态组件名时，按文件名生成 PascalCase 回退值；
6. 将文件路径规范化为 `/src/...` 的 POSIX 形式；
7. 按完整路径字典序排序后生成两空格缩进 JSON，并以换行结尾；
8. 只有新内容与现有文件不同才写入，避免每次启动产生无意义 Git 修改；
9. 通过同目录临时文件加原子 rename 更新目标，避免进程中断留下半写 JSON。

静态名称只接受非空 JavaScript 字符串字面量。动态表达式、模板插值、同一文件中的多个 `defineOptions.name` 声明或无法解析的 SFC 必须返回带相对文件路径的明确错误并阻止启动/构建，不能静默回退后产生错误 keep-alive 名称。不同文件使用相同组件名是现有 GVA 页面中的合法情况，不作为生成错误。

PascalCase 回退规则按连字符、下划线和空白分词并将每段首字母大写；已有驼峰文件名保持内部大小写。例如 `chapter-clean.vue` 映射为 `ChapterClean`，`scanUpload.vue` 映射为 `ScanUpload`。

### 5.2 Vite 生命周期适配器

新增 `web/vitePlugin/pathMap/index.js`，导出一个 Vite 插件：

- `configResolved` 保存规范化项目根目录；
- `buildStart` 在开发服务器和生产构建解析模块前全量生成映射；
- `configureServer` 复用 Vite 已有 watcher，监听两个目标目录中的 Vue 文件新增、修改、删除和重命名；
- 文件事件经短时间去抖后重新执行全量生成，避免编辑器一次保存触发重复写入；
- watcher 或生成失败通过 Vite logger 和开发服务器错误 overlay 报错，保留最后一次完整映射；生产构建直接失败；
- 开发服务器关闭时注销事件处理器和定时器，不遗留监听器。

适配器不得实现 `generateBundle`，不得读取或修改输出 chunk，不得执行网络请求，也不得读取未声明的全局变量。

### 5.3 Vite 配置与依赖

`web/vite.config.js` 改为导入本地 `pathMap` 插件。删除：

- `vite-vue-path-map` 导入；
- `AddSecret` 导入和调用；
- `web/vitePlugin/secret/index.js`；
- `package.json` 与 `pnpm-lock.yaml` 中的恶意包及其不再需要的传递项。

`@vue/compiler-sfc` 已是直接开发依赖。为避免用正则解释 JavaScript，`@babel/parser` 从现有传递依赖提升为直接开发依赖并锁定到当前兼容版本，不引入新的依赖家族。Vite watcher 由现有 Vite 实例提供。移除恶意包后 `chokidar` 不再被仓库代码直接导入，因此同时删除直接 `chokidar` 开发依赖；Vite 自己的传递依赖由锁文件正常解析。

### 5.4 供应链检查器

新增 `web/scripts/check-supply-chain.mjs`，以结构化解析和明确指纹检查以下层面：

- `package.json` 不得声明 `vite-vue-path-map`；
- `pnpm-lock.yaml` 不得包含 `vite-vue-path-map@` 或其完整包键；
- `vite.config.js` 和 `vitePlugin` 不得包含 `AddSecret`、`gva-project-name`、`gva-secret` 或恶意包导入；
- 安装树存在时不得解析出 `vite-vue-path-map`；
- 生产 `dist` 存在时不得出现已确认的远程地址明文/Base64 指纹、未授权替换页指纹、`document.open(` 或 `document.write(`；
- `dist` 必须存在至少一个 JavaScript 入口和生成的路径映射 chunk，避免对空目录误报通过。

检查器不使用网络，不读取秘密，不修改文件。任一命中必须打印检查层、相对文件和指纹名称并以非零状态退出；不输出整段压缩代码。

`package.json` 增加 `check:supply-chain`。CI 的管理端步骤调整为：固定锁文件安装、单元测试、lint、生产构建、供应链检查。供应链检查必须位于生产构建之后，确保扫描的是本次生成的产物。

## 6. 数据流与失败语义

### 6.1 启动和构建

```text
Vite buildStart
  -> 扫描 view/plugin
  -> 解析组件名
  -> 排序并序列化
  -> 内容变化时原子更新 pathInfo.json
  -> Vite 正常加载 JSON 与页面模块
  -> 生产构建
  -> 供应链检查扫描声明、锁文件、源码、安装树和 dist
```

扫描目录不存在、文件不可读、SFC 语法错误、组件名歧义、目标目录不可写或 rename 失败均阻止构建。继续使用旧映射会隐藏菜单和 keep-alive 错误，因此不允许失败后降级。

### 6.2 开发期更新

Vue 文件事件只触发本地全量重建。生成器不访问网络、不执行 Vue 文件代码，也不基于路径加载模块。写入 `pathInfo.json` 后由 Vite 正常触发模块更新；删除文件必须从映射中移除。

### 6.3 安全失败

锁文件或产物再次命中恶意特征时，CI 和本地验收立即失败。不得通过放宽指纹、跳过扫描、保留旧 `dist` 或仅在运行时封禁域名换取绿色结果。

## 7. 测试策略

### 7.1 生成器单元测试

新增 `web/test/pathMap.test.js`，使用临时目录覆盖：

- `view` 与 `plugin` 递归扫描；
- 单行、多行、普通 script 和 script setup 的静态 `defineOptions` 名称；
- 连字符、下划线和驼峰文件名回退；
- POSIX 路径、稳定排序和确定性 JSON；
- 新增、修改、删除文件后的映射变化；
- 内容相同时不重写；
- 无效 SFC、动态/空/重复名称和写入失败的明确错误；
- 符号链接目录不被跟随。

测试不得依赖已安装的恶意包或网络。

### 7.2 仓库一致性测试

同一测试读取当前 `web/src`，断言：

- 实际 Vue 文件集合与生成映射键集合完全一致；
- 生成结果与已提交 `src/pathInfo.json` 字节一致；
- 当前 133 个页面和插件组件仍全部存在于映射中；
- Moonbook 管理页面的显式组件名保持不变。

文件总数变化是允许的，但提交中必须同时更新映射，因此测试应输出实际数量而不是永久把 133 写成未来上限。

### 7.3 供应链检查测试

新增 `web/test/supplyChainCheck.test.js`，在隔离临时夹具中证明检查器会拒绝：

- 恶意包声明和锁文件条目；
- 隐藏全局变量入口；
- 明文或 Base64 远程地址；
- 生产 chunk 中的 `document.open()`/`document.write()`；
- 空或缺少 JavaScript 的 `dist`。

正常声明、锁文件和可信最小产物必须通过。

### 7.4 回归验证

实现完成后执行：

1. `pnpm install --frozen-lockfile`；
2. `pnpm run test:moonbook`；
3. `pnpm run lint`；
4. `pnpm run build`；
5. `pnpm run check:supply-chain`；
6. 重建管理端镜像并确认镜像内容来自新产物；
7. Playwright 验证管理员登录、动态菜单、Moonbook 主要菜单、页面切换和 keep-alive；
8. 重新执行秘密扫描、管理功能相关契约和 M1/CI 可信范围；
9. 对新管理端镜像执行漏洞扫描，确认恶意插件关闭，同时把其余漏洞继续记录为独立 No-Go。

生产构建前必须先删除或由 Vite `emptyOutDir` 清空旧 `dist`，避免旧注入 chunk 混入证据。构建产物继续受 `.gitignore` 管理，不提交到 Git。

## 8. 提交与证据

按逻辑拆分提交：

1. 设计规格；
2. 本地路径映射生成器、测试和锁文件；
3. 供应链检查器与 CI 门；
4. 验收证据与进度文档。

至少更新：

- `docs/verification/m7-security-supply-chain-audit.md`；
- `docs/verification/m7-ci-coverage-audit.md`；
- `docs/verification/m7-readiness-audit.md`；
- `docs/progress/refactor-status.md`；
- 后续统一验收计划或最终报告中的安全状态。

证据必须记录固定 commit、Node/pnpm 版本、命令、结果、产物哈希和管理镜像 digest。报告必须明确“恶意插件已关闭”和“其余安全退出门仍未关闭”两项结论。

## 9. 验收条件

本切片只有在以下条件全部满足后才完成：

- `package.json`、锁文件、源码和安装树不再包含 `vite-vue-path-map`；
- `AddSecret`、两个隐藏全局变量和 secret 插件文件全部删除；
- 本地生成器对当前全部 Vue 文件生成与业务合同一致的映射；
- 新增、修改、删除 Vue 文件在开发期和生产构建中正确更新映射；
- 生成器、供应链检查器、现有管理测试和 lint 全部通过；
- 新生产构建不含已确认注入特征；
- 管理端关键浏览器旅程和 keep-alive 行为无回归；
- CI 在生产构建后执行供应链检查；
- 新管理镜像与产物哈希可定位，旧管理镜像明确废弃；
- 其余 high/critical 和外部授权项继续作为 No-Go 记录；
- 所有有效修改按功能使用简体中文提交，工作区干净且未 push。

## 10. 服务影响

设计文档本身不影响服务，无需重启。实施完成后影响管理前端构建链和 `moonbook-web` 静态镜像；本地或部署环境需要重新构建并替换 `moonbook-web` 容器。Go 后端、应用内 Worker、PostgreSQL、Redis、MinIO 和冻结 Reader 不需要因本切片单独重启或迁移。
