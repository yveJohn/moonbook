import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const pagePath = new URL('../src/view/novel/crawlSources/index.vue', import.meta.url)
const apiPath = new URL('../src/api/novel/crawlSources.js', import.meta.url)

test('forum source page only edits controlled cookie secret references', async () => {
  const page = await readFile(pagePath, 'utf8')
  assert.match(page, /cookieSecretRef/)
  assert.match(page, /MOONBOOK_FORUM_COOKIE_\[A-Z0-9_\]/)
  assert.doesNotMatch(page, /cookieText|Cookie（留空保持原值）|maxlength="8192"/)
  assert.doesNotMatch(page, /\b(?:Number|parseInt)\s*\(|el-input-number/)
})

test('forum source connection check keeps long ids as strings and exposes stable results', async () => {
  const [page, api] = await Promise.all([readFile(pagePath, 'utf8'), readFile(apiPath, 'utf8')])
  assert.match(api, /appendLongId\('\/novel\/crawl\/sources', id\) \+ '\/check'/)
  assert.match(page, /checkCrawlSource\(row\.id\)/)
  assert.match(page, /AUTH_FAILED:'认证失败'/)
  assert.match(page, /SECRET_NOT_CONFIGURED:'Cookie Secret 未配置'/)
  assert.doesNotMatch(page + api, /\b(?:Number|parseInt)\s*\(|el-input-number/)
})
