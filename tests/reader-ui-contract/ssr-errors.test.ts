import { afterEach, describe, expect, it, vi } from 'vitest';
import { getSeoConfig, ServerApiError } from '../../reader-ui/app/lib/readerApi.server';
import { loader as siteLoader } from '../../reader-ui/app/routes/site';
import { loader as robotsLoader } from '../../reader-ui/app/routes/robots';
import { loader as sitemapLoader } from '../../reader-ui/app/routes/sitemap';
import { loader as sitemapBooksLoader } from '../../reader-ui/app/routes/sitemap-books';

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('frozen Reader SSR upstream failures', () => {
  it('maps the 12 second JSON upstream timeout to service unavailable', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('fetch', vi.fn((_url: string | URL | Request, init?: RequestInit) => (
      new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
      })
    )));

    const assertion = expect(getSeoConfig()).rejects.toMatchObject<Partial<ServerApiError>>({
      status: 503,
      code: 503
    });
    await vi.advanceTimersByTimeAsync(12_000);

    await assertion;
  });

  it('fails the public SSR page closed when a successful upstream response is not JSON', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => { throw new SyntaxError('Unexpected token <'); }
    }) as Response));

    await expect(siteLoader({
      request: new Request('https://ybsc.me/'),
      params: {},
      context: {}
    })).rejects.toMatchObject({ status: 503 });
  });
});

describe('frozen Reader SEO proxy failures', () => {
  it('returns a fail-closed robots response when the upstream aborts', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new DOMException('aborted', 'AbortError'); }));

    const response = await robotsLoader();

    expect(response.status).toBe(503);
    expect(response.headers.get('content-type')).toBe('text/plain; charset=utf-8');
    expect(response.headers.get('cache-control')).toBe('no-store');
    await expect(response.text()).resolves.toBe('User-agent: *\nDisallow: /\n');
  });

  it('returns a retryable empty sitemap response when the upstream aborts', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new DOMException('aborted', 'AbortError'); }));

    const response = await sitemapLoader();

    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
    expect(response.headers.get('cache-control')).toBe('no-store');
    await expect(response.text()).resolves.toBe('');
  });

  it('rejects invalid sitemap pages before calling the upstream', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    const response = await sitemapBooksLoader({ params: { page: 'invalid' } } as never);

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('returns 503 for a valid sitemap page when the upstream aborts', async () => {
    const fetchMock = vi.fn(async () => { throw new DOMException('aborted', 'AbortError'); });
    vi.stubGlobal('fetch', fetchMock);

    const response = await sitemapBooksLoader({ params: { page: '2' } } as never);

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:55328/reader/seo/sitemap-books-2.xml',
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
    expect(response.headers.get('cache-control')).toBe('no-store');
  });
});
