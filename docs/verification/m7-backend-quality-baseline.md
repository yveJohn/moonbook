# M7 后端质量门基线

验证时间：2026-08-15

## 结论

固定提交 `c88052fba2ff1d7a8710b568eddfd38b033d2790` 的后端格式、Go 模块校验和 `go vet` 通过，但 CI 同范围的 `go test` 未通过。确定阻断不是外部依赖或沙箱环境，而是模块依赖规则发现 15 处跨域直接引用实现包。

因此当前 `.github/workflows/ci.yml` 的 `Test Moonbook foundation` 在 Linux CI 中也应失败，M7 后端质量门和统一验收门均不能标记完成。本报告不通过跳过、删除或放宽边界测试来换取绿色结果。

## 验证范围

命令与仓库 CI 的后端包范围一致，本机按仓库约束关闭 cgo 并使用独立缓存：

```bash
gofmt -l internal/modules internal/platform \
  cmd/moonbook-admin cmd/moonbook-config \
  cmd/moonbook-legacy-migrate cmd/moonbook-migrate

go mod verify

CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache go vet \
  ./internal/... ./config ./core/... ./initialize/... ./middleware \
  ./cmd/moonbook-admin ./cmd/moonbook-config \
  ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate

CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache go test \
  ./internal/... ./config ./core/... ./initialize/... ./middleware \
  ./cmd/moonbook-admin ./cmd/moonbook-config \
  ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate
```

本地没有执行 `-race`。仓库已记录本机 ARM cgo 在上游 `go-m1cpu` 初始化期间不稳定，race 证据仍必须由 Linux CI 或等价 Linux 验证环境提供；这不改变普通测试当前已有确定失败的事实。

## 通过项

- `gofmt -l` 无输出；
- `go mod verify` 输出 `all modules verified`；
- 同范围 `go vet` 退出码为 0；
- 全量普通测试运行中，除下述模块边界包和本地监听环境项外，其余列出的包均通过或明确为无测试文件；
- `internal/modules/novel/importtask` 的四个本地 HTTP fixture 用例在允许回环监听后通过；
- `internal/platform/legacymigrate` 的封面下载安全与限制用例在允许回环监听后通过。

最后两项在受限沙箱内最初因 `listen tcp4 127.0.0.1:0: bind: operation not permitted` 失败。沙箱外仅放开本机随机端口监听后对应包分别返回 `ok`，因此不记录为 Moonbook 代码失败。

## 确定失败

`internal/modules/dependency_test.go` 要求业务模块只能通过目标模块的 `contract` 包协作。当前没有任何业务域 `contract` 目录，测试报告 15 处违规，分布在 9 个文件：

| 调用模块 | 被直接引用模块 | 文件 | 直接依赖 |
| --- | --- | --- | --- |
| commerce | reader | `commerce/checkin/http.go` | Reader 鉴权实现 |
| commerce | reader | `commerce/purchase/http.go` | Reader 鉴权与 wire 实现 |
| commerce | reader | `commerce/recharge/http.go` | Reader 鉴权与 wire 实现 |
| commerce | reader | `commerce/wallet/http.go` | Reader 鉴权与 wire 实现 |
| reader | commerce | `reader/account/http.go` | 邀请奖励实现 |
| reader | commerce | `reader/invite/repository.go` | 邀请奖励实现 |
| reader | commerce、novel | `reader/public/http.go` | Commerce 与小说实现 |
| reader | commerce、novel | `reader/public/service.go` | 目录访问、对象存储和 SEO 实现 |
| reader | commerce | `reader/public/types.go` | Commerce 访问结果类型 |

这不是单纯的测试维护问题。批准架构明确禁止模块跨边界操作其他模块内部实现，而当前 Reader 兼容层、交易 HTTP 层和鉴权/序列化帮助代码已经形成双向实现依赖。

## 关闭条件

关闭该质量门必须满足：

1. 为 Reader 身份、Commerce 访问判定和 Novel 正文/SEO 等跨域能力定义最小稳定契约；
2. 由初始化组合根注入契约实现，业务模块不直接构造或导入其他模块内部 Service、DTO 或帮助包；
3. 保持冻结 Reader API 的路由、响应、错误语义和 Long ID 字符串合同不变；
4. `TestModuleDependencyRules` 原样通过，不增加文件级或包级放行；
5. 同范围普通测试、vet 和 Linux race 测试通过；
6. 在固定验收提交上重新生成本报告，并由 CI 保存可定位日志。

该重构涉及多个既有模块边界，应先完成专项设计和批准，再实施；本报告只固定问题证据，不代表设计已经批准。
