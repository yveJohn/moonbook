# M7 SBOM 与许可证验收

验收日期：2026-08-21

镜像源码基线：`a828be3a24b933abb0aad7668807e82847dd3e39`

证据生成目录：`/private/tmp/moonbook-task9-supply.Flm5yq`

## 结论

Server、管理端 Web 和冻结 Reader 的最终候选镜像均已生成 SPDX 2.3 JSON 与 CycloneDX 1.6 JSON SBOM，并使用同一固定 Trivy 数据库复扫。镜像直接扫描与 SBOM 扫描的已有修复版本 High/Critical 均为 0；管理端 `pnpm audit --prod` 和 Reader `npm audit --omit=dev` 的所有严重级别均为 0。

许可证清单已经形成，但不能据此宣称生产许可证全部放行。Syft 从静态 Go 二进制识别的 559 个 Server 包中有 525 个没有嵌入可判定的许可证元数据；三个 Alpine 运行层还包含 GPL/LGPL 多许可证组件，需要在分发方式确定后进行法律复核。仓库根 GVA BSL 1.1 明确要求 Production Use 取得商业授权，该授权证据尚未提供，因此生产 Go/No-Go 仍为 No-Go。

## 固定工具

| 工具 | 固定版本 |
| --- | --- |
| govulncheck | `v1.1.4` |
| Trivy | `0.67.2@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08` |
| Syft / 许可证元数据编目 | `1.51.0@sha256:678bfa565b60f747aac0f8e964fe5588a24445b8d0a480e91f6efd70020dfbb0` |

`scripts/scan-supply-chain.sh` 要求三个已构建镜像引用和仓库外已存在的非符号链接绝对输出目录。脚本扫描源码锁文件，导出镜像 tar，生成两种 SBOM，分别扫描镜像和 CycloneDX SBOM，输出许可证摘要与 SHA-256 清单，并在结束时删除临时 tar。任何已有修复版本 High/Critical 或锁文件审计失败都会使脚本非零退出。

## 候选镜像

| 组件 | 镜像 ID | 大小 | 镜像扫描 | SBOM 扫描 |
| --- | --- | ---: | ---: | ---: |
| Server | `sha256:80d3c76244ea14b5b224ad7039c0ac862d2970ba847a5418a7150719187cbd60` | 183,900,927 B | 0 | 0 |
| Web | `sha256:e17a048d4c5fd3cc2f9d9bed367e6dd3217cd4eab7945ce7c004416cc453298e` | 71,093,320 B | 0 | 0 |
| Reader | `sha256:9505ce3747827e5e7a86deb9d88d6efb53a04610b354fde909ef0ebc8ef6eee2` | 350,877,337 B | 0 | 0 |

扫描口径为 `HIGH,CRITICAL --ignore-unfixed`，所以 0 表示不存在上游已有修复版本的 High/Critical，不表示没有任何低中危或尚无修复的公告。

## SBOM 摘要

| 组件 | 包数量 | `NOASSERTION` | CycloneDX SHA-256 | SPDX SHA-256 |
| --- | ---: | ---: | --- | --- |
| Server | 559 | 525 | `94133ded4bf005b43b59dcb94c37341aae4ca016e798a9725c85adecc0317b7f` | `308418b93fa138ce9e4729a9ded4001a1b546efe43996ac4a6821ce8c2ddeb54` |
| Web | 72 | 1 | `1de09f0e8eae751cc85cb3664f9d06f3d451adb5dcb8b0c8a3a9300186538947` | `ed4816bfc71a2227ae994293a72a6cece8127b864974e7cb76979e0398d3693d` |
| Reader | 144 | 3 | `cf877175d0d4ea6aa9c4e3d19bf113e37e04dcffdc028b2048fe643d997ef479` | `7d029ae4d4997718d577d36d0e4210d15bc4a8dac75d795060202c3cef030118` |

完整 SBOM、Trivy JSON、锁文件审计、镜像 inspect、许可证摘要和 `SHA256SUMS` 保存在仓库外证据目录，不提交镜像 tar、扫描缓存或大体积机器报告。

## Go 漏洞例外

govulncheck 使用 Go 1.25.13 工具链语义完成，退出码为 0：可达漏洞 0、已导入漏洞 0。唯一模块级 finding 为 `GO-2026-5932`，路径仅包含 `golang.org/x/crypto@v0.55.0`，项目没有导入或调用已废弃且无修复版本的 `openpgp` 包。该项保留为模块级观察，不作为可达漏洞例外放行。

## 许可证复核

- Web、Reader 和 Server 的强 copyleft 候选分别为 19、11 和 12 个，均来自 Alpine OS 包或其多许可证表达式；未发现镜像依赖中的 AGPL、SSPL 或额外 BSL 候选。
- Server 的 525 个 `NOASSERTION` 主要来自静态 Go 二进制只携带模块身份、不携带许可证正文的限制。它们是“无法由当前二进制证据判定”，不是“已确认无许可证”或“已确认违规”。
- 生产分发前必须由负责人保存第三方许可证复核结果、所需 notice/source-offer 履约方式和 GVA 商业授权证明。
- 根 `LICENSE` 的 BSL 1.1 Production Use 商业授权是独立强制阻断，技术测试、SBOM 或漏洞为 0 均不能替代。

## 复现命令

```bash
SERVER_IMAGE=moonbook/server:task9 \
WEB_IMAGE=moonbook/web:task9-v2 \
READER_IMAGE=moonbook/reader-ui:task9-v2 \
scripts/scan-supply-chain.sh /absolute/path/outside/repository
```

## 服务影响

本报告和扫描脚本不改变运行时服务。对应镜像整改影响 `server`、`web` 和 `reader-ui`：部署时需重建并替换三个镜像；`server` 与 `reader-ui` 需重启，`web` 需由 Gateway 重新加载新静态容器。PostgreSQL、Redis 和 MinIO 无需因本批次单独重启。
