# 旧 Moonbook 冻结基线

## 仓库来源

- 路径：`/Users/yve/Documents/moonbook`
- 远端：`https://github.com/yveJohn/moonbook.git`
- 分支：`main`
- 冻结 commit：`8e7f57316638d199d7a8d7c964a52a89e13fa281`
- commit 时间：`2026-08-12T03:21:28+08:00`
- commit 标题：`新增TXT失败文件章节修复计划`
- 冻结日期：`2026-08-14`

旧仓库是功能、读者接口和数据迁移的只读来源。冻结时工作区存在一个未跟踪数据库备份 `exports/moonbook_admin_20260715_175649.sql.gz`，该文件未导入新仓库。`reader-ui` 路径没有已跟踪或未跟踪修改。

## reader-ui 导入证据

- 来源 tree：`8e7f57316638d199d7a8d7c964a52a89e13fa281:reader-ui`
- Git tree ID：`ffbe7bb4c56792e94e160de86cc71bce0a6e0e5e`
- 已跟踪文件数：`138`
- 导入方式：从冻结 commit 执行 `git archive`，只导出 `reader-ui/` 已跟踪文件
- 排除内容：旧目录的 `node_modules`、`build`、`dist`、`.playwright-cli` 日志、`.DS_Store` 和其他未跟踪/忽略文件

导入后使用从同一归档解出的参考副本执行：

```bash
diff -qr /private/tmp/moonbook-reader-reference/reader-ui reader-ui
```

命令退出码为 `0` 且无输出，证明导入内容逐文件一致。

## reader-ui 测试基线

依赖按冻结的 `package-lock.json` 使用 `npm ci` 安装，锁文件未变化。执行 `npm test` 的结果：

- 测试文件：`44 passed`
- 测试用例：`536 passed`
- 失败：`0`
- 执行日期：`2026-08-14`

## 已知安全基线

冻结依赖执行 `npm audit` 报告 `9 high`、`0 critical`：

- React Router 7.18.0/7.18.1 依赖链，包括 CSRF 绕过公告；可升级到 7.18.2；
- `nanoid` 3.3.17 及更低版本的拒绝服务公告；
- `postcss` 8.5.22 及更低版本的 source map 路径读取公告；
- `undici` 7.28.0 及更低版本的响应处理和缓存相关公告。

为保证本次“原样迁入”可证明，基线提交不修改依赖。安全修复必须作为后续独立提交执行，升级后重新运行全部读者测试、SSR 构建和契约测试。所有高危项必须在 M7 安全验收前关闭或形成经批准的缓解及风险接受记录。

## 冻结规则

- 正式切换并通过兼容验收前，不进行读者端业务重构或新功能开发。
- 必要安全升级和阻止上线的严重缺陷修复必须单独记录，并确保不改变读者 API 契约。
- 每次修改后都要重新与本基线比较，并在兼容证据中解释差异。
