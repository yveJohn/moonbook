# Gin-Vue-Admin 固定基线

## 来源

- 官方仓库：`https://github.com/flipped-aurora/gin-vue-admin.git`
- 分支：`main`
- 固定 commit：`02f37833255e0e339c3d69199cb5a468f17de9fc`
- commit 时间：`2026-08-06T18:15:44+08:00`
- commit 标题：`fix: 修复版本依赖错误导致的无法拉取更新的bug`
- 锁定日期：`2026-08-14`

## 导入方式

基线先克隆到独立临时目录，再从固定 commit 使用 `git archive` 导出以下已跟踪路径并解包到 Moonbook 仓库：

- `server/`
- `web/`
- `deploy/`
- `LICENSE`
- `Makefile`
- `README.md`
- `README-en.md`
- `SECURITY.md`
- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `.gitattributes`
- `gin-vue-admin.code-workspace`

未导入上游 `.git`、编辑器/Agent 配置和上游 `AGENTS.md`。Moonbook 仓库已有 Git 历史、`AGENTS.md` 和设计文档保持不变。

固定 commit 的已跟踪源码原本含有尾随空格和 EOF 空行，因此首次基线导入的全量 `git diff --cached --check` 会报告上游既有格式问题。为保持来源文件逐字可追溯，导入提交不机械清理这些内容；Moonbook 自有文件已单独通过 whitespace 检查。后续修改的文件必须正常通过 `git diff --check`，不能援引该例外引入新的格式问题。

## 冻结规则

- 本 commit 是永久开发基线，不自动合并上游 `main`。
- 后续安全修复只能在明确评估后选择性移植，并记录来源 commit。
- Moonbook 业务能力应放在独立模块中，尽量不侵入 GVA 公共基座。
- 禁止通过重新初始化仓库、覆盖或重置来更新基线。

## 许可证审查

该基线根目录 `LICENSE` 为 Business Source License 1.1，并明确规定超出个人使用、评估开发和教育用途的 Production Use 需要另行取得商业许可证。正式处理真实业务数据、公开部署或生产切换前，项目负责人必须确认并保存有效授权证据。

许可证还要求：

- 向接收者提供许可证副本；
- 修改的上游文件携带醒目的修改及日期声明；
- 不移除版权、署名和其他通知。

本仓库保留原始 `LICENSE`。对 GVA 上游文件的后续修改必须遵守上述要求。许可证授权确认属于生产切换前的强制 Go/No-Go 项，不以技术测试通过替代。
