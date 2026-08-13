import type { LoaderFunctionArgs } from 'react-router';
import { proxySeoResource } from '../lib/readerApi.server';
import { forwardedHeaders } from './robots';

export async function loader({ params }: LoaderFunctionArgs) {
  const page = params.page || '';
  if (!/^\d+$/.test(page)) return new Response(null, { status: 404 });
  try {
    const upstream = await proxySeoResource(`/reader/seo/sitemap-books-${page}.xml`);
    return new Response(await upstream.text(), { status: upstream.status, headers: forwardedHeaders(upstream) });
  } catch {
    return new Response(null, { status: 503, headers: { 'Retry-After': '60', 'Cache-Control': 'no-store' } });
  }
}
