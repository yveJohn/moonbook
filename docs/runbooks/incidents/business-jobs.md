# 支付、任务与迁移事件

1. 支付告警先检查 callback metrics collection 是否成功，再核对滞留 received、503、rejected/failed 和幂等重试。
2. 不手工修改订单、钱包、流水、会员或权益；使用只读全域财务核对器确认差异，并按支付回调 Runbook 处理。
3. 任务告警按 module/job type/status 聚合检查 attempt 和 lease；租约过期先确认旧 Worker 已退出，再由受控 Worker 恢复流程重领。
4. 迁移错误或对象核对失败立即阻止切换，保留 checkpoint、脱敏错误和对象哈希证据；禁止跳过错误继续宣布完成。
5. 恢复后确认失败/积压/过期租约归零、财务零差异，并等待 Alertmanager resolved 通知。
