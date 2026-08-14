# 冻结读者 API 契约清单

## 契约来源

- 冻结 reader-ui tree：`ffbe7bb4c56792e94e160de86cc71bce0a6e0e5e`
- 浏览器 API：`reader-ui/src/api/reader.ts`
- HTTP 包装：`reader-ui/src/api/http.ts`
- SSR API：`reader-ui/app/lib/readerApi.server.ts`
- SEO 代理路由：`reader-ui/app/routes/robots.ts`、`sitemap.ts`、`sitemap-books.ts`
- 数据类型：`reader-ui/src/types/reader.ts`
- 旧实现：旧仓库 `Reader*Controller.java`、对应 DTO、Service 和测试

本清单是 M0 接口表面基线。M3 实施时必须为每项补齐请求/响应样例、空值、错误码、鉴权和边界值测试，并把测试文件链接回本表。

## 全局传输契约

- 浏览器基础路径：`VITE_READER_API_BASE`，生产冻结值为 `/prod-api`，开发缺省值为 `/dev-api`。
- SSR 上游：`READER_API_ORIGIN`，缺省值为 `http://127.0.0.1:55328`。
- 浏览器超时：15 秒；SSR JSON 和 SEO 代理超时：12 秒。
- 登录态 Header：`Authorization: Bearer <accessToken>`。
- 普通成功响应：`{"code":200,"msg":"...","data":...}`；`code` 缺省也按成功处理。
- 分页成功响应：`{"code":200,"msg":"...","rows":[],"total":0}`；缺省 `rows` 转为空数组，缺省 `total` 转为 0。
- 非 200 业务 `code` 抛出 `ApiError(code, msg)`。
- HTTP 401、业务 `code=401`、或 `msg` 包含“未登录/登录已过期”时，只有请求携带的 Token 仍是当前本地 Token 才清除当前会话。
- 所有数据库 Long/雪花 ID 响应字段必须为字符串。金额/币值使用 `ReaderLongValue` 的历史兼容输入，但新 Go API 应优先输出十进制字符串。
- 日期时间格式、`null` 与空字符串语义必须从旧实现和现有测试固化，不能由 GVA 默认 DTO 替代。

## 公开 JSON API

| 方法 | 路径 | 请求要点 | 响应 data/分页 | 调用方 |
| --- | --- | --- | --- | --- |
| GET | `/reader/seo/config` | 无 | `ReaderSeoConfig` | SSR |
| GET | `/reader/books/featured` | 无 | `ReaderBookSummary[]` | 浏览器、SSR |
| GET | `/reader/books` | `keyword/categoryCode/subCategoryCode/sort/pageNum/pageSize` | `rows: ReaderBookCatalogItem[]`, `total` | 浏览器、SSR |
| GET | `/reader/books/random` | 无 | `ReaderBookCatalogItem[]` | 浏览器、SSR |
| GET | `/reader/books/categories` | 无 | `ReaderCategory[]` | 浏览器、SSR |
| GET | `/reader/books/sub-categories` | 无 | `ReaderCategory[]` | 浏览器、SSR |
| GET | `/reader/books/{bookId}` | 字符串 ID | `ReaderBookDetail` | 浏览器、SSR |
| GET | `/reader/books/{bookId}/chapters` | 字符串 ID | `ReaderChapterSummary[]` | 浏览器、SSR |
| GET | `/reader/chapters/{chapterId}` | 字符串 ID；可选登录影响权益 | `ReaderChapterContent` | 浏览器 |
| POST | `/reader/auth/register` | `ReaderRegisterPayload` | `ReaderLoginResult` | 浏览器 |
| POST | `/reader/auth/login` | `username/password` | `ReaderLoginResult` | 浏览器 |
| POST | `/reader/auth/logout` | Bearer Token | 空 data | 浏览器 |
| GET | `/reader/auth/profile` | Bearer Token | `ReaderProfile` | 浏览器 |
| PUT | `/reader/auth/password` | Bearer Token 快照；`currentPassword/newPassword/confirmPassword` | 空 data | 浏览器 |
| GET | `/reader/me/wallet` | Bearer Token | `ReaderWallet` | 浏览器 |
| GET | `/reader/me/wallet/ledgers` | `coinType/pageNum/pageSize` | `rows: ReaderWalletLedger[]`, `total` | 浏览器 |
| GET | `/reader/recharge/products` | 无 | `ReaderRechargeCatalog` | 浏览器 |
| POST | `/reader/recharge/quote` | `diamondAmount` 字符串 | `ReaderRechargeQuote` | 浏览器 |
| POST | `/reader/me/recharge/orders` | `productId? / customDiamondAmount? / requestId` | `ReaderRechargeOrder` | 浏览器 |
| GET | `/reader/me/recharge/orders/{orderId}` | 字符串 ID | `ReaderRechargeOrder` | 浏览器 |
| GET | `/reader/me/checkin/status` | Bearer Token | `ReaderCheckinStatus` | 浏览器 |
| POST | `/reader/me/invite/code` | Bearer Token | `ReaderInviteDashboard` | 浏览器 |
| POST | `/reader/me/checkin` | Bearer Token | `ReaderCheckinStatus` | 浏览器 |
| GET | `/reader/me/entitlements` | Bearer Token | `ReaderEntitlements` | 浏览器 |
| GET | `/reader/products/membership` | 无 | `ReaderProduct[]` | 浏览器 |
| POST | `/reader/me/orders/membership` | `productId/requestId` | `ReaderOrder` | 浏览器 |
| POST | `/reader/me/orders/chapter` | `chapterId/expectedPrice/requestId` | `ReaderChapterPurchaseResult` | 浏览器 |
| POST | `/reader/me/orders/book` | `bookId/expectedPrice` | `ReaderOrder` | 浏览器 |
| GET | `/reader/me/bookshelf` | Bearer Token | `ReaderBookshelf[]` | 浏览器 |
| POST | `/reader/me/bookshelf/{bookId}` | 字符串 ID | `ReaderBookshelf` | 浏览器 |
| DELETE | `/reader/me/bookshelf/{bookId}` | 字符串 ID | `boolean` | 浏览器 |
| POST | `/reader/me/likes/{bookId}` | 字符串 ID | `ReaderBookLike` | 浏览器 |
| DELETE | `/reader/me/likes/{bookId}` | 字符串 ID | `ReaderBookLike` | 浏览器 |
| GET | `/reader/me/likes` | Bearer Token | `ReaderLikedBook[]` | 浏览器 |
| POST | `/reader/me/feedbacks` | `content` | `ReaderFeedback` | 浏览器 |
| GET | `/reader/me/feedbacks` | `pageNum/pageSize` | `rows: ReaderFeedback[]`, `total` | 浏览器 |
| GET | `/reader/me/history` | Bearer Token | `ReaderReadingHistory[]` | 浏览器 |
| GET | `/reader/me/history/{bookId}` | 字符串 ID | `ReaderReadingHistory | null` | 浏览器 |
| PUT | `/reader/me/history/{bookId}` | `ReaderHistoryUpdatePayload` | `ReaderReadingHistory` | 浏览器 |
| GET | `/reader/me/preference` | Bearer Token | `ReaderPreference` | 浏览器 |
| PUT | `/reader/me/preference` | `ReaderPreferencePayload` | `ReaderPreference` | 浏览器 |

## SEO 原始资源

这些接口不使用 JSON 包装，SSR 将上游 Response 原样代理：

| 方法 | 路径 | Content-Type | 语义 |
| --- | --- | --- | --- |
| GET | `/reader/seo/robots.txt` | `text/plain` | robots 内容 |
| GET | `/reader/seo/sitemap.xml` | XML | sitemap 索引或主 sitemap |
| GET | `/reader/seo/sitemap-books-{page}.xml` | XML | 分页书籍 sitemap，`page` 为正整数 |

## 关键兼容枚举和特殊码

- 图书计费：`word_charge`、`membership_only`、`login_free`、`fixed_price`。
- 阅读原因：`login_required`、`book_owned`、`chapter_owned`、`membership`、`login_free`、`membership_required`、`book_purchase_required`、`chapter_purchase_required`、`free_chapter`、`unsupported_mode`。
- 充值状态：`creating`、`pending`、`gateway_unknown`、`create_failed`、`superseded`、`expired`、`callback_exception`、`paid`。
- 章节购买状态：`paid`、`already_owned`、`free`、`quote_changed`。
- 报价变化业务码：`46106`。
- 偏好枚举：主题 `cream/night/green`，阅读模式 `scroll/page`。

## M3 待固化证据

认证 HTTP 包装和 Long ID 边界已有可执行证据：`server/internal/modules/reader/auth/http_contract_test.go`，并与 `service_test.go`、真实 PostgreSQL/Redis 集成测试共同覆盖登录、未登录和会话语义。

2026-08-15 的隔离 Compose + Playwright 关键旅程已覆盖匿名目录/详情、匿名章节 `46101`、邀请码注册、登录免费 MinIO 正文、书架、两章切换、第二章历史恢复、退出及旧 Token `401`。`server/internal/modules/reader/public` 的单元和真实依赖测试固化完整作品商品状态、登录态详情聚合、Long ID 字符串与前后章 ID；详细运行证据见 `docs/migration/m3-reader-commerce.md`。以下清单仍按逐接口矩阵继续收敛，不能因关键旅程通过而整体勾选。

### 可执行覆盖矩阵

| 路由组 | 正常 JSON | 错误/鉴权 | 空值/分页/Long ID | 当前结论 |
| --- | --- | --- | --- | --- |
| `/reader/auth/*` 5 条 | 登录与注册 Long ID 已覆盖；其余成功响应待逐条快照 | `http_contract_test.go` 覆盖无 Token、过期、撤销、封禁账号统一 HTTP 200 + 业务 `401`；服务测试覆盖旧密码升级和退出撤销 | `9007199254740993`、`9223372036854775807` 已覆盖 | 部分完成 |
| `/reader/books*`、`/reader/chapters/*` 8 条 | `public/http_integration_test.go` 与 Playwright 覆盖分页书库、详情、目录、正文；精选、随机、分类仍待逐条快照 | 匿名正文 `46101` 已覆盖；计费模式错误矩阵待补 | 作品、章节、前后章 ID 字符串已覆盖；空目录与分页边界待补 | 部分完成 |
| `/reader/seo/*` 4 条 | SSR、curl 和真实依赖 HTTP 测试已有正常证据 | 超时、非 JSON 和非法 sitemap page 待补 | Content-Type 已在运行证据核对，仍需可执行测试 | 部分完成 |
| `/reader/me/bookshelf|likes|feedbacks|history|preference` 13 条 | `me/http_contract_test.go` 对全部路由执行 JSON 快照 | 共享 Reader 中间件的无 Token/失效 Token `401` 已由认证契约覆盖；服务测试覆盖非法 ID、进度和偏好 | 显式 `data:null`、空数组、空分页、分页归一化及两个 Long ID 边界已覆盖 | 正常响应矩阵完成 |
| 钱包、流水、充值商品/报价/订单 6 条 | `wallet/http_contract_test.go`、`recharge/http_contract_test.go` 已逐条执行 JSON 快照 | 共享 Reader 鉴权已覆盖；充值参数和仓储错误仍待 HTTP 错误快照 | 余额、流水、钻石、USDT 金额、ID 均为字符串；分页归一化和订单 `null` 已覆盖 | 正常响应矩阵完成 |
| 签到、邀请、权益、会员产品、购买 8 条 | 真实 PostgreSQL 服务/仓储集成已覆盖业务闭环 | 报价变化、余额不足、重复请求等服务语义已有测试 | HTTP JSON、金额字符串仍待逐条快照 | 待补 HTTP 契约 |

- [ ] 每个接口的正常请求与响应 JSON 快照
- [ ] 未登录、Token 过期、封禁和权限不足响应
- [ ] `null`、空数组、缺省字段和分页边界
- [ ] `9007199254740993` 与 `9223372036854775807` ID 边界
- [ ] 金额、钱包和报价变化并发语义
- [ ] MinIO 缺失、哈希不一致和超时的外部错误映射
- [ ] SSR 超时、上游非 JSON 和 SEO Content-Type
- [ ] CORS、反向代理前缀与生产 `/prod-api` 路由
