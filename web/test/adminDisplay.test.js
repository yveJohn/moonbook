import test from 'node:test'
import assert from 'node:assert/strict'
import { adminDateTime, adminStatus, bookLabel } from '../src/utils/adminDisplay.js'

test('timestamps display local calendar components, including afternoon and midnight', () => {
  assert.equal(adminDateTime(new Date(2026, 8, 11, 15, 4, 5)), '2026-09-11 15:04:05')
  assert.equal(adminDateTime(new Date(2026, 0, 2, 0, 0, 0)), '2026-01-02 00:00:00')
  assert.equal(adminDateTime('2026-09-11'), '2026-09-11 00:00:00')
  const instant = new Date('2026-09-11T07:04:05Z')
  assert.equal(adminDateTime(instant.toISOString()), adminDateTime(instant))
  for (const missing of [null, undefined, '', 'invalid']) assert.equal(adminDateTime(missing), '-')
  assert.notEqual(adminDateTime(0), '-')
})

test('workflow, quality, reader and publication states have Chinese display labels', () => {
  for (const status of ['pending', 'running', 'succeeded', 'success', 'failed', 'cancelled', 'retry',
    'started', 'importing', 'imported', 'skipped', 'enabled', 'disabled', 'deleted', 'replied',
    'passed', 'warning', 'draft', 'published', 'deprecated', 'pending_review', 'manual_discarded',
    'active', 'blocked', 'expired', 'granted', 'depleted', 'confirmed', 'paid', 'refunded']) {
    assert.match(adminStatus(status), /[\u4e00-\u9fff]/)
    assert.notEqual(adminStatus(status), '未知状态', status)
  }
  assert.equal(adminStatus('FAILED'), '失败')
  assert.equal(adminStatus('未来状态'), '未来状态')
  assert.equal(adminStatus('future_state'), '未知状态')
  assert.equal(adminStatus(200), '200') // HTTP status codes remain numeric.
  assert.equal(adminStatus(null), '-')
})

test('work references preserve snowflake IDs exactly and distinguish unavailable works', () => {
  const id = '9223372036854775807'
  assert.equal(bookLabel(id, '同名作品'), `同名作品（${id}）`)
  assert.equal(bookLabel(id, ''), `作品不可用（${id}）`)
  assert.equal(bookLabel('', ''), '-')
  assert.equal(bookLabel('9007199254740993', '长篇'), '长篇（9007199254740993）')
})
