import { getNovelBook } from '@/api/novel/books'
// Only share in-flight lookups; names must be refreshed after a work is renamed.
const pending = new Map()
export function resolveBook(id) {
  if (!/^[1-9]\d*$/.test(id || '')) return Promise.resolve(null)
  if (!pending.has(id)) {
    pending.set(id, getNovelBook(id).then(response => response.code === 0 ? response.data : null)
      .catch(() => null).finally(() => pending.delete(id)))
  }
  return pending.get(id)
}
