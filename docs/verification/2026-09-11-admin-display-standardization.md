# 管理后台展示规范统一

## 范围与实现

本次按用户要求统一管理端展示，不改变读者契约、业务状态机、数据库结构或接口提交值。

- 状态：`web/src/utils/adminDisplay.js` 提供中文映射，修复采集候选、导入任务、TXT 导入、抓取日志、清洗结果、读者、反馈、邀请码、钱包奖励、消费订单、定时任务和平台任务/尝试记录中的原始英文状态。已有领域中文标签保留，HTTP 状态码保持数字。
- 时间：复用统一的本地时区格式化函数，显示 `yyyy-mm-dd hh24:mi:ss`（例如 `2026-09-11 15:04:05`）。覆盖列表、详情、仪表盘更新时间和系统运行状态更新时间；空值或无效值显示 `-`。日期筛选参数和业务日统计维度保持原有语义。
- 长文本：管理源码中 59 个含表格的组件、88 处 Element Plus 表格接入 `v-table-display`。纯文本单元格单行省略、原生悬浮提示完整文本，点击或聚焦后 Enter/空格复制显示文本；异步更新重新绑定标题。交互控件、链接、图片与展开按钮不拦截。卸载时清理监听器和观察器；复制失败提供中文提示。
- 作品：小说、章节、清洗、画像、合并来源/目标、TXT 导入、作品商品和整书订单统一使用“作品名称（作品ID）”。作品选择支持名称搜索、完整 ID 精确查询与已选值回显，接口仍提交原始字符串 ID。优先使用响应已有名称；缺少名称时调用已有只读作品接口，合并同时发生的同 ID 查询。删除/无权限/查询失败时显示“作品不可用（ID）”，不伪造名称。
- 选择框用显式 label 插槽更新异步名称，避免 Element Plus 远程选项缓存继续显示初始“加载中”。不增加新的 API 或权限。

## 验证

使用项目锁定的 pnpm 版本（本机默认 pnpm 11 与项目要求不一致）：

```sh
cd web
corepack pnpm@10.15.1 run lint
corepack pnpm@10.15.1 run test:moonbook
corepack pnpm@10.15.1 run build
corepack pnpm@10.15.1 run check:supply-chain
```

- 管理端测试共 43 项，包含下午/午夜/日期/空值/无效日期、本地时区、中文状态、HTTP 状态码和超过 JavaScript 安全整数的作品引用。
- Playwright Chromium 使用临时隔离页面挂载真实组件并提供 API 测试响应，验证：文本确实超宽且 CSS 为 ellipsis/nowrap、悬浮 title 与完整文本一致、点击和键盘复制结果完整、更新后复制新值、操作按钮仍执行、异步作品名称回显、选择 `9007199254740993` 后 ID 精确不变。
- 抽查真实候选、TXT 导入和章节组件：英文状态转换、完整日期时间、缺少书名时补查名称、上传表单作品选择均通过。
- 本地浏览器产物（忽略文件，不提交）：`output/playwright/admin-display-desktop.png`、`admin-display-chapters.png`、`admin-display-mobile.png`。手机截图用于检查表格横向滚动及作品选择显示，不代表全站移动端重构验收。
- 临时页面、Vite 进程和本次浏览器会话在验收后清理。没有连接生产或执行真实业务写入。此次为前端展示改动，不重复运行与之无关的后端/数据库集成测试。

## 服务影响

仅影响管理前端 `web`。本地已有 Vite 开发服务可热更新，刷新页面即可；部署环境需要重新构建并发布管理前端静态资源，若通过容器提供资源则重建/重启对应管理前端容器。Go 后端、reader-ui、PostgreSQL、Redis、MinIO 无需重启。

## 生产发布记录

用户于本次任务中明确授权部署生产。发布在 **2026-09-11 00:39:39—00:39:46（UTC+8）** 完成。

| 项目 | 结果 |
| --- | --- |
| 代码提交 | `67841f1faf56563ac1d7d8cb33c3c9661053ddb5` |
| 生产目标 | `101.32.210.229:/opt/moonbook-app` |
| 新 Web 镜像 | `moonbook/web:67841f1faf56`，Linux AMD64 |
| 镜像 ID | `sha256:5c3678b15369262c4df09eeb63f97761a6e88beb477f5608c9e914d3a8bbf0eb` |
| 归档 SHA-256 | `89770577ca6b166865bed615b6ff25506614699395188350ee41e93d6e118bec` |
| HTML SHA-256 | `ace4ae1b89bed18fd5892fa6cc42ce6e5188f64e10407322b0d3f234d2f3410e` |
| 旧 Web 镜像 | `moonbook/web:0b6b6e003b42`，保留用于回退 |
| 发布及回退资料 | `/opt/moonbook-app/releases/admin-display-67841f1faf56/` |

构建从固定 Git 提交导出管理源码，附加经白名单核查的前端公开生产变量，未复制后端 Secret。镜像在本地构建和验收；最终镜像的静态资源提取后通过供应链检查（354 个源文件、227 个 JavaScript 产物）、HTTP 和本次功能标记检查。约 32 MB 归档上传后远端 SHA-256 及镜像 ID 均校验一致。

发布仅替换 `compose.release.yml` 的 `web.image`，执行：

```sh
docker compose --project-directory /opt/moonbook-app \
  --env-file /opt/moonbook-app/.env \
  -f /opt/moonbook-app/compose.yml \
  -f /opt/moonbook-app/compose.release.yml \
  up -d --no-deps --no-build --pull never --wait --wait-timeout 120 web
docker exec moonbook-app-gateway-1 nginx -t
docker exec moonbook-app-gateway-1 nginx -s reload
```

网关仅平滑重载以刷新 Web 上游地址，容器没有重建。Server、Reader、Gateway、PostgreSQL、Redis 的容器 ID 和启动时间与发布前一致；全部健康，四个应用容器 restart count 为 0。没有运行迁移、修改业务数据、写入 R2 或更改生产 Secret、DNS、证书和宿主反向代理配置。

验证结果：

- 源站管理入口及 HTML 哈希通过；管理/读者健康端点连续 15 次 HTTP 200。
- 公网管理首页、网关健康、API readiness、读者首页、读者健康、读者 API readiness 共六项 HTTP 200。
- 公网管理 HTML 及 29 个入口/展示相关资源与本地发布物哈希一致，包含状态中文、作品选择和表格复制实现。
- Chromium 生产登录页面加载成功，控制台 0 error、0 warning。截图为本地忽略产物 `output/playwright/admin-display-production-login.png`。未使用生产账号写入业务数据；登录后交互以此前隔离组件验收为证据。

发布目录保存 `release.json`、归档和校验文件、`compose.release.before.yml`、`containers.before.json`、`containers.after.json`、`deployment-result.json`、`observation.json`、`public-verification.json` 和本次 `activate.py`。发布脚本在切换或源站验证失败时会自动恢复原 Web 配置和容器；此次未触发回退。手动回退时应先确认没有后续发布，只恢复 Web 镜像到已保存的旧镜像并平滑重载网关，不回退数据库或其他服务。

本次生产更新已完成，无需用户再重启服务；浏览器刷新管理页面即可加载新版本。此后追加的发布记录属于文档变更，不影响服务，无需重启。
