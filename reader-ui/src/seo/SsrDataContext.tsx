import { createContext, useContext } from 'react';
import type {
  ReaderBookCatalogItem,
  ReaderBookDetail,
  ReaderBookSummary,
  ReaderCategory,
  ReaderChapterSummary,
  ReaderPageResult
} from '../types/reader';

export interface ReaderSsrData {
  home?: {
    featured: ReaderBookSummary[];
    categories: ReaderCategory[];
    subCategories: ReaderCategory[];
  };
  books?: {
    mode: 'random' | 'paged';
    result: ReaderPageResult<ReaderBookCatalogItem>;
    categories: ReaderCategory[];
    subCategories: ReaderCategory[];
  };
  bookDetail?: {
    book: ReaderBookDetail;
    chapters: ReaderChapterSummary[];
  };
}

const SsrDataContext = createContext<ReaderSsrData>({});

export function SsrDataProvider({ data, children }: { data: ReaderSsrData; children: React.ReactNode }) {
  return <SsrDataContext.Provider value={data}>{children}</SsrDataContext.Provider>;
}

export function useSsrData() {
  return useContext(SsrDataContext);
}
