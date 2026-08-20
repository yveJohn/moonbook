import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const pagePath = new URL('../src/view/novel/crawlSources/index.vue', import.meta.url)

test('forum source page only edits controlled cookie secret references', async () => {
  const page = await readFile(pagePath, 'utf8')
  assert.match(page, /cookieSecretRef/)
  assert.match(page, /MOONBOOK_FORUM_COOKIE_\[A-Z0-9_\]/)
  assert.doesNotMatch(page, /cookieText|Cookie（留空保持原值）|maxlength="8192"/)
  assert.doesNotMatch(page, /\b(?:Number|parseInt)\s*\(|el-input-number/)
})
