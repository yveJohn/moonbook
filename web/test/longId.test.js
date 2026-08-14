import assert from 'node:assert/strict'
import test from 'node:test'

import { appendLongId, assertLongId } from '../src/utils/longId.js'

test('preserves bigint IDs beyond JavaScript safe integer range', () => {
  const id = '9223372036854775807'
  assert.equal(assertLongId(id), id)
  assert.equal(appendLongId('/novel/authors', id), `/novel/authors/${id}`)
  assert.equal(appendLongId('/novel/categories', '9007199254740993'), '/novel/categories/9007199254740993')
  assert.equal(appendLongId('/novel/books', '9007199254740995'), '/novel/books/9007199254740995')
  assert.equal(appendLongId('/novel/chapters', '9007199254740997'), '/novel/chapters/9007199254740997')
})

test('rejects number conversion and non-decimal ID forms', () => {
  for (const value of [Number.MAX_SAFE_INTEGER, Number.MAX_SAFE_INTEGER + 1, 0, '0', '+1', '1.0', ' 1', '', null, undefined]) {
    assert.throws(() => assertLongId(value), TypeError)
  }
})
