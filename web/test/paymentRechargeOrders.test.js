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

test('recharge order list defaults to ten rows and renders localized status and time', () => {
  assert.match(page, /const pageSize\s*=\s*ref\(10\)/)

  for (const [value, label] of [
    ['creating', '创建中'],
    ['pending', '待支付'],
    ['gateway_unknown', '网关状态未知'],
    ['create_failed', '创建失败'],
    ['superseded', '已替换'],
    ['expired', '已过期'],
    ['callback_exception', '回调异常'],
    ['paid', '已支付']
  ]) {
    assert.ok(page.includes(`value:'${value}',label:'${label}'`), `missing localized recharge status: ${value}`)
  }

  assert.match(page, /:label="item\.label"\s+:value="item\.value"/)
  assert.ok(page.includes('statusLabel(row.status)'), 'list status must use the shared label formatter')
  assert.ok(page.includes('statusLabel(detail.status)'), 'detail status must use the shared label formatter')
  assert.match(page, /const statusLabel=\(value\)=>statusOptions\.find\(item=>item\.value===value\)\?\.label\|\|value\|\|'-'/)
  assert.match(page, /import \{ formatDate \} from '@\/utils\/format'/)
  assert.ok(page.includes("row.createdAt ? formatDate(row.createdAt) : '-'"), 'created time must be formatted with an empty fallback')
})
