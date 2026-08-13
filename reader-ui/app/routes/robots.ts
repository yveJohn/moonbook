import { proxySeoResource } from '../lib/readerApi.server';

export async function loader() {
  return proxy('/reader/seo/robots.txt');
}

async function proxy(path: string) {
  try {
    const upstream = await proxySeoResource(path);
    return new Response(await upstream.text(), { status: upstream.status, headers: forwardedHeaders(upstream) });
  } catch {
    return new Response('User-agent: *\nDisallow: /\n', {
      status: 503,
      headers: { 'Content-Type': 'text/plain; charset=utf-8', 'Cache-Control': 'no-store' }
    });
  }
}

export function forwardedHeaders(response: Response) {
  const headers = new Headers();
  for (const name of ['content-type', 'cache-control', 'etag']) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }
  return headers;
}
