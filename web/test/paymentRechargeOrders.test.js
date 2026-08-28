import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const page = await readFile(new URL('../src/view/reader/payment/orders/index.vue', import.meta.url), 'utf8')
const api = await readFile(new URL('../src/api/reader/rechargeOrders.js', import.meta.url), 'utf8')

test('recharge order detail exposes failure diagnostics and preserves Long IDs', () => {
  for (const contract of ['失败码', '失败原因', 'detail.failureCode', 'detail.failureMessage']) {
    assert.ok(page.includes(contract), `missing recharge order diagnostic contract: ${contract}`)
  }
  assert.match(api, /appendLongId/)
  assert.doesNotMatch(page, /\b(?:Number|parseInt)\s*\(/)
  assert.doesNotMatch(page, /el-input-number/)
})
