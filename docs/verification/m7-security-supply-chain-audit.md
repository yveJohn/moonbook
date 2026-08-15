# M7 依赖与供应链安全审计

审计时间：2026-08-15 01:15-01:21 UTC

## 结论

当前 `moonbook/server:local`、`moonbook/web:local` 和 `moonbook/reader-ui:local` 均不能作为生产候选镜像。管理端构建链包含一个已被 OpenSSF/npm 标记为恶意的直接生产依赖，且注入逻辑已经出现在本地生产构建产物；Server 和 Reader 镜像也存在已发布修复版本的 high/critical 漏洞。

本审计只读取锁文件、依赖图和本地镜像，不连接生产环境，不把“发现漏洞”误报为“已修复”。

## 扫描基线

- 仓库 commit：`acca1a4`
- npm：`11.11.0`
- pnpm：本机 `11.17.0`，锁文件声明版本 `10.15.1`
- govulncheck：`golang.org/x/vuln/cmd/govulncheck@v1.1.4`
- Trivy：`0.67.2`，镜像 digest `sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08`
- Trivy 数据库：2026-08-15 01:19 UTC 下载
- Server image ID：`c94e0a20b692`
- Web image ID：`9622ab4d21b8`
- Reader image ID：`56409bf67f9c`

依赖扫描使用锁文件且只统计生产依赖。镜像扫描使用 `HIGH,CRITICAL`、`--ignore-unfixed`，因此这里只报告已有上游修复的高危/严重项。为避免把 Docker socket 暴露给扫描器，三个明确镜像先导出到 `/tmp`，Trivy 只读扫描 tar；临时 tar、数据库缓存和 JSON 报告均不提交。

## 管理端恶意依赖

`web/package.json` 直接依赖 `vite-vue-path-map@^1.0.2`，锁文件解析为 `1.0.2`。npm 公告 `GHSA-jvpq-f7f2-w263` 将该精确版本标记为 critical 恶意包，且没有可用修复版本。

该包不是未使用依赖：

- `web/vite.config.js` 导入并在生产构建插件列表调用它；
- 包内 `generateBundle` 会根据隐藏的 GVA 授权全局变量，把混淆脚本插入生产 JavaScript；
- 脚本会延迟请求作者控制的远程图片地址，请求失败时调用 `document.open()`/`document.write()` 替换页面；
- 本地 `web/dist` 的 `pathInfo`、`index` 和 `echarts` chunk 已命中该注入逻辑特征。

因此当前管理端镜像存在供应链远程控制风险，生产 Go/No-Go 必须立即为 No-Go。修复不能只在运行时阻断域名：必须删除恶意包及其构建调用，用仓库自有的确定性路径映射生成器替代，重新生成锁文件和镜像，并验证产物不再包含注入特征。

管理端 `pnpm audit --prod --json` 汇总为：367 个生产依赖，1 critical、15 high、22 moderate、1 low。除恶意包外，主要高风险包括无修复版本的旧 `wangeditor` XSS、直接依赖 `axios@1.8.2`、`echarts@5.5.1`，以及 `vue3-sfc-loader` 引入的旧 PostCSS/Vue 2 编译链。修复需要按直接依赖、真实调用面和替代组件分别验证，不能用批量强制升级替代回归测试。

## 冻结 Reader

`npm audit --omit=dev --json` 对当前锁文件报告 5 个 high、0 critical，均来自 `react-router` 的 `GHSA-qwww-vcr4-c8h2`：RSC Mode 的 CSRF 绕过可能在返回 400 前执行 Action。直接依赖 `@react-router/node`、`@react-router/serve`、`react-router-dom` 当前为 `7.18.0`，公告给出的非 major 修复版本为 `7.18.2`。

这与此前记录的 9 high 基线不同，说明公告集合和锁文件已经变化；M7 必须以固定 commit、扫描时间和数据库版本记录结果。Reader 切换前仍要求冻结业务代码零差异，依赖升级是否纳入允许的安全例外需要明确设计并重新执行 536 个基线测试、生产构建、SSR、契约和浏览器关键旅程，不能只依赖 `npm audit fix`。

Reader 镜像扫描结果：

| 层 | High | Critical |
| --- | ---: | ---: |
| Alpine 3.22.1 | 17 | 2 |
| Node.js 及生产包 | 32 | 3 |

镜像层计数大于 npm audit，因为镜像还包含 Node 运行时和 npm 自身依赖。当前 Reader Dockerfile 运行完整 Node SSR 环境，需升级固定基础镜像并重新扫描。

## Go 与 Server 镜像

源代码扫描使用本机 Go 1.25.12，govulncheck 找到 19 个存在可达调用路径的漏洞，来自标准库和 8 个模块；另有 7 个已导入包漏洞和 27 个依赖模块漏洞没有发现调用路径。可达项包括 `x/text`、Excelize、AWS SDK、Mongo driver、`x/net` HTML 解析、pgx、xz 和 go-redis。

生产镜像更严重：`server/Dockerfile` 固定使用 Go 1.24.2 构建。Trivy 对实际镜像中的五个静态二进制得到：

| 目标 | High | Critical |
| --- | ---: | ---: |
| Alpine 3.21.7 | 0 | 0 |
| `moonbook-admin` | 26 | 3 |
| `moonbook-config` | 16 | 1 |
| `moonbook-finance-reconcile` | 17 | 3 |
| `moonbook-migrate` | 17 | 3 |
| `moonbook-server` | 34 | 3 |

同一 CVE 会出现在多个二进制中，表格不能相加作为唯一漏洞数。确定的生产修复边界包括升级 Go 构建器、pgx `5.8.0`、`x/crypto 0.46.0`、`x/net 0.48.0`、`x/text 0.32.0` 及其兼容依赖，然后重新执行 Go 单元/race/真实集成、空库迁移、Compose 和镜像扫描。

## 管理端镜像

管理端镜像只包含 Nginx 静态产物，Trivy 没有从镜像识别 pnpm 构建依赖，因此镜像扫描不能发现恶意插件；这也是锁文件审计和产物特征扫描必须同时存在的原因。Alpine 3.21.5 层报告 30 high、2 critical，主要来自已过期的 OpenSSL、libexpat、libpng 等系统包。需要更新固定 Nginx/Alpine 基础镜像并重新构建，而不是在运行容器内执行包升级。

## CI 与完成门

当前 CI 只有构建、测试和 Gitleaks，没有依赖、镜像、SBOM 或许可证门。M7 安全项至少在以下全部完成后才能关闭：

1. 删除 `vite-vue-path-map@1.0.2`，使用可审计的本地确定性生成器，并证明新产物不存在远程注入特征。
2. 修复或经用户明确批准缓解管理端其余 high/critical，重新完成管理功能回归。
3. 明确 Reader 安全升级与冻结零差异边界，升级后完成完整 M3 回归或形成有期限、责任人的缓解批准。
4. 升级 Go 工具链和可达漏洞依赖，govulncheck 对生产工具链无可达 high/critical。
5. 更新三个固定基础镜像，Trivy 对生产候选镜像无可修复 high/critical，或每项都有批准例外。
6. CI 固定扫描器版本并生成 CycloneDX/SPDX SBOM、漏洞报告、许可证清单和产物哈希。
7. GVA BSL 1.1 Production Use 商业授权证据由用户提供并进入外部 Go/No-Go 证据位置。

本报告是确定的 No-Go 审计证据，不是安全验收通过报告。
