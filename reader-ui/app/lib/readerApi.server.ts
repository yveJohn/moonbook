import type {
  ReaderBookCatalogItem,
  ReaderBookDetail,
  ReaderBookSummary,
  ReaderCategory,
  ReaderChapterSummary,
  ReaderPageResult
} from '../../src/types/reader';
import type { ReaderSeoConfig } from './seo';

interface ApiBody<T> {
  code?: number;
  msg?: string;
  data?: T;
  rows?: T extends Array<infer Item> ? Item[] : never;
  total?: number;
}

export class ServerApiError extends Error {
  constructor(public readonly status: number, public readonly code: number, message: string) {
    super(message);
    this.name = 'ServerApiError';
  }
}

function apiOrigin() {
  return (process.env.READER_API_ORIGIN || 'http://127.0.0.1:55328').replace(/\/$/, '');
}

async function request<T>(path: string): Promise<ApiBody<T>> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 12_000);
  try {
    const response = await fetch(`${apiOrigin()}${path}`, {
      headers: { Accept: 'application/json' },
      signal: controller.signal
    });
    const body = await response.json().catch(() => ({})) as ApiBody<T>;
    if (!response.ok || (body.code && body.code !== 200)) {
      throw new ServerApiError(response.status || 503, body.code || response.status, body.msg || '后端请求失败');
    }
    return body;
  } catch (error) {
    if (error instanceof ServerApiError) throw error;
    throw new ServerApiError(503, 503, error instanceof Error ? error.message : '后端请求失败');
  } finally {
    clearTimeout(timeout);
  }
}

export async function getSeoConfig(): Promise<ReaderSeoConfig> {
  const body = await request<ReaderSeoConfig>('/reader/seo/config');
  return body.data as ReaderSeoConfig;
}

export async function getFeatured(): Promise<ReaderBookSummary[]> {
  return (await request<ReaderBookSummary[]>('/reader/books/featured')).data || [];
}

export async function getCategories(path = '/reader/books/categories'): Promise<ReaderCategory[]> {
  return (await request<ReaderCategory[]>(path)).data || [];
}

export async function getBooks(search: URLSearchParams): Promise<ReaderPageResult<ReaderBookCatalogItem>> {
  const params = new URLSearchParams(search);
  const categoryCode = params.get('category');
  const subCategoryCode = params.get('subCategory');
  params.delete('category');
  params.delete('subCategory');
  if (categoryCode) params.set('categoryCode', categoryCode);
  if (subCategoryCode) params.set('subCategoryCode', subCategoryCode);
  params.set('pageNum', '1');
  params.set('pageSize', '10');
  if (!params.has('sort')) params.set('sort', 'recent');
  const body = await request<ReaderBookCatalogItem[]>(`/reader/books?${params}`);
  return { rows: body.rows || [], total: body.total || 0 };
}

export async function getRandomBooks(): Promise<ReaderBookCatalogItem[]> {
  return (await request<ReaderBookCatalogItem[]>('/reader/books/random')).data || [];
}

export async function getBook(bookId: string): Promise<ReaderBookDetail> {
  return (await request<ReaderBookDetail>(`/reader/books/${encodeURIComponent(bookId)}`)).data as ReaderBookDetail;
}

export async function getChapters(bookId: string): Promise<ReaderChapterSummary[]> {
  return (await request<ReaderChapterSummary[]>(`/reader/books/${encodeURIComponent(bookId)}/chapters`)).data || [];
}

export async function proxySeoResource(path: string) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 12_000);
  try {
    return await fetch(`${apiOrigin()}${path}`, { signal: controller.signal });
  } finally {
    clearTimeout(timeout);
  }
}
