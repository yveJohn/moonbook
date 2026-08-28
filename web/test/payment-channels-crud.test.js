import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const page = await readFile(new URL('../src/view/reader/payment/channels/index.vue', import.meta.url), 'utf8')
const api = await readFile(new URL('../src/api/reader/paymentChannels.js', import.meta.url), 'utf8')

test('payment channels expose complete CRUD and keep long ids as strings', () => {
  for (const method of ['createPaymentChannel', 'updatePaymentChannel', 'archivePaymentChannel', 'checkPaymentChannel']) {
    assert.match(api, new RegExp(`export const ${method}`))
    assert.match(page, new RegExp(method))
  }
  assert.match(api, /appendLongId/)
  assert.doesNotMatch(page, /Number\([^)]*\.id\)|parseInt\([^)]*\.id\)|el-input-number/)
})

test('payment channel credentials are write-only in the management form', () => {
  assert.match(page, /merchantPid: ''/)
  assert.match(page, /secret: ''/)
  assert.match(page, /type="password"/)
  assert.match(page, /留空保持不变/)
  assert.doesNotMatch(page, /row\.merchantPid(?![A-Za-z])|row\.secret(?![A-Za-z])/)
})
