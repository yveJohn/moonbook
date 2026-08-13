import { proxySeoResource } from '../lib/readerApi.server';
import { forwardedHeaders } from './robots';

export async function loader() {
  try {
    const upstream = await proxySeoResource('/reader/seo/sitemap.xml');
    return new Response(await upstream.text(), { status: upstream.status, headers: forwardedHeaders(upstream) });
  } catch {
    return new Response(null, { status: 503, headers: { 'Retry-After': '60', 'Cache-Control': 'no-store' } });
  }
}
