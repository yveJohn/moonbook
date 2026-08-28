# Moonbook 本地构建远程发布脚本设计

## 1. 目标与范围

新增 `scripts/deploy-production.sh`，在本地把当前固定 Git 提交构建为 Linux AMD64 Docker 镜像，传输到 `root@101.32.210.229`，并在现有生产目录 `/opt/moonbook-app` 完成受控发布。

脚本复用 `docs/superpowers/specs/2026-08-27-r2-production-docker-deployment-design.md` 的生产边界：只替换 Go 后端、管理前端和 Reader 前端应用镜像，允许执行版本化数据库迁移，不删除或重建 PostgreSQL、Redis、MySQL、R2 对象、Docker 数据卷、DNS、证书或宿主反向代理配置。

本任务只实现脚本及其自动化测试，不实际发布生产。

## 2. 命令接口

脚本默认值：

- SSH 目标：`root@101.32.210.229`
- 生产目录：`/opt/moonbook-app`
- 平台：`linux/amd64`
- 镜像标签：当前 Git 短提交号
- 应用镜像：`moonbook/server:{sha}`、`moonbook/web:{sha}`、`moonbook/reader-ui:{sha}`

支持以下模式：

- 默认模式：检查、构建、归档、上传、校验、导入、迁移、替换和健康验证。
- `--build-only`：只完成本地构建和发布物生成，不连接远程服务器。
- `--dry-run`：完成输入和环境检查，只显示经过脱敏的执行阶段，不构建、上传或修改远程状态。
- `--help`：显示参数、环境变量和退出码说明。

SSH 目标、生产目录、输出目录和平台允许通过显式参数覆盖，便于测试和后续迁移；脚本不得从位置不明的隐式配置推断生产目标。

## 3. 发布流程

1. 验证本地依赖、仓库位置、Git 提交和工作区状态。默认拒绝脏工作区，避免发布无法复现的代码。
2. 使用 Docker Buildx 按 `linux/amd64` 构建三个镜像并加载到本地 Docker。
3. 使用 Git 短 SHA 作为不可变标签，记录本地镜像 ID。
4. 将三个镜像通过 `docker save` 打包为单个 tar，使用 gzip 压缩并计算 SHA-256；发布物放在仓库外的临时目录。
5. 通过 SSH 创建本次发布专用临时目录，通过 SCP 上传压缩包和校验文件。
6. 远程先校验 SHA-256，再执行 `docker load`。校验失败时不导入、不迁移、不重启。
7. 校验 `/opt/moonbook-app/compose.yml` 和现有 Secret 环境文件存在，但不读取、上传或打印 Secret 内容。
8. 在远程 Compose 环境中记录旧应用镜像，设置三个目标镜像标签，运行一次性 `migrate`，迁移成功后按依赖顺序重建应用容器。
9. 等待 Server、Web、Reader UI 和 Gateway 健康，验证宿主回环地址 `18080`、`18081` 及后端 readiness。
10. 输出提交号、目标镜像、健康结果和旧镜像恢复命令，清理本次远程临时发布物；不清理旧镜像。

## 4. 远程配置更新

当前生产 Compose 文件位于 `/opt/moonbook-app/compose.yml`。脚本不覆盖整个 Compose 文件，也不上传本地 `.env`。应用镜像值通过远程 Compose 已采用的环境文件或专用发布覆盖文件注入；具体实现优先使用独立、非敏感的 `compose.release.yml`，原子替换该文件并保留上一版本副本。

发布覆盖文件只包含三个应用镜像标签，不包含密码、Token、R2 凭据或统一主密钥。Compose 调用必须显式指定项目目录、现有 Compose 文件、发布覆盖文件和现有环境文件，防止作用到其他项目。

## 5. 安全与失败处理

- 禁止 `set -x`，不得打印环境文件内容或展开后的 Compose Secret。
- 本地归档和远程临时目录名称只含提交号与随机后缀，不含凭据。
- 所有 SSH 和 SCP 调用启用 BatchMode，并有连接超时。
- 任一步失败立即停止，不继续迁移或替换容器。
- 迁移失败不重启应用；迁移成功但健康失败时保留新数据库结构，不执行 down migration，只输出旧镜像恢复命令并要求人工确认回退兼容性。
- 不调用 `docker compose down`，不使用 `-v`，不删除卷、数据库、对象或旧镜像。
- 脚本不创建或修改生产 Secret；缺少 `MOONBOOK_APP_MASTER_KEY` 等必需配置时由现有 Compose 校验失败并停止发布。

## 6. 测试设计

新增 Shell 测试，使用临时目录和命令替身验证：

- `--help`、未知参数和必需值校验；
- 默认目标、生产目录、平台和镜像标签生成；
- 脏工作区阻断；
- `--dry-run` 不调用 Docker、SSH、SCP；
- `--build-only` 构建三个镜像、生成压缩归档和 SHA-256，不调用远程命令；
- 默认发布阶段按构建、上传、校验、导入、迁移、替换、健康检查的顺序执行；
- 校验失败、迁移失败和健康失败时停止后续危险步骤；
- 输出不包含测试注入的密码、Token 或主密钥。

测试只使用本地替身，不连接 `101.32.210.229`，不修改生产服务。实现完成后执行 Shell 语法检查、脚本测试、仓库秘密扫描和 `git diff --check`。

## 7. 服务影响

仅新增脚本和测试时不影响正在运行的服务，无需重启。未来实际执行默认发布模式时，会构建并替换 `server`、`web`、`reader-ui` 和 `gateway` 相关应用容器，并先运行 PostgreSQL 版本化迁移；PostgreSQL、Redis、MySQL 和 R2 本身不重启。
