import assert from 'node:assert/strict'
import test from 'node:test'

import { buildTrendSeries, compareCounts, formatCount, normalizeCount } from '../src/view/dashboard/dashboardMetrics.js'

test('dashboard counts are normalized without changing aggregate meaning', () => {
  assert.equal(normalizeCount('1234'), 1234)
  assert.equal(normalizeCount(-1), 0)
  assert.equal(normalizeCount('invalid'), 0)
  assert.equal(formatCount(1234567), '1,234,567')
})

test('dashboard comparison handles zero baseline and percentage changes', () => {
  assert.deepEqual(compareCounts(8, 0), { direction: 'up', text: '昨日为 0' })
  assert.deepEqual(compareCounts(12, 10), { direction: 'up', text: '较昨日 20%' })
  assert.deepEqual(compareCounts(9, 10), { direction: 'down', text: '较昨日 10%' })
  assert.deepEqual(compareCounts(10, 10), { direction: 'flat', text: '与昨日持平' })
})

test('dashboard trend preserves date ordering and zero values', () => {
  assert.deepEqual(buildTrendSeries([
    { date: '2026-08-13', newUserCount: 2, activeUserCount: 4 },
    { date: '2026-08-14', newUserCount: 0, activeUserCount: 3 }
  ]), {
    dates: ['2026-08-13', '2026-08-14'],
    newUsers: [2, 0],
    activeUsers: [4, 3]
  })
})
