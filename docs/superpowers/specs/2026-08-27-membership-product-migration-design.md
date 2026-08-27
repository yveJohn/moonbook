# Moonbook 会员商品迁移兼容设计

日期：2026-08-27

## 背景与目标

旧 MySQL 中存在周卡、月卡、季卡、年卡和永久会员五个独立会员商品。它们的 `target_id` 均为空，但价格、时长、名称和排序不同。目标 PostgreSQL 的 `commerce_products_target_unique` 索引使用 `COALESCE(target_id, 0)`，将所有空目标会员商品视为同一个商品，导致迁移器只保留首个套餐。

本次修复需要完整迁移五个会员套餐，同时继续保证书籍和章节商品在同一业务目标上唯一。

## 方案

新增 `00072` PostgreSQL 前向迁移：

- 删除现有 `commerce_products_target_unique` 索引。
- 创建同名部分唯一索引，仅对 `target_id IS NOT NULL` 的记录约束 `(product_type, target_id)` 唯一。
- 不修改商品主键、商品类型检查、目标关联检查或购买流程。

迁移器同步调整 `reader_product` 去重规则：

- 书籍和章节等有目标商品继续按 `(product_type, target_id)` 检查目标重复。
- 会员和免广告等空目标商品按源 ID 幂等迁移，不再因同为 `NULL` 而互相去重。
- 已迁移的商品继续通过 `ON CONFLICT(id)` 更新，不产生重复业务记录。

## 数据流程

1. 在目标 PostgreSQL 应用 `00072`。
2. 重置 `reader-commerce` checkpoint 前保存原始 checkpoint。
3. 幂等重跑 Reader Commerce 阶段。
4. 验证五个会员商品的 ID、名称、价格、时长、销售状态和排序与源库一致。
5. 对已补迁的 `DUPLICATE_PRODUCT_TARGET` 迁移错误设置 `resolved_at` 和说明，保留完整审计记录。

## 测试与验收

- 嵌入式迁移清单包含 `00072`，迁移 SQL 保持 forward-only。
- 迁移契约测试确认唯一索引带有 `WHERE target_id IS NOT NULL`。
- Reader 迁移测试覆盖多个 `target_id=NULL` 的会员商品可同时迁移。
- 书籍和章节商品仍不能为同一 `(product_type, target_id)` 创建重复记录。
- 目标库最终存在五个会员套餐，四条旧迁移错误均已解决。

## 回退

数据库迁移保持前向修复，不提供破坏性 Down。应用前保存目标数据库备份和 checkpoint；若验证失败，停止业务切换并恢复目标数据库备份。源 MySQL 始终保持只读，不受本次索引调整影响。
