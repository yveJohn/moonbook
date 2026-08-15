# Moonbook 管理前端恶意构建依赖整改实施计划

> **目标：** 删除 `vite-vue-path-map@1.0.2` 和隐藏注入入口，以仓库内确定性路径映射实现保持管理菜单、动态路由和 keep-alive 行为，并建立生产产物供应链检查门。

**设计规格：** `docs/superpowers/specs/2026-08-16-moonbook-management-frontend-supply-chain-remediation-design.md`

**技术栈：** Node.js 22、pnpm 10.15.1、Vite 8、Vue 3、`@vue/compiler-sfc`、`@babel/parser`、Node test、Playwright、Docker Compose。

## 全局约束

- [ ] 只修改活跃仓库 `/Users/yve/code/ai-project/moonbook`，冻结仓库和 `reader-ui` 不修改。
- [ ] 不执行生产连接、生产迁移、切流、真实支付或 `git push`。
- [ ] 先写失败测试并确认失败原因，再做最小实现；不得删除、跳过或放宽既有测试。
- [ ] 依赖安装固定使用 pnpm 10.15.1 和锁文件；不得使用 `--force`、忽略脚本或手工伪造锁文件。
- [ ] 旧 `web/dist` 不是可信证据；新构建必须清空输出目录并由供应链检查器验证。
- [ ] 每个逻辑单元检查全部 tracked/untracked 修改、秘密和暂存区后使用简体中文提交，最终工作区干净且未 push。
- [ ] 本切片只关闭恶意直接依赖；其余 high/critical、基础镜像和外部授权继续明确 No-Go。

## Task 1：固定路径映射生成合同

**文件：**

- Create: `web/test/pathMap.test.js`
- Reference: `web/src/pathInfo.json`
- Reference: `web/src/pinia/modules/router.js`
- Reference: `web/src/view/superAdmin/menu/menu.vue`
- Reference: `web/src/view/superAdmin/menu/components/components-cascader.vue`

- [ ] **Step 1：临时目录测试先失败。** 覆盖 `view/plugin` 递归扫描、静态 `defineOptions.name`、文件名回退、POSIX 路径和稳定排序；因生成器不存在而确定失败。
- [ ] **Step 2：错误合同测试先失败。** 覆盖无效 SFC、动态/空名称、同文件多个名称、符号链接目录和写入失败；错误必须含相对文件且不输出源码正文。
- [ ] **Step 3：当前仓库一致性测试先失败。** 证明实际 Vue 文件集合、生成映射和已提交 `pathInfo.json` 必须字节一致，并固定 Moonbook 页面名称。
- [ ] **Step 4：记录失败原因。** 失败只能是本地生成器尚不存在，不得由缺依赖、网络或旧恶意插件执行造成。

## Task 2：实现确定性路径映射生成器

**文件：**

- Create: `web/vitePlugin/pathMap/generator.js`
- Modify: `web/src/pathInfo.json`

- [ ] **Step 1：安全扫描。** 使用 `lstat/readdir` 且不跟随目录符号链接，只读取两个批准目录中的普通 `.vue` 文件。
- [ ] **Step 2：结构化解析。** `@vue/compiler-sfc` 解析 SFC，`@babel/parser` 解析 JS/TS AST；只接受非空静态字符串 `defineOptions.name`。
- [ ] **Step 3：确定性输出。** 路径规范化为 `/src/...`，键排序，两空格 JSON 加末尾换行；内容不变不写入。
- [ ] **Step 4：原子更新。** 同目录临时文件写入后 rename，失败清理临时文件并保留最后完整目标。
- [ ] **Step 5：测试转绿。** 运行 `node --test test/pathMap.test.js`，确认全部合同通过且不执行网络。

## Task 3：接入本地 Vite 插件

**文件：**

- Create: `web/vitePlugin/pathMap/index.js`
- Create: `web/test/pathMapPlugin.test.js`
- Modify: `web/vite.config.js`

- [ ] **Step 1：生命周期测试先失败。** 使用 Vite 适配替身固定 `configResolved`、`buildStart`、watcher 增删改、去抖和关闭清理行为。
- [ ] **Step 2：实现适配器。** 构建前同步生成，开发期复用 Vite watcher；错误进入 logger/overlay，生产构建失败。
- [ ] **Step 3：固定禁止面。** 本地插件不得暴露 `generateBundle`，不得修改 chunk、访问网络或读取隐藏全局变量。
- [ ] **Step 4：替换配置。** `vite.config.js` 只调用本地插件，保持输出目录和现有插件顺序中的业务语义。
- [ ] **Step 5：定向测试转绿。** 运行路径映射和插件生命周期测试。

## Task 4：删除恶意依赖与隐藏入口

**文件：**

- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Delete: `web/vitePlugin/secret/index.js`
- Modify: `web/vite.config.js`

- [ ] **Step 1：依赖边界。** 删除 `vite-vue-path-map` 和仓库不再直接使用的 `chokidar`；把现有传递依赖 `@babel/parser` 提升为直接开发依赖。
- [ ] **Step 2：删除注入入口。** 删除 `AddSecret()`、`gva-project-name`、`gva-secret` 和 secret 插件文件。
- [ ] **Step 3：正规更新锁文件。** 使用 pnpm 更新并执行 `pnpm install --frozen-lockfile` 证明锁文件可复现。
- [ ] **Step 4：依赖树核对。** `pnpm why vite-vue-path-map` 无结果；`pnpm why @babel/parser` 只能定位可信直接/现有传递依赖。
- [ ] **Step 5：提交。** 暂存生成器、Vite 适配、测试、配置、路径映射和锁文件，提交信息：`替换管理前端恶意路径插件`。

## Task 5：建立供应链检查器

**文件：**

- Create: `web/scripts/check-supply-chain.mjs`
- Create: `web/test/supplyChainCheck.test.js`
- Modify: `web/package.json`

- [ ] **Step 1：夹具测试先失败。** 固定恶意声明、锁文件、隐藏全局、明文/Base64 远程指纹、未授权页、`document.open/write` 和空 dist 的拒绝语义。
- [ ] **Step 2：实现结构化检查。** 解析 package JSON，检查锁文件键、批准源码目录、可选安装树和本次生产 dist；只输出相对文件与指纹名。
- [ ] **Step 3：可信夹具转绿。** 正常声明、锁文件和最小生产产物通过；检查器不联网、不修改文件、不打印压缩代码。
- [ ] **Step 4：添加命令。** `pnpm run check:supply-chain` 对真实仓库执行，缺少/空生产 dist 明确失败。

## Task 6：把检查门接入 CI

**文件：**

- Modify: `.github/workflows/ci.yml`
- Modify: `Makefile` 或现有统一验证脚本（仅在能复用而不重复逻辑时）

- [ ] **Step 1：管理端 CI 顺序。** 固定锁文件安装、`test:moonbook`、lint、生产构建、供应链检查，检查必须扫描本次 build 输出。
- [ ] **Step 2：失败闭环。** 临时注入每类禁止指纹时测试/检查确定失败，恢复夹具后转绿。
- [ ] **Step 3：本地等价命令。** 仓库提供与 CI 同顺序的可重复入口，不隐藏单步失败。
- [ ] **Step 4：提交。** 暂存检查器、测试、package 脚本和 CI，提交信息：`增加管理前端供应链安全门`。

## Task 7：可信管理端构建与功能回归

**文件：**

- Generated and ignored: `web/dist/**`
- Generated and ignored: `web/node_modules/**`
- Rebuilt image: `moonbook/web:local`

- [ ] **Step 1：测试与 lint。** 运行 `pnpm run test:moonbook` 和 `pnpm run lint`，不得排除旧测试或新增测试。
- [ ] **Step 2：可信生产构建。** 确认 Vite 清空旧 dist 后运行 `pnpm run build`；记录 Node、pnpm、Vite 版本和构建摘要。
- [ ] **Step 3：供应链门。** 运行 `pnpm run check:supply-chain`，确认声明、锁文件、源码、安装树和 dist 全部通过。
- [ ] **Step 4：产物定位。** 计算 dist 确定性清单哈希，记录主要入口和路径映射 chunk；禁止提交 dist。
- [ ] **Step 5：镜像重建。** 使用仓库 Dockerfile 重建 `moonbook/web:local`，记录 image ID/digest，并证明容器静态文件与本次 dist 对应。
- [ ] **Step 6：Playwright。** 在隔离 Compose 栈验证管理员登录、动态菜单、Moonbook 主要菜单、页面切换、刷新和 keep-alive，桌面和移动视口控制台 0 error。

## Task 8：重新执行安全与项目质量门

**文件：**

- Reference: `scripts/scan-secrets.sh`
- Reference: `scripts/verify-m1.sh`
- Reference: `docs/verification/m7-security-supply-chain-audit.md`

- [ ] **Step 1：秘密扫描。** 仓库 Gitleaks/秘密扫描无泄漏。
- [ ] **Step 2：M1/CI 回归。** 运行受管理镜像变化影响的 M1 Compose 和 CI 等价前后端范围；冻结 Reader tree 保持零差异。
- [ ] **Step 3：镜像漏洞扫描。** 用固定 Trivy 版本/数据库扫描新管理镜像，确认恶意插件由源码/产物门关闭；其余系统层和 npm high/critical 原样列出。
- [ ] **Step 4：检查旧镜像。** 文档明确旧 image ID 不再是候选制品，不删除其他项目或用户镜像。

## Task 9：固定验收证据

**文件：**

- Modify: `docs/verification/m7-security-supply-chain-audit.md`
- Modify: `docs/verification/m7-ci-coverage-audit.md`
- Modify: `docs/verification/m7-readiness-audit.md`
- Modify: `docs/progress/refactor-status.md`
- Modify: `docs/verification/final-report.md`（仅更新已有安全状态，不提前关闭 M7）
- Modify: this plan

- [ ] **Step 1：记录固定证据。** 写入 commit、工具版本、命令结果、产物哈希、镜像 ID/digest、浏览器结果和剩余漏洞。
- [ ] **Step 2：更新结论。** 只把 `vite-vue-path-map@1.0.2` 与对应注入特征标为关闭；管理端其余依赖、Server/Reader 和基础镜像继续 No-Go。
- [ ] **Step 3：完成清单。** 逐项勾选本计划，不使用进度文字替代命令和报告证据。
- [ ] **Step 4：最终检查。** 检查 tracked/untracked、忽略产物、秘密、`git diff --check` 和暂存区内容。
- [ ] **Step 5：提交。** 提交信息：`补充管理前端供应链整改证据`。
- [ ] **Step 6：状态确认。** `git status` 干净，不 push；继续下一个 M7 安全阻断，不将长期目标标记完成。

## 服务影响

计划文档本身不影响服务，无需重启。实现完成后需要重新构建并替换 `moonbook-web` 管理前端容器；Go 后端、应用内 Worker、PostgreSQL、Redis、MinIO 和冻结 Reader 不需要因本切片单独重启或迁移。
