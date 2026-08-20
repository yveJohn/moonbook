import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const pagePath = new URL('../src/view/reader/payment/callbackLogs/index.vue', import.meta.url)
const apiPath = new URL('../src/api/reader/paymentCallbackLogs.js', import.meta.url)

test('payment callback audit page keeps Long IDs as strings and exposes read-only audit controls', async () => {
  const [page, api] = await Promise.all([readFile(pagePath, 'utf8'), readFile(apiPath, 'utf8')])
  for (const contract of [
    'signatureStatus', 'failureCode', 'responseStatus', 'timeRange', '处理中断', 'not_checked',
    'getPaymentCallbackLog', 'payloadHash', 'payloadBytes', 'payloadTruncated', 'snapshotItems'
  ]) {
    assert.ok(page.includes(contract), `missing callback audit contract: ${contract}`)
  }
  assert.match(api, /appendLongId\('\/reader\/payment\/callbackLogs', id\)/)
  assert.doesNotMatch(page, /\b(?:Number|parseInt)\s*\(/)
  assert.doesNotMatch(page, /el-input-number/)
  assert.doesNotMatch(page, /manualPay|replayCallback|补单|重放回调/)
})
