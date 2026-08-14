# Reader 认证兼容接口

## 状态与范围

本文固化 M3 的五个身份路由，来源为冻结 `reader-ui`、旧 Reader 控制器和 M3 规格。实现必须保持 Reader 专用响应器，不复用 GVA 管理端响应。本文不是已通过测试的声明；“待验证”项必须在实现后由契约测试和真实依赖验收补齐。

基础路径由网关决定（开发 `/dev-api`、生产 `/prod-api`），服务内部路径固定为 `/reader/**`。请求认证使用 `Authorization: Bearer <accessToken>`。

## 通用规则

- 成功 JSON：`{"code":200,"msg":"操作成功","data":...}`；查询资料使用 `msg="查询成功"`。
- 认证业务失败保持 HTTP 200，返回 `code=401`、`msg="认证失败，无法访问系统资源"`；网络、路由和原始 SEO 资源按真实 HTTP 语义处理。
- 无 Token 的公开接口按匿名处理；五个身份路由均要求有效 Reader Token，缺失、伪造、过期、撤销或禁用账号统一为上述认证失败。
- GVA 管理员 Token 与 Reader Token 完全隔离。即使管理员 Token 有效，也不能注入 reader ID；在 Reader 身份路由中按未登录处理。
- 所有 `bigint` ID（`id`、`readerId`、`bookId`、`chapterId`、`inviteCodeId`）以及币值在 JSON 中使用十进制字符串，禁止 JavaScript `Number` 运算。`9223372036854775807` 必须无损往返。
- Token 只含 reader ID、session ID 和过期时间等最小声明；数据库只保存 SHA-256 会话摘要，不保存明文 Token。退出、改密和禁用账号可撤销会话。

## 路由契约

### `POST /reader/auth/register`

请求 JSON：

```json
{"username":"alice","password":"示例密码","nickname":"Alice","inviteCode":"ABCD1234"}
```

字段：`username`、`password`、`nickname`、`inviteCode` 均为字符串；邀请码必填，注册必须在同一 PostgreSQL 事务中消费有效邀请码、创建账号、幂等生成新读者自动邀请码并建立邀请关系。M3 不创建钱包或邀请奖励事实。

成功响应：`code=200,msg="操作成功"`，`data` 为登录结果：

```json
{"code":200,"msg":"操作成功","data":{"accessToken":"<opaque>","expireIn":3600,"reader":{"readerId":"9223372036854775807","username":"alice","nickname":"Alice","status":"enabled"}}}
```

用户名重复、邀请码缺失/过期/禁用/超限、字段校验失败使用 HTTP 200 的稳定业务错误；不得泄漏密码摘要或数据库异常。Redis 限流不可用时注册失败关闭，不能绕过限流。

### `POST /reader/auth/login`

请求 JSON：`{"username":"alice","password":"示例密码"}`。成功响应与注册相同。当前 `reader_accounts` 中的 BCrypt 摘要直接校验；历史 `user.password` 32 位 MD5 摘要仅在识别格式后校验，成功登录立即在事务中升级为 BCrypt 并记录算法变更。未知摘要、错误密码和禁用账号均拒绝且不升级。

账号或 IP 限流使用 Redis；Redis 不可用时返回兼容服务不可用错误。响应和日志不得包含密码、摘要全文或 Token 明文。

### `POST /reader/auth/logout`

请求无需 JSON，必须携带 Reader Bearer Token。成功响应：

```json
{"code":200,"msg":"操作成功","data":null}
```

服务端撤销当前 session 摘要；重复退出保持幂等。管理员 Token、过期 Token 或伪造 Token 不得撤销任何 Reader 会话，并返回 `code=401` 认证失败。

### `GET /reader/auth/profile`

必须携带有效 Reader Token。成功响应：

```json
{"code":200,"msg":"查询成功","data":{"readerId":"9223372036854775807","username":"alice","nickname":"Alice","status":"enabled"}}
```

实际字段以冻结 `ReaderProfile` 类型为准；新增字段需先更新契约测试。账号状态、会话撤销和过期时间每次请求均由服务端校验，Redis 缓存丢失不得让有效 PostgreSQL 会话失效。

### `PUT /reader/auth/password`

必须携带有效 Reader Token。请求 JSON：

```json
{"currentPassword":"旧密码","newPassword":"新密码","confirmPassword":"新密码"}
```

成功响应：`{"code":200,"msg":"操作成功","data":null}`。当前密码必须匹配（包括历史 MD5 仅在尚未升级的迁移账号上），两次新密码必须一致；写入 BCrypt 后撤销该读者除当前流程外的全部会话。失败不修改密码或会话，错误仍以 HTTP 200 业务响应表达。

## 验收矩阵

| 场景 | 预期 | 证据状态 |
| --- | --- | --- |
| 五路由成功响应、字段和空值 | 与本文及冻结类型一致 | `http_contract_test.go` 已逐条覆盖，空成功固定 `data:null` |
| 无效 Reader Token | HTTP 200，`code=401` 固定消息 | `http_contract_test.go` 已覆盖缺失、过期、撤销和禁用账号 |
| 管理员 Token 访问 Reader 私有路由 | 不获得 Reader 身份 | `http_contract_test.go` 使用独立签名和管理员 claims 验证固定 `401` |
| BCrypt、历史 MD5 升级、未知摘要 | 仅成功 MD5 登录升级 BCrypt | `service_test.go` 与真实 PostgreSQL 集成测试已覆盖 |
| Redis 不可用的认证限流 | 注册/登录失败关闭 | `service_test.go` 与真实 Redis 集成测试已覆盖 |
| 最大 bigint ID 和字符串 JSON | 无精度损失 | `9007199254740993`、`9223372036854775807` HTTP 契约已覆盖 |
