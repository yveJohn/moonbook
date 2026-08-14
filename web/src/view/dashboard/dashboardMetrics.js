export const normalizeCount = (value) => {
  const count = Number(value)
  return Number.isFinite(count) && count >= 0 ? Math.trunc(count) : 0
}

export const formatCount = (value) => normalizeCount(value).toLocaleString('zh-CN')

export const compareCounts = (current, previous) => {
  const now = normalizeCount(current)
  const before = normalizeCount(previous)
  if (now === before) return { direction: 'flat', text: '与昨日持平' }
  const direction = now > before ? 'up' : 'down'
  if (before === 0) return { direction, text: now > 0 ? '昨日为 0' : '较昨日下降' }
  const percent = Math.abs(((now - before) / before) * 100)
  return { direction, text: `较昨日 ${percent.toFixed(percent >= 10 ? 0 : 1)}%` }
}

export const buildTrendSeries = (dailyTrend = []) => ({
  dates: dailyTrend.map((item) => String(item?.date || '')),
  newUsers: dailyTrend.map((item) => normalizeCount(item?.newUserCount)),
  activeUsers: dailyTrend.map((item) => normalizeCount(item?.activeUserCount))
})
