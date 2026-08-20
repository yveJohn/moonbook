# M6 完整数据副本迁移演练

检查日期：2026-08-21

## 当前状态

状态：No-Go，源库 16 个 TXT 导入任务缺少原文件。

完整副本已成功恢复到隔离 MySQL，并在业务迁移前由 TXT 原文件门停止。本文件不把成功恢复表述为完整迁移通过；隔离项目和证据继续保留，待取得原文件后从空目标重新演练。

## 固定输入

| 项目 | 固定值 |
| --- | --- |
| 候选 | 冻结旧仓库 `exports/moonbook_admin_20260715_175649.sql.gz` |
| 压缩字节 | 1,065,163,488 |
| SHA-256 | `94d86df76780cc85cdec4690265866140b202bb2b5d908826bf37783c9c8cbe3` |
| gzip 完整性 | `gzip -t` 通过 |
| 隔离项目 | 必须匹配 `moonbook_verify_m6_*` |
| 源库 | 恢复后 `read_only=ON`、`super_read_only=ON`，迁移账号仅 SELECT/SHOW VIEW |
| 目标 | 空 PostgreSQL、空 MinIO 和独立 Redis/网络/命名卷 |

## 已完成的脚本合同

- 拒绝相对备份路径、错误 SHA-256、非 M6 项目名、环境/命令项目名不一致、位于新旧仓库内的工作目录和错误 cleanup 确认；
- 默认 `--preflight` 不启动容器；
- TXT 任务为零时生成空清单，任务非零时缺少外部清单或根目录立即 No-Go；
- 恢复过程只流式计数字节，不落地解压 SQL；
- 保存源盘点前后哈希、结构重跑 `applied=0`、两次稳定审计结果和 30 秒资源采样；
- 清理只接受与隔离 Compose 项目名完全一致的二次确认，并同时载入受控 overlay；项目容器或卷仍有残留时返回失败。

验证命令：

```bash
bash -n scripts/verify-m6-full-copy.sh scripts/lib/full-copy-common.sh \
  scripts/inventory-legacy-mysql.sh scripts/test-verify-m6-full-copy.sh
scripts/test-verify-m6-full-copy.sh
```

## 2026-08-21 真实运行结果

| 证据 | 结果 |
| --- | --- |
| commit / Compose 项目 / 工作目录 | `41b53e9691db99b16be55d527a2bc04512d4b88f` / `moonbook_verify_m6_20260821` / 仓库外受控目录 |
| 实际恢复字节 / 恢复耗时 / 吞吐 | 3,834,607,260 / 64 秒 / 约 59.92 MB/s（57.14 MiB/s） |
| TXT 源任务数 / 清单结论 | 16 / 无清单；旧实现未保存原文件，冻结仓库和本机常用目录无精确匹配，No-Go |
| 源只读状态 | `read_only=1`、`super_read_only=1` |
| 源盘点 | 103 行 TSV；SHA-256 `2d98a2a91272aedaa72013148ac1a6db712e9590b2f5437cff4a8bba6888cad2` |
| 结构迁移 / 业务迁移 / 核对 / 幂等重跑 | 未运行；在所有目标写入前被 TXT 门阻止 |
| checkpoint / 行数 / 关联 / 财务 / 对象 | 未运行，不得判为通过 |
| 恢复阶段资源采样 | MySQL 最高样本 75.14% CPU、712.2 MiB 内存、22.4 GB 块写入、46 PID；宿主最低样本可用 1,067,052,023,808 字节 |
| 12 小时窗口 | 仅恢复阶段约 76 秒；缺少业务迁移和核对数据，不能计算完整余量或最晚回退时间 |
| cleanup | 首次端口冲突产生的空资源已精确清理；当前恢复项目刻意保留供 TXT 缺口排查 |

## TXT 缺口证据

- 只读查询确认 16 条任务中既有成功也有失败状态，所有记录都声明原文件名和字节数；报告不记录具体书名或正文。
- 旧 `TxtImportServiceImpl` 通过 `MultipartFile.getBytes()` 解析章节，只在 MySQL 保存文件名、大小和任务结果，没有把原文件持久化到 OSS、文件系统或数据库。
- 冻结仓库有效目录、Documents（排除旧项目）、系统 Spotlight 索引及按声明字节数的常用目录扫描均无匹配。
- 不能从已规范化的章节正文反向伪造原文件，因为编码、章节标题格式和原始字节无法证明一致。
- 解除门槛：提供这 16 个原文件的只读目录，并按任务 ID 建立 `docs/migration/legacy-txt-manifest.example.json` 格式的仓库外清单；脚本会逐文件校验存在性和声明大小，迁移后再由 MinIO 内容 SHA-256 门验证。

任一迁移错误、未完成 checkpoint、源变化、对象差异、财务差异、幂等差异或时间窗口不足都必须写为 No-Go，并保留隔离资源供排查，不能执行 cleanup 后宣称通过。
