import { beforeEach, describe, expect, it, vi } from 'vitest';

const readerApi = vi.hoisted(() => ({
  proxySeoResource: vi.fn()
}));

vi.mock('../lib/readerApi.server', () => readerApi);

import { loader as robotsLoader } from './robots';
import { loader as sitemapLoader } from './sitemap';
import { loader as sitemapBooksLoader } from './sitemap-books';

describe('SEO resource proxy failures', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    readerApi.proxySeoResource.mockRejectedValue(new DOMException('aborted', 'AbortError'));
  });

  it('returns a fail-closed robots response when the upstream times out', async () => {
    const response = await robotsLoader();

    expect(response.status).toBe(503);
    expect(response.headers.get('content-type')).toBe('text/plain; charset=utf-8');
    expect(response.headers.get('cache-control')).toBe('no-store');
    await expect(response.text()).resolves.toBe('User-agent: *\nDisallow: /\n');
  });

  it('returns a retryable empty sitemap response when the upstream times out', async () => {
    const response = await sitemapLoader();

    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
    expect(response.headers.get('cache-control')).toBe('no-store');
    await expect(response.text()).resolves.toBe('');
  });

  it('rejects invalid sitemap pages before calling the upstream', async () => {
    const response = await sitemapBooksLoader({ params: { page: 'invalid' } } as never);

    expect(response.status).toBe(404);
    expect(readerApi.proxySeoResource).not.toHaveBeenCalled();
  });

  it('returns 503 for a valid sitemap page when the upstream times out', async () => {
    const response = await sitemapBooksLoader({ params: { page: '2' } } as never);

    expect(readerApi.proxySeoResource).toHaveBeenCalledWith('/reader/seo/sitemap-books-2.xml');
    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
    expect(response.headers.get('cache-control')).toBe('no-store');
  });
});
