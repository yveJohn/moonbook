import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { StrictMode } from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../api/http';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import type { ReaderLikedBook } from '../types/reader';
import { LikedBooksPage } from './LikedBooksPage';

const readerApi = vi.hoisted(() => ({
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  listLikedBooks: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  registerReader: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}{location.search}</div>;
}

function renderLikedBooksPage(strictMode = false) {
  const content = (
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={['/me/likes']}>
        <Routes>
          <Route path="/me/likes" element={<LikedBooksPage />} />
          <Route path="/me" element={<div>我的页面</div>} />
          <Route path="/books/:bookId" element={<LocationProbe />} />
          <Route path="/auth/login" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
  return render(strictMode ? <StrictMode>{content}</StrictMode> : content);
}

function loginReader(token = 'reader.jwt.token', readerId = '9223372036854775801') {
  window.localStorage.setItem(READER_TOKEN_KEY, token);
  window.localStorage.setItem(
    READER_PROFILE_KEY,
    JSON.stringify({
      readerId,
      username: 'reader',
      nickname: '读者',
      status: 'enabled'
    })
  );
}

async function switchReader(token: string, readerId: string) {
  const oldValue = window.localStorage.getItem(READER_TOKEN_KEY);
  loginReader(token, readerId);
  await act(async () => {
    window.dispatchEvent(new StorageEvent('storage', {
      key: READER_TOKEN_KEY,
      oldValue,
      newValue: token
    }));
  });
}

function likedBook(overrides: Partial<ReaderLikedBook> = {}): ReaderLikedBook {
  return {
    likeId: '9223372036854775810',
    bookId: '9223372036854775808',
    bookName: '月下长书',
    authorName: '青衫',
    bookDesc: '旧城月下，故人归来。',
    categoryCode: 'history',
    categoryName: '历史',
    wordCount: 128600,
    likeCount: 3650,
    likedAt: '2026-07-12 21:30:00',
    ...overrides
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

describe('LikedBooksPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.listLikedBooks.mockResolvedValue([]);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('redirects anonymous readers without requesting liked books', async () => {
    renderLikedBooksPage();

    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent('/auth/login?redirect=/me/likes');
    });
    expect(readerApi.listLikedBooks).not.toHaveBeenCalled();
  });

  it('shows row-shaped skeletons while loading', () => {
    loginReader();
    readerApi.listLikedBooks.mockReturnValue(new Promise(() => {}));

    renderLikedBooksPage();

    const status = screen.getByRole('status', { name: '赞过的图书加载中' });
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(within(status).getByText('赞过的图书加载中')).toHaveClass('liked-books-sr-only');
    const skeletons = screen.getAllByTestId('liked-book-skeleton');
    expect(skeletons).toHaveLength(3);
    expect(skeletons.every((skeleton) => skeleton.getAttribute('aria-hidden') === 'true')).toBe(true);
    expect(screen.queryByText('暂无赞过的图书')).not.toBeInTheDocument();
  });

  it('shows the empty state after an empty response', async () => {
    loginReader();

    renderLikedBooksPage();

    expect(await screen.findByText('暂无赞过的图书')).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('link', { name: '我的' })).toHaveClass('is-active');
    fireEvent.click(screen.getByRole('button', { name: '返回' }));
    expect(screen.getByText('我的页面')).toBeInTheDocument();
  });

  it('uses a safe message for generic HTTP failures and retries', async () => {
    loginReader();
    readerApi.listLikedBooks
      .mockRejectedValueOnce(new Error('Request failed with status code 500'))
      .mockResolvedValueOnce([likedBook()]);

    renderLikedBooksPage();

    expect(await screen.findByText('赞过的图书加载失败')).toBeInTheDocument();
    expect(screen.queryByText('Request failed with status code 500')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));

    expect(await screen.findByText('月下长书')).toBeInTheDocument();
    expect(readerApi.listLikedBooks).toHaveBeenCalledTimes(2);
  });

  it('uses the safe fallback for API errors without exposing backend details', async () => {
    loginReader();
    readerApi.listLikedBooks.mockRejectedValue(new ApiError(40021, '内部表 reader_book_like 查询失败'));

    renderLikedBooksPage();

    expect(await screen.findByText('赞过的图书加载失败')).toBeInTheDocument();
    expect(screen.queryByText('内部表 reader_book_like 查询失败')).not.toBeInTheDocument();
  });

  it('falls back for blank author and category names', async () => {
    loginReader();
    readerApi.listLikedBooks.mockResolvedValue([
      likedBook({ authorName: '', categoryName: '   ' }),
      likedBook({
        likeId: '9223372036854775811',
        bookId: '9223372036854775809',
        bookName: '空白作者作品',
        authorName: '\t',
        categoryName: ''
      })
    ]);

    renderLikedBooksPage();

    const rows = await screen.findAllByTestId('liked-book-row');
    expect(rows[0]).toHaveTextContent('佚名');
    expect(rows[0]).toHaveTextContent('未分类');
    expect(rows[1]).toHaveTextContent('佚名');
    expect(rows[1]).toHaveTextContent('未分类');
  });

  it('handles nullable book metadata safely', async () => {
    loginReader();
    readerApi.listLikedBooks.mockResolvedValue([
      likedBook({ authorName: null, bookDesc: null, categoryCode: null, categoryName: null })
    ]);

    renderLikedBooksPage();

    const row = await screen.findByTestId('liked-book-row');
    expect(row).toHaveTextContent('佚名');
    expect(row).toHaveTextContent('未分类');
    expect(row).not.toHaveTextContent('旧城月下，故人归来。');
  });

  it('renders all metadata in response order', async () => {
    loginReader();
    readerApi.listLikedBooks.mockResolvedValue([
      likedBook({ bookName: '先赞的书', bookDesc: '', categoryName: '科幻', wordCount: 42000, likeCount: 81 }),
      likedBook({
        likeId: '9223372036854775811',
        bookId: '9223372036854775809',
        bookName: '后赞的书',
        authorName: '白石',
        categoryName: '悬疑',
        likedAt: '2026-07-11 08:00:00'
      })
    ]);

    renderLikedBooksPage();

    const rows = await screen.findAllByTestId('liked-book-row');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('先赞的书');
    expect(rows[1]).toHaveTextContent('后赞的书');
    expect(rows[0]).toHaveTextContent('青衫');
    expect(rows[0]).toHaveTextContent('科幻');
    expect(rows[0]).toHaveTextContent('42,000 字');
    expect(rows[0]).toHaveTextContent('81');
    const likedDate = new Date('2026-07-12T21:30:00');
    expect(rows[0]).toHaveTextContent(likedDate.toLocaleString('zh-CN', { hour12: false }));
    expect(rows[0].querySelector('time')).toHaveAttribute('datetime', likedDate.toISOString());
    expect(rows[0]).not.toHaveTextContent('旧城月下，故人归来。');
    expect(rows[1]).toHaveTextContent('旧城月下，故人归来。');
  });

  it('preserves an unsafe integer book id exactly in navigation', async () => {
    loginReader();
    const bookId = '922337203685477580812345678901';
    readerApi.listLikedBooks.mockResolvedValue([likedBook({ bookId })]);

    renderLikedBooksPage();

    const link = await screen.findByRole('link', { name: /月下长书.*青衫.*历史/ });
    expect(link).toHaveAttribute('href', `/books/${bookId}`);
    link.focus();
    expect(link).toHaveFocus();
    fireEvent.click(link);
    expect(screen.getByTestId('location')).toHaveTextContent(`/books/${bookId}`);
  });

  it('ignores state updates after unmount', async () => {
    loginReader();
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    const request = deferred<ReaderLikedBook[]>();
    readerApi.listLikedBooks.mockReturnValue(request.promise);
    const view = renderLikedBooksPage();

    view.unmount();
    await act(async () => {
      request.resolve([likedBook()]);
      await request.promise;
    });

    expect(screen.queryByText('月下长书')).not.toBeInTheDocument();
    expect(consoleError).not.toHaveBeenCalled();
  });

  it('loads the retry result after a failed request', async () => {
    loginReader();
    const retryRequest = deferred<ReaderLikedBook[]>();
    readerApi.listLikedBooks
      .mockRejectedValueOnce(new Error('network failed'))
      .mockReturnValueOnce(retryRequest.promise);

    renderLikedBooksPage();

    fireEvent.click(await screen.findByRole('button', { name: '重新加载' }));
    expect(screen.getAllByTestId('liked-book-skeleton')).toHaveLength(3);

    await act(async () => {
      retryRequest.resolve([likedBook({ bookName: '重试后的书' })]);
      await retryRequest.promise;
    });

    expect(await screen.findByText('重试后的书')).toBeInTheDocument();
    expect(screen.queryByText('赞过的图书加载失败')).not.toBeInTheDocument();
  });

  it('clears account data and loads again when the token changes', async () => {
    loginReader('token-a', '9223372036854775801');
    const accountBRequest = deferred<ReaderLikedBook[]>();
    readerApi.listLikedBooks
      .mockResolvedValueOnce([likedBook({ bookName: '账户A的书' })])
      .mockReturnValueOnce(accountBRequest.promise);

    renderLikedBooksPage();
    expect(await screen.findByText('账户A的书')).toBeInTheDocument();

    await switchReader('token-b', '9223372036854775802');
    expect(screen.queryByText('账户A的书')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('liked-book-skeleton')).toHaveLength(3);
    expect(readerApi.listLikedBooks).toHaveBeenCalledTimes(2);

    await act(async () => {
      accountBRequest.resolve([likedBook({ bookName: '账户B的书' })]);
      await accountBRequest.promise;
    });
    expect(await screen.findByText('账户B的书')).toBeInTheDocument();
  });

  it('never publishes an account A response after switching to account B', async () => {
    loginReader('token-a', '9223372036854775801');
    const accountARequest = deferred<ReaderLikedBook[]>();
    const accountBRequest = deferred<ReaderLikedBook[]>();
    readerApi.listLikedBooks
      .mockReturnValueOnce(accountARequest.promise)
      .mockReturnValueOnce(accountBRequest.promise);

    renderLikedBooksPage();
    await waitFor(() => expect(readerApi.listLikedBooks).toHaveBeenCalledTimes(1));
    await switchReader('token-b', '9223372036854775802');
    await waitFor(() => expect(readerApi.listLikedBooks).toHaveBeenCalledTimes(2));

    await act(async () => {
      accountBRequest.resolve([likedBook({ bookName: '账户B的新书' })]);
      await accountBRequest.promise;
    });
    expect(await screen.findByText('账户B的新书')).toBeInTheDocument();

    await act(async () => {
      accountARequest.resolve([likedBook({ bookName: '账户A的旧书' })]);
      await accountARequest.promise;
    });
    expect(screen.getByText('账户B的新书')).toBeInTheDocument();
    expect(screen.queryByText('账户A的旧书')).not.toBeInTheDocument();
  });

  it('uses semantic time with a safe fallback for invalid values', async () => {
    loginReader();
    readerApi.listLikedBooks.mockResolvedValue([
      likedBook({ likedAt: 'not-a-time' }),
      likedBook({
        likeId: '9223372036854775811',
        bookId: '9223372036854775809',
        bookName: '没有点赞时间的书',
        likedAt: '   '
      })
    ]);

    renderLikedBooksPage();

    const times = await screen.findAllByText('赞于 时间待确认');
    expect(times).toHaveLength(2);
    expect(times.every((time) => time.tagName === 'TIME')).toBe(true);
    expect(times.every((time) => !time.hasAttribute('datetime'))).toBe(true);
    expect(screen.queryByText('not-a-time')).not.toBeInTheDocument();
  });

  it('keeps the newer response when Strict Mode requests settle out of order', async () => {
    loginReader();
    const olderRequest = deferred<ReaderLikedBook[]>();
    const newerRequest = deferred<ReaderLikedBook[]>();
    readerApi.listLikedBooks
      .mockReturnValueOnce(olderRequest.promise)
      .mockReturnValueOnce(newerRequest.promise);

    renderLikedBooksPage(true);
    await waitFor(() => expect(readerApi.listLikedBooks).toHaveBeenCalledTimes(2));

    await act(async () => {
      newerRequest.resolve([likedBook({ bookName: '较新的结果' })]);
      await newerRequest.promise;
    });
    expect(await screen.findByText('较新的结果')).toBeInTheDocument();

    await act(async () => {
      olderRequest.resolve([likedBook({ bookName: '较旧的结果' })]);
      await olderRequest.promise;
    });
    expect(screen.getByText('较新的结果')).toBeInTheDocument();
    expect(screen.queryByText('较旧的结果')).not.toBeInTheDocument();
  });
});
