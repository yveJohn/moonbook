# M7 模块边界跨域工作流清单

检查日期：2026-08-15

## 目的与边界

本清单补充 `m7-backend-quality-baseline.md`，把已发现的跨模块 import 和跨域 SQL 归并为实际业务工作流，固定整改时必须保持的现有语义。本文只记录当前事实、数据所有权和回归要求，不批准接口形态、包结构或事务协调实现。

## 数据所有权

| 领域 | 拥有的数据与能力 |
| --- | --- |
| Reader | 读者账号、认证会话、邀请码与邀请关系、书架、阅读历史、偏好、点赞关系、反馈，以及冻结 Reader HTTP 契约的路由、DTO、错误和日期映射 |
| Commerce | 钱包与不可变流水、商品、购买订单、充值订单、会员授予、权益、签到和邀请奖励配置 |
| Novel | 分类、书籍、章节、发布可见性、SEO、正文对象引用与已校验正文读取 |
| Platform/initialize | PostgreSQL 事务基础设施、组合根、配置和横切平台能力；不拥有 Reader、Commerce 或 Novel 业务事实 |

表名前缀不改变所有权。例如 `reader_wallets`、`reader_wallet_ledgers` 和 `reader_purchase_orders` 属于 Commerce，`reader_invite_relations` 属于 Reader。

## 跨域工作流

| 工作流 | 当前越界 | 必须保持的语义与回归证据 |
| --- | --- | --- |
| Reader 路由认证与输出 | Commerce 的签到、购买、充值和钱包 HTTP 包直接依赖 Reader `auth` 与 `wire` 实现 | 冻结路由、HTTP 200 包装、业务 code/message、认证失败语义、日期格式和 Long ID 字符串必须逐接口保持；现有 Reader 契约测试不得减少 |
| 公开书库、详情、章节与正文 | `reader/public` 直接依赖 Commerce catalog、Novel objectstore/readerseo，并直接查询 Novel 表 | 发布过滤、分页排序、章节导航、SEO、权益/价格投影、MinIO 大小与 SHA-256 校验、对象故障脱敏及冻结响应必须保持 |
| Reader 账号商业视图 | `reader/account/http.go` 直接查询权益、会员授予、会员商品、邀请奖励配置和钱包流水 | `/reader/me/entitlements`、`/reader/products/membership`、`/reader/me/invite/code` 的空值、排序、汇总、日期和错误合同保持不变；Commerce 继续拥有商业事实 |
| 邀请注册与双方奖励 | `reader/invite/repository.go` 在 Reader 注册事务中读取 Commerce 奖励配置并直接调用钱包实现 | 邀请码锁定、账号/关系唯一、used_count、自动邀请码、奖励配置快照、双方幂等流水必须同成同败；任何失败不得留下半注册或单边奖励 |
| Reader 个人内容投影 | `reader/me/repository.go` 直接联查 Novel 书籍/章节以生成书架、点赞和历史响应 | 只展示已发布且未删除书籍和启用章节；书架与历史排序、章节名称、空值和冻结 DTO 不变；Reader 事实仍由 Reader 写入 |
| 点赞与书籍汇总 | Reader 点赞事务直接锁定和更新 `novel_books.like_count` | 重复点赞幂等；并发点赞/取消后 `like_count` 必须等于 Reader 点赞关系行数；关系与 Novel 汇总必须原子提交或全部回滚 |
| Commerce 管理端读者校验与展示 | 会员发放、订单、充值订单和钱包管理直接查询或锁定 `reader_accounts` | 不存在、禁用读者的拒绝语义不变；管理列表继续显示正确用户名；人工发放、补单和调账的幂等、余额保护及原事务边界不弱化 |
| 邀请首充奖励 | Commerce 邀请奖励实现直接读取 `reader_invite_relations` | 首次充值奖励只能命中真实有效邀请关系；模拟充值、EPUSDT 回调和人工补单跨入口只能发放一次；订单、奖励事实和钱包流水保持同事务 |
| 商品目标与购买报价 | Commerce 商品管理和章节购买直接查询 `novel_books`、`novel_chapters` | 商品只能引用有效目标；章节购买只能使用启用且未删除章节；书籍/章节快照、字数计价、报价变化、免费章节、扣款、订单和权益语义保持 |

## 事务与锁不变量

整改不能把当前同一 PostgreSQL 事务内的工作流拆成没有补偿的顺序调用。至少以下不变量必须由真实 PostgreSQL 并发或故障测试证明：

1. 注册、邀请关系、邀请码计数和邀请双方钱包奖励同成同败；
2. 点赞关系与 `novel_books.like_count` 同成同败，并在并发后精确收敛；
3. 购买订单、钱包扣款流水、会员授予或内容权益同成同败；
4. 模拟充值/支付入账、首充邀请奖励和订单终态保持现有跨入口幂等；
5. 读者账号锁、钱包锁、书籍锁和业务幂等键的顺序必须明确，新增合同不能制造循环等待。

## 整改验收面

- `TestModuleDependencyRules` 原样通过，不增加例外；
- 业务模块不 import 其他业务模块的非 `contract` 包；
- SQL 所有权审计不再发现 Reader、Commerce、Novel 直接访问其他领域表；
- 冻结 Reader 契约、真实 PostgreSQL/Redis/MinIO 集成测试和 Long ID 边界测试保持通过；
- 为上述四类跨域原子工作流增加回滚、幂等和并发证据；
- 初始化层是唯一实现装配位置，业务包测试可使用窄接口替身而不构造其他领域实现。

## 整改结果

专项设计批准后的实现已在运行时代码基线 `7ec5d01c8a17531c212ae4701018f6dc304038af` 完成：

- Reader 认证、账号、邀请关系、个人内容事实通过 `reader/contract` 与 Provider 对外提供窄能力；
- Commerce 访问判定、Reader 管理搜索投影、钱包与邀请奖励通过 `commerce/contract` 与 Provider 对外提供能力；
- Novel 展示、点赞汇总和购买目标通过 `novel/contract` 与 Provider 对外提供能力；
- `initialize` 是唯一跨领域实现组合根，业务包不再直接构造其他领域实现；
- 注册奖励、点赞、购买和首充奖励保留同一 PostgreSQL 事务，统一 Reader 账号锁、订单锁、钱包锁顺序，并增加 advisory lock 防止跨入口首充奖励竞争；
- `TestModuleDependencyRules`、SQL 所有权、表所有权和组合根测试全部通过，未增加例外；
- 真实 PostgreSQL 回滚、幂等和并发测试、CI 完整范围 Linux race、M3 冻结契约和浏览器关键旅程均通过。

隔离验收库的 Commerce Reader 搜索投影最终为 `237/237`，四字段双向差异为 0。投影只用于 Commerce 管理搜索和展示，认证、状态校验、锁定、调账、购买、会员与奖励决定继续调用实时 Reader 合同。

本清单的模块边界整改项已关闭；管理前端恶意依赖、镜像漏洞、统一发布门和 8 GB 完整副本演练仍按 M6/M7 独立跟踪。
