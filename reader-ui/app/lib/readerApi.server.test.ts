import { afterEach, describe, expect, it, vi } from 'vitest';
import { getBooks, getRandomBooks } from './readerApi.server';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('reader server book API', () => {
  it('loads random books from the dedicated endpoint', async () => {
    let requestedUrl = '';
    vi.stubGlobal('fetch', vi.fn(async (url: string | URL | Request) => {
      requestedUrl = String(url);
      return {
        ok: true,
        status: 200,
        json: async () => ({
          code: 200,
          data: [{ bookId: '9223372036854775807', bookName: '随机作品' }]
        })
      } as Response;
    }));

    const result = await getRandomBooks();

    expect(requestedUrl).toBe('http://127.0.0.1:55328/reader/books/random');
    expect(result[0]?.bookId).toBe('9223372036854775807');
  });

  it('maps public URL category names to backend query parameters', async () => {
    let requestedUrl = '';
    vi.stubGlobal('fetch', vi.fn(async (url: string | URL | Request) => {
      requestedUrl = String(url);
      return {
        ok: true,
        status: 200,
        json: async () => ({ code: 200, rows: [], total: 0 })
      } as Response;
    }));

    await getBooks(new URLSearchParams('category=13&subCategory=system'));

    const requested = new URL(requestedUrl);
    expect(requested.pathname).toBe('/reader/books');
    expect(Object.fromEntries(requested.searchParams)).toEqual({
      categoryCode: '13',
      subCategoryCode: 'system',
      pageNum: '1',
      pageSize: '10',
      sort: 'recent'
    });
  });
});
