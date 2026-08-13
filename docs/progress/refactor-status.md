# Moonbook 重构进度

更新时间：2026-08-14

## 总览

| 里程碑 | 状态 | 当前证据 |
| --- | --- | --- |
| M0 冻结、基线与盘点 | 进行中 | GVA 与旧仓库基线已锁定；reader-ui 已原样迁入；首版功能/API/数据清单已建立 |
| M1 工程与本地基础设施 | 未开始 | - |
| M2 小说核心与对象存储 | 未开始 | - |
| M3 读者域与零修改兼容 | 未开始 | - |
| M4 交易、支付与运营 | 未开始 | - |
| M5 内容生产与长任务 | 未开始 | - |
| M6 全量迁移与校验 | 未开始 | - |
| M7 生产切换就绪验收 | 未开始 | - |

## M0 已完成

- Gin-Vue-Admin 固定基线：`02f37833255e0e339c3d69199cb5a468f17de9fc`，见 `docs/baselines/gin-vue-admin.md`。
- 旧 Moonbook 冻结基线：`8e7f57316638d199d7a8d7c964a52a89e13fa281`，见 `docs/baselines/legacy-moonbook.md`。
- `reader-ui` 从冻结 commit 原样导入，138 个文件逐文件比较零差异。
- reader-ui 基线测试：44 个测试文件、536 个用例全部通过。
- 读者 API 首版契约清单：`docs/contracts/reader-api.md`。
- 管理功能矩阵：`docs/inventory/admin-feature-matrix.md`。
- 旧数据域映射总表：`docs/migration/mapping.md`。
- 只读 MySQL 容量盘点脚本：`scripts/inventory-legacy-mysql.sh`。

## 当前已知问题

- GVA BSL 1.1 规定 Production Use 需要商业许可证；生产切换前必须取得并保存授权证据。
- 冻结 reader-ui 的 npm 依赖存在 9 个 high 漏洞，必须在 M7 前升级或形成批准的缓解记录。
- 旧仓库存在约 8 GB 数据库备份，但新仓库未复制该敏感/大体积文件；完整数据副本演练将在 M6 使用受控来源执行。
- 当前有效第三方集成的生产启用状态不能仅凭仓库默认配置确定，需要后续脱敏环境清单或用户确认。

## 当前工作

完成 M0 的清单交叉核对、盘点脚本自测和提交；随后进入 M1，替换上游 MySQL 示例 Compose，建立 Moonbook 专用 PostgreSQL、Redis 和 MinIO 基础设施。

## 下一步

1. 交叉核对旧业务页面、Controller 和迁移映射是否均有归属。
2. 验证盘点脚本帮助、缺参和 Shell 语法路径。
3. 提交 M0 盘点证据。
4. 开始 M1 Compose、配置和真实基础设施集成验证。
