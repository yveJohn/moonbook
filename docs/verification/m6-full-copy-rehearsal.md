# M6 完整数据副本迁移演练

检查日期：2026-08-21

## 当前状态

状态：待执行完整副本 `--run`。

脚本、隔离边界和报告模板已经就绪；本文件不把预检或小夹具测试表述为完整演练通过。真实运行完成后必须用工作目录中的脱敏证据填写所有结果，不得保留“待填写”项后关闭 M6。

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

## 真实运行结果

以下字段必须在 `--run` 完成后填写：

| 证据 | 结果 |
| --- | --- |
| commit / Compose 项目 / 工作目录 | 待填写 |
| 实际恢复字节 / 恢复耗时 / 吞吐 | 待填写 |
| TXT 源任务数 / 清单结论 | 待填写 |
| 结构迁移 / 业务迁移 / 核对 / 幂等重跑耗时 | 待填写 |
| checkpoint processed/errors/done | 待填写 |
| 源盘点前后 SHA-256 | 待填写 |
| 行数、最大主键和关联 | 待填写 |
| MinIO 对象数量、字节和 SHA-256 | 待填写 |
| 财务核对差异 | 待填写 |
| 峰值 CPU、内存、块 IO、PID 和最低磁盘余量 | 待填写 |
| 总耗时 / 12 小时余量 / 最晚回退时间输入 | 待填写 |
| cleanup 项目和卷结果 | 待填写 |

任一迁移错误、未完成 checkpoint、源变化、对象差异、财务差异、幂等差异或时间窗口不足都必须写为 No-Go，并保留隔离资源供排查，不能执行 cleanup 后宣称通过。
