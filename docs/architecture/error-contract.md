# 统一错误规范

## 边界

Moonbook 业务模块使用 `server/internal/platform/apperror` 表达可预期失败。错误由稳定 `Code`、HTTP 状态、可公开消息和可选内部 cause 组成。cause 只用于服务端诊断，不得直接写入 API 响应、操作记录或访问日志。

稳定错误码如下：

| Code | 语义 | 缺省 HTTP 状态 |
| --- | --- | --- |
| `INVALID_ARGUMENT` | 请求字段或状态不合法 | 400 |
| `UNAUTHENTICATED` | 未登录或凭据失效 | 401 |
| `FORBIDDEN` | 已认证但无权操作 | 403 |
| `NOT_FOUND` | 目标业务对象不存在 | 404 |
| `CONFLICT` | 幂等键、版本或业务状态冲突 | 409 |
| `RATE_LIMITED` | 超过限流或配额 | 429 |
| `UNAVAILABLE` | 依赖或服务暂时不可用 | 503 |
| `INTERNAL` | 未分类内部失败 | 500 |

任何未知 Go `error` 对外统一映射为 `INTERNAL` 和 `internal service error`，不得透传 `err.Error()`。

## 管理 API

管理接口保持 GVA 现有 HTTP 200、`code=7` 失败包装，业务模块通过 `apperror.WriteManagement` 返回：

```json
{
  "code": 7,
  "data": {
    "errorCode": "NOT_FOUND",
    "requestId": "...",
    "traceId": "..."
  },
  "msg": "book not found"
}
```

`requestId` 和 `traceId` 用于关联结构化日志。调用方记录内部 cause 时必须经过敏感信息脱敏，禁止把 cause 交给 `gin.Context.Error` 后再由访问日志原样输出。

## 读者兼容 API

读者接口不得直接使用管理包装。M3 为每个冻结接口建立显式错误映射，将平台错误码转换为旧 `reader-ui` 实际依赖的 HTTP 状态、业务 `code`、`message` 和 `data` 形态。内部模块错误和 GVA 响应格式不得泄漏到兼容接口。
