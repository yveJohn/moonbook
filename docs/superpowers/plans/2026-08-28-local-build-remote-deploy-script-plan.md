# Moonbook 本地构建远程发布脚本实施计划

## 1. 约束

- 依据 `docs/superpowers/specs/2026-08-28-local-build-remote-deploy-script-design.md` 实施。
- 默认目标固定为 `root@101.32.210.229:/opt/moonbook-app`。
- 本地构建 Linux AMD64 镜像，生产服务器只校验、导入和运行镜像，不编译源码。
- 不上传、覆盖、读取或打印生产 `.env` 内容，不在仓库写入任何 Secret。
- 不执行 `docker compose down`、卷删除、数据库降级、旧镜像删除或数据服务重建。
- 本任务只运行本地自动化测试和 dry-run，不执行真实生产发布。

## 2. 实施任务

### 任务一：发布脚本主体

- [x] 新增 `scripts/deploy-production.sh`，实现帮助、参数解析、默认值和严格错误处理。
- [x] 校验仓库、Git 工作区、Docker Buildx、SSH/SCP、压缩与 SHA-256 工具。
- [x] 按 Git 短 SHA 构建 `server`、`web`、`reader-ui` 三个 Linux AMD64 镜像。
- [x] 在仓库外临时目录导出压缩归档、SHA-256 文件和非敏感发布清单。
- [x] 实现 `--dry-run` 和 `--build-only`，保证模式边界明确。

### 任务二：远程发布流程

- [x] 使用 BatchMode SSH/SCP 上传到发布专用临时目录。
- [x] 校验远程生产目录、Compose 文件、环境文件权限和磁盘容量。
- [x] 远程校验归档 SHA-256 后执行 `docker load`。
- [x] 原子写入只包含三个应用镜像标签的 `compose.release.yml`，保留上一版本副本。
- [x] 使用当前 `compose.yml`、`.env` 和发布覆盖文件运行迁移及应用替换。
- [x] 等待容器健康并验证 `18080`、`18081` 和后端 readiness。
- [x] 输出非敏感发布结果与旧镜像恢复命令，清理远程临时发布物。

### 任务三：自动化测试与文档

- [x] 新增 `scripts/test-deploy-production.sh`，用临时命令替身覆盖参数和流程。
- [x] 验证 dry-run/build-only 不越界，默认发布步骤顺序正确。
- [x] 验证校验、迁移和健康失败时停止后续步骤。
- [x] 验证输出不会泄漏测试 Secret。
- [x] 在部署 Runbook 中补充脚本用法、生产发布仍需明确授权和回退边界。

### 任务四：综合验证与提交

- [x] 执行 Bash 语法检查和发布脚本测试。
- [x] 执行 ShellCheck（本机可用时）、仓库秘密扫描和 `git diff --check`。
- [x] 检查脚本 executable 权限、全部已跟踪和未跟踪文件。
- [x] 按逻辑使用简体中文提交，不执行 `git push`。

## 3. 完成检查

- [x] 默认值准确指向当前生产目录，但测试不连接生产。
- [x] 三个应用镜像具有可复现的 Git SHA 标签和归档校验值。
- [x] 生产 `.env`、数据库、Redis、MySQL、R2 和数据卷不被脚本修改。
- [x] 迁移和健康门禁失败时不会继续危险步骤。
- [x] 工作区干净，所有有效修改已提交。

## 4. 服务影响

- 本次新增脚本、测试和文档不影响运行中服务，无需重启。
- 未来执行默认发布模式会运行 PostgreSQL 版本化迁移，并重建 `server`、`web`、`reader-ui`、`gateway`；PostgreSQL、Redis、MySQL 和 R2 不重启。
