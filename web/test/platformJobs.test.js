import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const pagePath = new URL('../src/view/platform/jobs/index.vue', import.meta.url)
const apiPath = new URL('../src/api/platform.js', import.meta.url)

test('platform jobs page is read-only and preserves Long IDs', async () => {
  const [page, api] = await Promise.all([readFile(pagePath, 'utf8'), readFile(apiPath, 'utf8')])
  for (const contract of ['PlatformJobs', 'timeRange', 'attempts', 'leaseOwner', 'lastErrorMessage', 'datetimerange', '查看详情']) {
    assert.ok(page.includes(contract), `missing platform job contract: ${contract}`)
  }
  assert.match(api, /appendLongId\('\/platform\/jobs', id\)/)
  assert.doesNotMatch(page, /\b(?:Number|parseInt)\s*\(/)
  assert.doesNotMatch(page, /el-input-number|v-html|retryJob|cancelJob|deleteJob|重跑|取消任务|删除任务/)
})
