import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BooksPage } from './BooksPage';

const readerApi = vi.hoisted(() => ({
  listCategories: vi.fn(),
  listRandomBooks: vi.fn(),
  listSubCategories: vi.fn(),
  queryBooks: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Button: ({ children, loading, onClick }: { children: React.ReactNode; loading?: boolean; onClick?: () => void }) => (
    <button disabled={loading} type="button" onClick={onClick}>
      {children}
    </button>
  ),
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <section className={className}>{children}</section>
  ),
  Input: ({ onChange, placeholder, suffix, value }: { onChange?: React.ChangeEventHandler<HTMLInputElement>; placeholder?: string; suffix?: React.ReactNode; value?: string }) => (
    <label>
      <input placeholder={placeholder} value={value} onChange={onChange} />
      {suffix}
    </label>
  )
}));

function renderBooksPage(initialEntry = '/books') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/books" element={<BooksPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('BooksPage categories', () => {
  const scrollIntoView = vi.fn();
  const observe = vi.fn();
  const disconnect = vi.fn();
  let intersectionCallback: IntersectionObserverCallback | undefined;
  const bookRows = [
    {
      bookId: '9007199254740993',
      bookName: '很长的测试作品标题',
      authorName: '作者甲',
      bookDesc: '这是第一本测试作品的简介',
      categoryCode: '13',
      categoryName: '科幻',
      bookStatus: '1',
      wordCount: 12345,
      likeCount: 6,
      lastChapterId: '9007199254740993001',
      lastChapterName: '第一章',
      lastChapterUpdateTime: '2026-06-27 10:00:00',
      featured: 0,
      featuredNote: ''
    },
    {
      bookId: '9007199254740994',
      bookName: '第二本测试作品',
      authorName: '作者乙',
      bookDesc: '这是第二本测试作品的简介',
      categoryCode: '11',
      categoryName: '正太',
      bookStatus: '1',
      wordCount: 67890,
      likeCount: 2,
      lastChapterId: '9007199254740994001',
      lastChapterName: '第一章',
      lastChapterUpdateTime: '2026-06-27 10:00:00',
      featured: 0,
      featuredNote: ''
    }
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    observe.mockClear();
    disconnect.mockClear();
    intersectionCallback = undefined;
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView
    });
    vi.stubGlobal(
      'IntersectionObserver',
      vi.fn(function MockIntersectionObserver(callback: IntersectionObserverCallback) {
        intersectionCallback = callback;
        return {
          observe,
          disconnect,
          unobserve: vi.fn(),
          takeRecords: vi.fn(),
          root: null,
          rootMargin: '',
          thresholds: []
        };
      })
    );
    readerApi.listCategories.mockResolvedValue([
      { categoryCode: '11', categoryName: '正太' },
      { categoryCode: '12', categoryName: '激情H文' },
      { categoryCode: '13', categoryName: '科幻' },
      { categoryCode: '14', categoryName: '恐怖灵异' }
    ]);
    readerApi.listSubCategories.mockResolvedValue([
      { categoryCode: 'system', categoryName: '系统' },
      { categoryCode: 'rebirth', categoryName: '重生' }
    ]);
    readerApi.queryBooks.mockResolvedValue({ rows: [], total: 0 });
    readerApi.listRandomBooks.mockResolvedValue([]);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('renders categories in the home-style horizontal strip and reveals category from the URL', async () => {
    const { container } = renderBooksPage('/books?category=13');

    const categoryButton = await screen.findByRole('button', { name: '科幻' });
    const categoryStrip = container.querySelector('.cat-strip');

    expect(categoryStrip).toContainElement(categoryButton);
    expect(categoryButton).toHaveClass('cat-chip', 'is-active');
    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalledWith({
        behavior: 'smooth',
        block: 'nearest',
        inline: 'center'
      });
    });
  });

  it('keeps book rows and the auto-load trigger inside the scrollable book list region', async () => {
    readerApi.queryBooks.mockResolvedValue({ rows: bookRows, total: 3 });

    const { container } = renderBooksPage('/books?sort=recent');

    await screen.findByText('很长的测试作品标题');
    const page = container.querySelector<HTMLElement>('.books-page');
    const results = container.querySelector<HTMLElement>('.books-results');
    const bookList = container.querySelector<HTMLElement>('.book-list');
    const autoLoadTrigger = container.querySelector<HTMLElement>('.book-list__load-trigger');

    expect(page).toContainElement(results);
    expect(results).toContainElement(bookList);
    expect(bookList).toContainElement(screen.getByText('很长的测试作品标题').closest('.book-row'));
    expect(bookList).toContainElement(autoLoadTrigger);
    expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument();
  });

  it('loads the next page automatically when the list bottom enters view', async () => {
    readerApi.queryBooks
      .mockResolvedValueOnce({ rows: bookRows, total: 3 })
      .mockResolvedValueOnce({ rows: [bookRows[0]], total: 3 });

    renderBooksPage('/books?sort=recent');

    await screen.findByText('很长的测试作品标题');

    expect(observe).toHaveBeenCalled();
    expect(intersectionCallback).toBeDefined();

    act(() => {
      intersectionCallback?.([
        {
          isIntersecting: true
        } as IntersectionObserverEntry
      ], {} as IntersectionObserver);
    });

    await waitFor(() => {
      expect(readerApi.queryBooks).toHaveBeenLastCalledWith({
        keyword: '',
        categoryCode: '',
        subCategoryCode: '',
        sort: 'recent',
        pageNum: 2,
        pageSize: 10
      });
    });
  });

  it('filters by the sub-category from the URL and shows its label in the tag button', async () => {
    renderBooksPage('/books?subCategory=system');

    const selectedButton = await screen.findByRole('button', { name: '系统', expanded: false });

    expect(selectedButton).toHaveClass('cat-chip', 'is-active');
    expect(screen.getByRole('button', { name: '全部' })).not.toHaveClass('is-active');
    await waitFor(() => {
      expect(readerApi.queryBooks).toHaveBeenLastCalledWith({
        keyword: '',
        categoryCode: '',
        subCategoryCode: 'system',
        sort: 'recent',
        pageNum: 1,
        pageSize: 10
      });
    });
  });

  it('switches sub-categories, closes the panel, and updates the query', async () => {
    renderBooksPage('/books?subCategory=system');

    fireEvent.click(await screen.findByRole('button', { name: '系统' }));
    const panel = screen.getByRole('navigation', { name: '副分类' });
    fireEvent.click(within(panel).getByRole('button', { name: '重生' }));

    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: '重生' })).toHaveClass('is-active');
    await waitFor(() => {
      expect(readerApi.queryBooks).toHaveBeenLastCalledWith({
        keyword: '',
        categoryCode: '',
        subCategoryCode: 'rebirth',
        sort: 'recent',
        pageNum: 1,
        pageSize: 10
      });
    });
  });

  it('clears a selected sub-category from the top-level all button', async () => {
    renderBooksPage('/books?subCategory=system&sort=recent');

    fireEvent.click(await screen.findByRole('button', { name: '全部' }));

    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
    await waitFor(() => {
      expect(readerApi.queryBooks).toHaveBeenLastCalledWith({
        keyword: '',
        categoryCode: '',
        subCategoryCode: '',
        sort: 'recent',
        pageNum: 1,
        pageSize: 10
      });
    });
  });

  it('switches from a sub-category to a main category', async () => {
    renderBooksPage('/books?subCategory=system&sort=recent');

    fireEvent.click(await screen.findByRole('button', { name: '科幻' }));

    await waitFor(() => {
      expect(readerApi.queryBooks).toHaveBeenLastCalledWith({
        keyword: '',
        categoryCode: '13',
        subCategoryCode: '',
        sort: 'recent',
        pageNum: 1,
        pageSize: 10
      });
    });
  });

  it('does not render book descriptions in the book list', async () => {
    readerApi.queryBooks.mockResolvedValue({ rows: bookRows, total: 2 });

    const { container } = renderBooksPage('/books?sort=recent');
    const currentPage = within(container);

    await currentPage.findByText('很长的测试作品标题');

    expect(container.querySelector('.book-list .book-row__description')).toBeNull();
    expect(currentPage.queryByText('这是第一本测试作品的简介')).toBeNull();
    expect(currentPage.queryByText('这是第二本测试作品的简介')).toBeNull();
  });

  it('shows a recent update sort control on the right side of the status row and opens other choices', async () => {
    readerApi.queryBooks.mockResolvedValue({ rows: bookRows, total: 2 });

    const { container } = renderBooksPage('/books?sort=recent');
    const currentPage = within(container);

    await screen.findByText('共 2 本');
    const statusRow = container.querySelector<HTMLElement>('.books-status');
    const sortButton = currentPage.getByRole('button', { name: '排序：最近更新' });

    expect(statusRow).toContainElement(sortButton);
    expect(sortButton).toHaveClass('books-sort__toggle');
    expect(sortButton.querySelector('.books-sort__icon')).not.toBeNull();
    expect(sortButton.querySelector('.books-sort__arrow')).not.toBeNull();

    fireEvent.click(sortButton);

    expect(screen.getByRole('menuitemradio', { name: '人气最高' })).toBeInTheDocument();
    expect(screen.getByRole('menuitemradio', { name: '字数最多' })).toBeInTheDocument();
  });

  it('reloads the first page using the selected server-side sort', async () => {
    readerApi.queryBooks.mockResolvedValue({ rows: bookRows, total: 2 });

    renderBooksPage('/books?sort=recent');
    fireEvent.click(await screen.findByRole('button', { name: '排序：最近更新' }));
    fireEvent.click(screen.getByRole('menuitemradio', { name: '人气最高' }));

    await waitFor(() => {
      expect(readerApi.queryBooks).toHaveBeenLastCalledWith({
        keyword: '',
        categoryCode: '',
        subCategoryCode: '',
        sort: 'popular',
        pageNum: 1,
        pageSize: 10
      });
    });
  });

  it('loads a random batch by default and replaces it when requested', async () => {
    readerApi.listRandomBooks
      .mockResolvedValueOnce(bookRows)
      .mockResolvedValueOnce([bookRows[1]]);

    const { container } = renderBooksPage();

    await screen.findByText('很长的测试作品标题');
    expect(readerApi.queryBooks).not.toHaveBeenCalled();
    expect(container.querySelector('.book-list__load-trigger')).toBeNull();
    expect(screen.queryByText(/共 \d+ 本/)).not.toBeInTheDocument();

    const statusControls = container.querySelector('.books-status');
    const sortControl = container.querySelector('.books-sort');
    const refreshButton = screen.getByRole('button', { name: '换一批' });
    expect(statusControls).not.toBeNull();
    expect(sortControl).not.toBeNull();
    expect(Array.from(statusControls!.children)).toEqual([
      sortControl,
      container.querySelector('.books-status__message'),
      refreshButton
    ]);

    fireEvent.click(refreshButton);

    await waitFor(() => expect(readerApi.listRandomBooks).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('第二本测试作品')).toBeInTheDocument();
    expect(screen.queryByText('很长的测试作品标题')).not.toBeInTheDocument();
  });
});
