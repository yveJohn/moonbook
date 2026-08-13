import { beforeEach, describe, expect, it, vi } from 'vitest';
import { safeSeoConfig } from '../lib/seo';

const readerApi = vi.hoisted(() => ({
  getBook: vi.fn(),
  getBooks: vi.fn(),
  getCategories: vi.fn(),
  getChapters: vi.fn(),
  getFeatured: vi.fn(),
  getRandomBooks: vi.fn(),
  getSeoConfig: vi.fn()
}));

vi.mock('../lib/readerApi.server', () => ({
  ...readerApi,
  ServerApiError: class ServerApiError extends Error {
    constructor(public status: number, public code: number, message: string) {
      super(message);
    }
  }
}));

import { hasBookQuery, loader } from './site';

describe('site books loader', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    readerApi.getSeoConfig.mockResolvedValue({ ...safeSeoConfig, seoEnabled: 1, indexingEnabled: 1 });
    readerApi.getCategories.mockResolvedValue([]);
    readerApi.getRandomBooks.mockResolvedValue([{ bookId: '101', bookName: '随机作品' }]);
    readerApi.getBooks.mockResolvedValue({ rows: [{ bookId: '102', bookName: '分页作品' }], total: 25 });
  });

  it('recognizes only supported book query parameters', () => {
    expect(hasBookQuery(new URLSearchParams())).toBe(false);
    expect(hasBookQuery(new URLSearchParams('utm_source=test'))).toBe(false);
    expect(hasBookQuery(new URLSearchParams('keyword=月光'))).toBe(true);
    expect(hasBookQuery(new URLSearchParams('category=13'))).toBe(true);
    expect(hasBookQuery(new URLSearchParams('sort=popular'))).toBe(true);
  });

  it('uses the random endpoint for the unfiltered books page', async () => {
    const result = await loader({
      request: new Request('https://ybsc.me/books'),
      params: {},
      context: {}
    });

    expect(readerApi.getRandomBooks).toHaveBeenCalledOnce();
    expect(readerApi.getBooks).not.toHaveBeenCalled();
    expect(result.ssrData.books).toMatchObject({ mode: 'random', result: { total: 1 } });
  });

  it('uses database paging for filtered or explicitly sorted books pages', async () => {
    const request = new Request('https://ybsc.me/books?subCategory=system&sort=popular');
    const result = await loader({ request, params: {}, context: {} });

    expect(readerApi.getBooks).toHaveBeenCalledWith(new URL(request.url).searchParams);
    expect(readerApi.getRandomBooks).not.toHaveBeenCalled();
    expect(result.ssrData.books).toMatchObject({ mode: 'paged', result: { total: 25 } });
  });
});
