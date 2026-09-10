// Display-only conversions: never replace the values sent to the API.
const statusLabels = {
  cleaning: '清洗中', cleaned: '已清洗', create_failed: '创建失败', superseded: '已替代',
  callback_exception: '回调异常', depleted: '已耗尽', granted: '已发放', confirmed: '已确认', retry: '等待重试',
  pending_review: '待审核', pending: '待处理', running: '运行中', started: '已开始', success: '成功', succeeded: '成功',
  failed: '失败', failure: '失败', error: '错误', cancelled: '已取消', canceled: '已取消',
  stopped: '已停止', skipped: '已跳过', importing: '导入中', imported: '已导入',
  enabled: '启用', disabled: '停用', deleted: '已删除', active: '有效', inactive: '未启用',
  blocked: '已封禁', expired: '已过期', replied: '已回复', approved: '已审核', rejected: '已拒绝',
  applied: '已应用', retried: '已重试', recovered: '已恢复', discarded: '已丢弃',
  manual_discarded: '人工丢弃', manual_success: '人工通过', processing: '处理中', queued: '排队中',
  draft: '草稿', published: '已上架', deprecated: '已下架', serializing: '连载中', completed: '已完成',
  on_sale: '在售', off_sale: '停售', paid: '已支付', unpaid: '未支付', refunded: '已退款',
  closed: '已关闭', creating: '创建中', gateway_unknown: '支付网关结果待确认',
  normal: '正常', ok: '正常', warning: '警告', abnormal: '异常', unknown: '未知',
  passed: '通过', rejected_quality: '质量不合格', unchecked: '未检查', checking: '检查中',
  not_checked: '未检查', good: '良好', poor: '较差', empty: '空内容', partial: '部分完成'
}
export function adminStatus(value) {
  if (value === null || value === undefined || value === '') return '-'
  if (typeof value === 'boolean') return value ? '启用' : '停用'
  if (typeof value !== 'string') return String(value)
  return statusLabels[value.toLowerCase()] || (/^[a-z][a-z_ -]*$/i.test(value) ? '未知状态' : value)
}
export function adminDateTime(value) {
  if (value === null || value === undefined || value === '') return '-'
  // A calendar date has no timezone. Parse it locally instead of treating it as UTC.
  const input = typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value) ? `${value}T00:00:00` : value
  const date = new Date(input)
  if (Number.isNaN(date.getTime())) return '-'
  const pad = n => String(n).padStart(2, '0')
  return `${String(date.getFullYear()).padStart(4, '0')}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
export const adminStatusColumn = (_row, _column, value) => adminStatus(value)
export const adminTimeColumn = (_row, _column, value) => adminDateTime(value)
export const bookLabel = (id, name) => id ? `${name || '作品不可用'}（${id}）` : '-'
