import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Card, Input } from 'animal-island-ui';
import { RefreshCw } from 'lucide-react';
import { listCategories, listRandomBooks, listSubCategories, queryBooks } from '../api/reader';
import { AppShell } from '../components/AppShell';
import { BookCategoryNav } from '../components/BookCategoryNav';
import { BookTextCard } from '../components/BookTextCard';
import type { ReaderBookCatalogItem, ReaderBookSort, ReaderCategory } from '../types/reader';
import { useSsrData } from '../seo/SsrDataContext';

const PAGE_SIZE = 10;
const SORT_OPTIONS = [
  { key: 'recent', label: '最近更新' },
  { key: 'popular', label: '人气最高' },
  { key: 'words', label: '字数最多' }
] as const;

function parseSort(value: string | null): ReaderBookSort | null {
  return SORT_OPTIONS.some((option) => option.key === value) ? value as ReaderBookSort : null;
}

export function BooksPage() {
  const initial = useSsrData().books;
  const [searchParams, setSearchParams] = useSearchParams();
  const [keywordInput, setKeywordInput] = useState(searchParams.get('keyword') || '');
  const [keyword, setKeyword] = useState(searchParams.get('keyword') || '');
  const initialSubCategoryCode = searchParams.get('subCategory') || '';
  const [categoryCode, setCategoryCode] = useState(initialSubCategoryCode ? '' : searchParams.get('category') || '');
  const [subCategoryCode, setSubCategoryCode] = useState(initialSubCategoryCode);
  const rawSort = searchParams.get('sort');
  const explicitSort = parseSort(rawSort);
  const hasFilters = Boolean(keyword.trim() || categoryCode.trim() || subCategoryCode.trim());
  const hasExplicitSort = Boolean(rawSort?.trim());
  const randomMode = !hasFilters && !hasExplicitSort;
  const effectiveSort: ReaderBookSort = explicitSort || 'recent';
  const initialMatchesMode = initial?.mode === (randomMode ? 'random' : 'paged');
  const [pageNum, setPageNum] = useState(1);
  const [randomRequest, setRandomRequest] = useState(0);
  const [books, setBooks] = useState<ReaderBookCatalogItem[]>(initialMatchesMode ? initial?.result.rows || [] : []);
  const [categories, setCategories] = useState<ReaderCategory[]>(initial?.categories || []);
  const [subCategories, setSubCategories] = useState<ReaderCategory[]>(initial?.subCategories || []);
  const [total, setTotal] = useState(initialMatchesMode && initial?.mode === 'paged' ? initial.result.total : 0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [sortOpen, setSortOpen] = useState(false);
  const listKey = useMemo(
    () => `${randomMode ? 'random' : 'paged'}\u0000${keyword}\u0000${categoryCode}\u0000${subCategoryCode}\u0000${effectiveSort}`,
    [categoryCode, effectiveSort, keyword, randomMode, subCategoryCode]
  );
  const hasMore = !randomMode && books.length < total;
  const selectedSortLabel = randomMode ? '随机推荐' : SORT_OPTIONS.find((option) => option.key === effectiveSort)?.label || '最近更新';
  const bookListRef = useRef<HTMLDivElement | null>(null);
  const loadMoreTriggerRef = useRef<HTMLDivElement | null>(null);
  const loadingRef = useRef(false);
  const hasMoreRef = useRef(hasMore);
  const listKeyRef = useRef(listKey);
  const loadedListKeyRef = useRef(initialMatchesMode ? listKey : '');
  const skipInitialRequestRef = useRef(Boolean(initialMatchesMode));

  useEffect(() => {
    if (initial) return;
    listCategories().then(setCategories).catch(() => setCategories([]));
    listSubCategories().then(setSubCategories).catch(() => setSubCategories([]));
  }, [initial]);

  useEffect(() => {
    listKeyRef.current = listKey;
  }, [listKey]);

  useEffect(() => {
    hasMoreRef.current = hasMore;
  }, [hasMore]);

  useEffect(() => {
    if (skipInitialRequestRef.current && pageNum === 1 && randomRequest === 0) {
      skipInitialRequestRef.current = false;
      return undefined;
    }
    let alive = true;
    loadingRef.current = true;
    setLoading(true);
    setError('');
    const request = randomMode
      ? listRandomBooks().then((rows) => ({ rows, total: 0 }))
      : queryBooks({ keyword, categoryCode, subCategoryCode, sort: effectiveSort, pageNum, pageSize: PAGE_SIZE });
    request
      .then((result) => {
        if (!alive) {
          return;
        }
        setBooks((current) => (randomMode || pageNum === 1 ? result.rows : [...current, ...result.rows]));
        setTotal(result.total);
        loadedListKeyRef.current = listKey;
      })
      .catch((nextError) => {
        if (alive) {
          setError(nextError instanceof Error ? nextError.message : '作品加载失败');
        }
      })
      .finally(() => {
        if (alive) {
          loadingRef.current = false;
          setLoading(false);
        }
      });
    return () => {
      alive = false;
    };
  }, [categoryCode, effectiveSort, keyword, listKey, pageNum, randomMode, randomRequest, subCategoryCode]);

  useEffect(() => {
    const root = bookListRef.current;
    const trigger = loadMoreTriggerRef.current;
    if (randomMode || !root || !trigger || typeof IntersectionObserver === 'undefined') {
      return undefined;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry?.isIntersecting) {
          return;
        }
        if (loadingRef.current || !hasMoreRef.current || loadedListKeyRef.current !== listKeyRef.current) {
          return;
        }
        loadingRef.current = true;
        setPageNum((current) => current + 1);
      },
      {
        root,
        rootMargin: '0px 0px 96px 0px'
      }
    );

    observer.observe(trigger);
    return () => observer.disconnect();
  }, [randomMode]);

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    const nextKeyword = keywordInput.trim();
    setKeyword(nextKeyword);
    setPageNum(1);
    setSearchParams((current) => {
      const params = new URLSearchParams(current);
      if (nextKeyword) {
        params.set('keyword', nextKeyword);
      } else {
        params.delete('keyword');
      }
      if (categoryCode) {
        params.set('category', categoryCode);
      } else {
        params.delete('category');
      }
      if (subCategoryCode) {
        params.set('subCategory', subCategoryCode);
      } else {
        params.delete('subCategory');
      }
      return params;
    });
  };

  const chooseCategory = (nextCategoryCode: string) => {
    setCategoryCode(nextCategoryCode);
    setSubCategoryCode('');
    setPageNum(1);
    setSearchParams((current) => {
      const params = new URLSearchParams(current);
      if (nextCategoryCode) {
        params.set('category', nextCategoryCode);
      } else {
        params.delete('category');
      }
      params.delete('subCategory');
      if (keyword) {
        params.set('keyword', keyword);
      }
      return params;
    });
  };

  const chooseSubCategory = (nextSubCategoryCode: string) => {
    setCategoryCode('');
    setSubCategoryCode(nextSubCategoryCode);
    setPageNum(1);
    setSearchParams((current) => {
      const params = new URLSearchParams(current);
      params.delete('category');
      if (nextSubCategoryCode) {
        params.set('subCategory', nextSubCategoryCode);
      } else {
        params.delete('subCategory');
      }
      if (keyword) {
        params.set('keyword', keyword);
      }
      return params;
    });
  };

  const chooseSort = (nextSortKey: ReaderBookSort | 'random') => {
    setPageNum(1);
    setSortOpen(false);
    setSearchParams((current) => {
      const params = new URLSearchParams(current);
      if (nextSortKey === 'random') {
        params.delete('sort');
      } else {
        params.set('sort', nextSortKey);
      }
      return params;
    });
  };

  return (
    <AppShell active="books" title="书库">
      <main className="reader-page reader-page--narrow books-page">
        <div className="books-controls">
          <form className="search-band" onSubmit={submitSearch}>
            <Input
              size="large"
              value={keywordInput}
              placeholder="搜索书名"
              onChange={(event) => setKeywordInput(event.target.value)}
              suffix={
                <button type="submit" className="search-band__go" aria-label="搜索">
                  <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <circle cx="11" cy="11" r="7" />
                    <path d="m20 20-3.2-3.2" />
                  </svg>
                </button>
              }
            />
          </form>
          <BookCategoryNav
            categories={categories}
            subCategories={subCategories}
            selectedCategoryCode={categoryCode}
            selectedSubCategoryCode={subCategoryCode}
            onSelectCategory={chooseCategory}
            onSelectSubCategory={chooseSubCategory}
          />
        </div>

        <div className="books-results">
          <div className="books-status">
            <div className="books-sort">
              <button
                className={`books-sort__toggle${sortOpen ? ' is-open' : ''}`}
                type="button"
                aria-label={`排序：${selectedSortLabel}`}
                aria-haspopup="menu"
                aria-expanded={sortOpen}
                onClick={() => setSortOpen((current) => !current)}
              >
                <svg className="books-sort__icon" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M12 7v5l3 2" />
                  <path d="M20.5 12a8.5 8.5 0 1 1-2.5-6" />
                  <path d="M20.5 4.5V9H16" />
                </svg>
                <span>{selectedSortLabel}</span>
                <svg className="books-sort__arrow" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="m6 9 6 6 6-6" />
                </svg>
              </button>
              {sortOpen ? (
                <div className="books-sort__menu" role="menu" aria-label="排序选项">
                  <button
                    className={randomMode ? 'books-sort__item is-active' : 'books-sort__item'}
                    type="button"
                    role="menuitemradio"
                    aria-checked={randomMode}
                    disabled={hasFilters}
                    onClick={() => chooseSort('random')}
                  >
                    随机推荐
                  </button>
                  {SORT_OPTIONS.map((option) => (
                    <button
                      className={!randomMode && option.key === effectiveSort ? 'books-sort__item is-active' : 'books-sort__item'}
                      key={option.key}
                      type="button"
                      role="menuitemradio"
                      aria-checked={!randomMode && option.key === effectiveSort}
                      onClick={() => chooseSort(option.key)}
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
            <div className="books-status__message" aria-live="polite">
              {!randomMode && !loading && total > 0 ? <p className="result-count">共 {total} 本</p> : null}
              {error ? <p className="notice notice--error">{error}</p> : null}
            </div>
            {randomMode ? (
              <button
                className="books-random-refresh"
                type="button"
                disabled={loading}
                onClick={() => setRandomRequest((current) => current + 1)}
              >
                <RefreshCw size={15} aria-hidden="true" />
                <span>换一批</span>
              </button>
            ) : null}
          </div>
          <div className="book-list" ref={bookListRef}>
            {!loading && books.length === 0 ? (
              <Card className="empty-card">没有找到匹配的作品</Card>
            ) : (
              books.map((book) => <BookTextCard key={book.bookId} book={book} to={`/books/${book.bookId}`} />)
            )}
            {!randomMode ? (
              <div className="book-list__load-trigger" ref={loadMoreTriggerRef} aria-live="polite">
                {hasMore && loading && books.length > 0 ? '加载中...' : null}
              </div>
            ) : null}
          </div>
        </div>
      </main>
    </AppShell>
  );
}
