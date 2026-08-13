import type { LoaderFunctionArgs, MetaFunction } from 'react-router';
import { useLoaderData } from 'react-router';
import { App } from '../../src/App';
import { SsrDataProvider, type ReaderSsrData } from '../../src/seo/SsrDataContext';
import {
  getBook,
  getBooks,
  getCategories,
  getChapters,
  getFeatured,
  getRandomBooks,
  getSeoConfig,
  ServerApiError
} from '../lib/readerApi.server';
import { bookSeo, booksSeo, indexingAllowed, safeJsonLd, safeSeoConfig, type ReaderSeoConfig } from '../lib/seo';

const PRIVATE_ROUTES = [
  /^\/shelf\/?$/,
  /^\/read\/[^/]+\/?$/,
  /^\/auth\/(login|register)\/?$/,
  /^\/me(?:\/.*)?$/
];

const BOOK_QUERY_KEYS = ['keyword', 'category', 'subCategory', 'sort'] as const;

export function hasBookQuery(search: URLSearchParams) {
  return BOOK_QUERY_KEYS.some((key) => Boolean(search.get(key)?.trim()));
}

interface SiteLoaderData {
  path: string;
  hasQuery: boolean;
  seo: ReaderSeoConfig;
  title: string;
  description: string;
  canonical?: string;
  robots: string;
  ssrData: ReaderSsrData;
  jsonLd?: unknown;
}

export async function loader({ request }: LoaderFunctionArgs): Promise<SiteLoaderData> {
  const url = new URL(request.url);
  const path = url.pathname.replace(/\/$/, '') || '/';
  const isPublic = path === '/' || path === '/books' || /^\/books\/[^/]+$/.test(path);
  if (!isPublic && !PRIVATE_ROUTES.some((pattern) => pattern.test(path))) {
    throw new Response('Not Found', { status: 404 });
  }

  if (!isPublic) {
    return {
      path,
      hasQuery: Boolean(url.search),
      seo: safeSeoConfig,
      title: `月白书城`,
      description: safeSeoConfig.defaultDescription,
      robots: 'noindex,nofollow',
      ssrData: {}
    };
  }

  try {
    const seo = await getSeoConfig();
    const allowIndex = indexingAllowed(seo);
    if (path === '/') {
      const [featured, categories, subCategories] = await Promise.all([
        getFeatured(), getCategories(), getCategories('/reader/books/sub-categories').catch(() => [])
      ]);
      return {
        path, hasQuery: false, seo, title: seo.homeTitle, description: seo.homeDescription,
        canonical: `${seo.siteUrl}/`, robots: allowIndex ? 'index,follow' : 'noindex,nofollow',
        ssrData: { home: { featured, categories, subCategories } },
        jsonLd: {
          '@context': 'https://schema.org', '@type': 'WebSite', name: seo.siteName, url: `${seo.siteUrl}/`,
          potentialAction: { '@type': 'SearchAction', target: `${seo.siteUrl}/books?keyword={search_term_string}`, 'query-input': 'required name=search_term_string' }
        }
      };
    }
    if (path === '/books') {
      const paged = hasBookQuery(url.searchParams);
      const [result, categories, subCategories] = await Promise.all([
        paged
          ? getBooks(url.searchParams)
          : getRandomBooks().then((rows) => ({ rows, total: rows.length })),
        getCategories(),
        getCategories('/reader/books/sub-categories').catch(() => [])
      ]);
      const rendered = booksSeo(seo, url.searchParams, categories, subCategories);
      const title = rendered.title;
      const description = rendered.description || seo.defaultDescription;
      return {
        path, hasQuery: Boolean(url.search), seo, title, description, canonical: `${seo.siteUrl}/books`,
        robots: allowIndex && !url.search ? 'index,follow' : 'noindex,follow',
        ssrData: { books: { mode: paged ? 'paged' : 'random', result, categories, subCategories } },
        jsonLd: {
          '@context': 'https://schema.org', '@type': 'CollectionPage', name: title, url: `${seo.siteUrl}/books`,
          breadcrumb: { '@type': 'BreadcrumbList', itemListElement: [
            { '@type': 'ListItem', position: 1, name: seo.siteName, item: `${seo.siteUrl}/` },
            { '@type': 'ListItem', position: 2, name: '书库', item: `${seo.siteUrl}/books` }
          ] }
        }
      };
    }

    const bookId = path.slice('/books/'.length);
    const [book, chapters] = await Promise.all([getBook(bookId), getChapters(bookId)]);
    if (!book) throw new Response('Not Found', { status: 404 });
    const rendered = bookSeo(seo, book);
    const canonical = `${seo.siteUrl}/books/${encodeURIComponent(bookId)}`;
    return {
      path, hasQuery: Boolean(url.search), seo, title: rendered.title, description: rendered.description || seo.defaultDescription,
      canonical, robots: allowIndex ? 'index,follow' : 'noindex,nofollow', ssrData: { bookDetail: { book, chapters } },
      jsonLd: {
        '@context': 'https://schema.org', '@type': 'Book', name: book.bookName, author: { '@type': 'Person', name: book.authorName || '未知作者' },
        description: book.bookDesc || seo.defaultDescription, genre: book.categoryName || undefined, url: canonical
      }
    };
  } catch (error) {
    if (error instanceof Response) throw error;
    if (error instanceof ServerApiError && /不存在|未找到|已下架/.test(error.message)) {
      throw new Response('Not Found', { status: 404 });
    }
    throw new Response('Service Unavailable', { status: 503, headers: { 'Retry-After': '60' } });
  }
}

export const meta: MetaFunction<typeof loader> = ({ data }) => {
  if (!data) return [{ title: '页面暂时无法访问 - 月白书城' }, { name: 'robots', content: 'noindex,nofollow' }];
  const tags: ReturnType<MetaFunction> = [
    { title: data.title },
    { name: 'description', content: data.description },
    { name: 'robots', content: data.robots },
    { property: 'og:title', content: data.title },
    { property: 'og:description', content: data.description },
    { property: 'og:type', content: data.path.startsWith('/books/') ? 'book' : 'website' }
  ];
  if (data.canonical) tags.push({ tagName: 'link', rel: 'canonical', href: data.canonical });
  return tags;
};

export default function SiteRoute() {
  const data = useLoaderData<typeof loader>();
  return (
    <SsrDataProvider data={data.ssrData}>
      <App />
      {data.jsonLd ? <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: safeJsonLd(data.jsonLd) }} /> : null}
    </SsrDataProvider>
  );
}
