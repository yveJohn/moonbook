# MinIO 与内容对象事件

1. 核对 MinIO live/ready、离线盘、容量和请求错误，区分指标缺失与对象服务不可用。
2. 对业务报错对象使用对象核对器按引用执行 Stat/Get、字节和 SHA-256 校验；报告不得输出对象键、正文或签名 URL。
3. 禁止直接删除 `uploading`、`orphaned`、`failed` 或 `deleting` 对象；先核对 PostgreSQL 引用和 GC 状态。
4. 容量告警时停止非必要批量导入和迁移，按协调备份 Runbook 扩容；不要清理未知项目卷。
5. 恢复后抽查 Reader 正文、封面和 TXT 原文件读取，并确认对象与 MinIO 告警 resolved。
