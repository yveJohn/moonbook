import type { ReaderBookDetail, ReaderCategory } from '../../src/types/reader';

export interface ReaderSeoConfig {
  seoEnabled: number;
  indexingEnabled: number;
  sitemapEnabled: number;
  siteName: string;
  siteUrl: string;
  defaultDescription: string;
  homeTitle: string;
  homeDescription: string;
  booksTitleTemplate: string;
  booksDescriptionTemplate: string;
  bookTitleTemplate: string;
  bookDescriptionTemplate: string;
}

export const safeSeoConfig: ReaderSeoConfig = {
  seoEnabled: 0,
  indexingEnabled: 0,
  sitemapEnabled: 0,
  siteName: '月白书城',
  siteUrl: 'https://ybsc.me',
  defaultDescription: '月白书城提供精选小说在线阅读与作品发现。',
  homeTitle: '月白书城 - 精选小说在线阅读',
  homeDescription: '在月白书城发现精选小说，浏览作品分类并开始阅读。',
  booksTitleTemplate: '书库 - {siteName}',
  booksDescriptionTemplate: '浏览{siteName}书库，发现不同分类的精选小说。',
  bookTitleTemplate: '{bookName} - {authorName} - {siteName}',
  bookDescriptionTemplate: '《{bookName}》由{authorName}创作，{bookDesc}'
};

export function renderTemplate(template: string, values: Record<string, string>, maxCodePoints: number) {
  const rendered = template
    .replace(/\{([A-Za-z][A-Za-z0-9]*)}/g, (_, name: string) => values[name]?.trim() || '')
    .trim()
    .replace(/\s+/g, ' ')
    .replace(/(?:\s*[-|·]\s*){2,}/g, ' - ')
    .replace(/^[\s\-|·]+|[\s\-|·]+$/g, '')
    .trim();
  return Array.from(rendered).slice(0, maxCodePoints).join('').trim();
}

export function bookSeo(config: ReaderSeoConfig, book: ReaderBookDetail) {
  const values = {
    siteName: config.siteName,
    bookName: book.bookName || '',
    authorName: book.authorName || '',
    categoryName: book.categoryName || '',
    bookDesc: book.bookDesc || ''
  };
  return {
    title: renderTemplate(config.bookTitleTemplate, values, 120),
    description: renderTemplate(config.bookDescriptionTemplate, values, 320)
  };
}

export function booksSeo(
  config: ReaderSeoConfig,
  search: URLSearchParams,
  categories: ReaderCategory[],
  subCategories: ReaderCategory[]
) {
  const categoryCode = search.get('categoryCode') || search.get('category') || '';
  const subCategoryCode = search.get('subCategoryCode') || search.get('subCategory') || '';
  const values = {
    siteName: config.siteName,
    keyword: (search.get('keyword') || '').trim(),
    categoryName: categories.find((item) => item.categoryCode === categoryCode)?.categoryName || '',
    subCategoryName: subCategories.find((item) => item.categoryCode === subCategoryCode)?.categoryName || ''
  };
  return {
    title: renderTemplate(config.booksTitleTemplate, values, 120),
    description: renderTemplate(config.booksDescriptionTemplate, values, 320)
  };
}

export function indexingAllowed(config: ReaderSeoConfig) {
  return config.seoEnabled === 1 && config.indexingEnabled === 1;
}

export function safeJsonLd(value: unknown) {
  return JSON.stringify(value).replace(/</g, '\\u003c');
}
