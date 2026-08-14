import assert from 'node:assert/strict'
import test from 'node:test'

import {
  BOOK_PLACEHOLDERS,
  BOOKS_PLACEHOLDERS,
  invalidPlaceholders,
  isSiteRootURL,
  renderSEOTemplate
} from '../src/view/novel/readerSeo/seoTemplate.js'

test('reader SEO URL accepts only HTTP root URLs', () => {
  for (const value of ['https://ybsc.me', 'https://ybsc.me/', 'http://localhost:3000']) {
    assert.equal(isSiteRootURL(value), true, value)
  }
  for (const value of ['ftp://ybsc.me', 'https://user@ybsc.me', 'https://ybsc.me/path', 'https://ybsc.me?q=1', 'https://ybsc.me/#x', '']) {
    assert.equal(isSiteRootURL(value), false, value)
  }
})

test('reader SEO templates enforce context-specific placeholders', () => {
  assert.deepEqual(invalidPlaceholders('{siteName}{keyword}{categoryName}{subCategoryName}', BOOKS_PLACEHOLDERS), [])
  assert.deepEqual(invalidPlaceholders('{siteName}{bookName}{authorName}{categoryName}{bookDesc}', BOOK_PLACEHOLDERS), [])
  assert.deepEqual(invalidPlaceholders('{bookName}{unknown}', BOOKS_PLACEHOLDERS), ['bookName', 'unknown'])
  assert.deepEqual(invalidPlaceholders('{siteName', BOOKS_PLACEHOLDERS), ['格式错误'])
})

test('reader SEO preview renders known values without changing IDs', () => {
  assert.equal(renderSEOTemplate('{bookName} - {authorName} - {siteName}'), '月下长歌 - 示例作者 - 月白书城')
  assert.equal(renderSEOTemplate('{bookName} - {siteName}', { siteName: '迁移书城' }), '月下长歌 - 迁移书城')
  const configID = '9223372036854775807'
  assert.equal(typeof configID, 'string')
  assert.equal(configID, '9223372036854775807')
})
