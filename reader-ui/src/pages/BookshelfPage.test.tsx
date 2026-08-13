import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import { BookshelfPage } from './BookshelfPage';

const readerApi = vi.hoisted(() => ({
  checkin: vi.fn(),
  getCheckinStatus: vi.fn(),
  getPreference: vi.fn(),
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  listBookshelf: vi.fn(),
  listHistory: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  registerReader: vi.fn(),
  removeBookshelf: vi.fn(),
  savePreference: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <section className={className}>{children}</section>
  ),
  Loading: () => <div>加载中</div>
}));

function LocationProbe() {
  const location = useLocation();
  return (
    <>
      <div data-testid="location">{location.pathname}{location.search}</div>
      <div data-testid="location-state">{JSON.stringify(location.state || null)}</div>
    </>
  );
}

function renderBookshelfPage(initialEntry = '/shelf') {
  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/shelf" element={<BookshelfPage />} />
          <Route path="/books" element={<div>书库页面</div>} />
          <Route path="/books/:bookId" element={<LocationProbe />} />
          <Route path="/read/:bookId" element={<LocationProbe />} />
          <Route path="/auth/login" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

function loginReader() {
  window.localStorage.setItem(READER_TOKEN_KEY, 'reader.jwt.token');
  window.localStorage.setItem(
    READER_PROFILE_KEY,
    JSON.stringify({
      readerId: '9223372036854775801',
      username: 'reader',
      nickname: '读者',
      status: 'enabled'
    })
  );
}

describe('BookshelfPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.listBookshelf.mockResolvedValue([]);
    readerApi.listHistory.mockResolvedValue([]);
    readerApi.removeBookshelf.mockResolvedValue(true);
  });

  afterEach(() => {
    cleanup();
  });

  it('redirects anonymous readers to login with shelf redirect', async () => {
    renderBookshelfPage();

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/auth/login?redirect=/shelf'));
    expect(readerApi.listBookshelf).not.toHaveBeenCalled();
    expect(readerApi.listHistory).not.toHaveBeenCalled();
  });

  it('loads shelf and history for authenticated readers and shows the empty state', async () => {
    loginReader();

    renderBookshelfPage();

    expect(screen.getByText('加载中')).toBeInTheDocument();
    expect(await screen.findByText('书架还是空的')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '去书库逛逛' })).toHaveAttribute('href', '/books');
    expect(readerApi.listBookshelf).toHaveBeenCalledTimes(1);
    expect(readerApi.listHistory).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('link', { name: '书架' })).toHaveClass('is-active');
  });

  it('keeps an SSR-restored reader on the shelf without visiting login', async () => {
    loginReader();

    render(
      <ReaderAuthProvider deferClientSession>
        <MemoryRouter initialEntries={['/shelf']}>
          <Routes>
            <Route path="/shelf" element={<BookshelfPage />} />
            <Route path="/auth/login" element={<div>登录页</div>} />
          </Routes>
          <LocationProbe />
        </MemoryRouter>
      </ReaderAuthProvider>
    );

    await waitFor(() => expect(readerApi.listBookshelf).toHaveBeenCalledTimes(1));
    expect(readerApi.listHistory).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('location')).toHaveTextContent('/shelf');
    expect(screen.queryByText('登录页')).not.toBeInTheDocument();
  });

  it('shows a safe fallback message when shelf loading fails', async () => {
    loginReader();
    readerApi.listBookshelf.mockRejectedValue(new Error('Request failed with status code 500'));

    renderBookshelfPage();

    expect(await screen.findByText('书架加载失败')).toBeInTheDocument();
    expect(screen.queryByText('Request failed with status code 500')).not.toBeInTheDocument();
    expect(screen.queryByText('书架还是空的')).not.toBeInTheDocument();
  });

  it('renders shelf rows sorted by last read time and opens book detail from the row', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '旧书',
        authorName: '作者乙',
        lastChapterId: '9223372036854775816',
        lastChapterName: '第十章',
        lastReadTime: '2026-07-07 10:00:00'
      },
      {
        bookshelfId: '9223372036854775803',
        bookId: '9223372036854775807',
        bookName: '新书',
        authorName: '作者甲',
        lastChapterId: '9223372036854775817',
        lastChapterName: '第二章',
        lastReadTime: '2026-07-08 09:00:00'
      }
    ]);

    renderBookshelfPage();

    await screen.findByText('共 2 本');
    const rows = screen.getAllByTestId('bookshelf-row');
    expect(rows[0]).toHaveTextContent('新书');
    expect(rows[1]).toHaveTextContent('旧书');

    fireEvent.click(screen.getByRole('link', { name: '新书 第二章 2026-07-08 09:00:00' }));

    expect(screen.getByTestId('location')).toHaveTextContent('/books/9223372036854775807');
  });

  it('uses reading history for continue links and progress display', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '第一序列',
        authorName: '作者甲',
        lastChapterId: '9223372036854775816',
        lastChapterName: '书架章节',
        lastReadTime: '2026-07-08 09:00:00'
      }
    ]);
    readerApi.listHistory.mockResolvedValue([
      {
        historyId: '9223372036854775900',
        bookId: '9223372036854775806',
        chapterId: '9223372036854775999',
        chapterNo: 128,
        chapterName: '历史章节',
        positionType: 'scroll',
        positionValue: 1200,
        progressPercent: '62.4',
        lastReadTime: '2026-07-08 09:30:00'
      }
    ]);

    renderBookshelfPage();

    expect(await screen.findByText('历史章节')).toBeInTheDocument();
    expect(screen.getByText('已读 62%')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('link', { name: '继续阅读 第一序列' }));

    expect(screen.getByTestId('location')).toHaveTextContent('/read/9223372036854775806');
    expect(screen.getByTestId('location-state')).toHaveTextContent('{"chapterId":"9223372036854775999"}');
  });

  it('renders zero progress from reading history', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '第一序列',
        authorName: '作者甲',
        lastChapterId: '9223372036854775816',
        lastChapterName: '第一章',
        lastReadTime: '2026-07-08 09:00:00'
      }
    ]);
    readerApi.listHistory.mockResolvedValue([
      {
        historyId: '9223372036854775900',
        bookId: '9223372036854775806',
        chapterId: '9223372036854775999',
        chapterNo: 1,
        chapterName: '第一章',
        positionType: 'scroll',
        positionValue: 0,
        progressPercent: '0.00',
        lastReadTime: '2026-07-08 09:30:00'
      }
    ]);

    renderBookshelfPage();

    expect(await screen.findByText('已读 0%')).toBeInTheDocument();
  });

  it('enters edit mode, selects all rows, and exits with complete', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '第一序列',
        authorName: '作者甲',
        lastChapterId: '9223372036854775816',
        lastChapterName: '第一章',
        lastReadTime: '2026-07-08 09:00:00'
      },
      {
        bookshelfId: '9223372036854775803',
        bookId: '9223372036854775807',
        bookName: '第二本',
        authorName: '作者乙',
        lastChapterId: '9223372036854775817',
        lastChapterName: '第二章',
        lastReadTime: '2026-07-07 09:00:00'
      }
    ]);

    renderBookshelfPage();

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    expect(screen.getByRole('heading', { name: '已选择 0 本' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '全选' }));
    expect(screen.getByRole('heading', { name: '已选择 2 本' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: '继续阅读 第一序列' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '完成' }));
    expect(screen.getByRole('heading', { name: '书架' })).toBeInTheDocument();
  });

  it('removes selected books with string book ids', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '第一序列',
        authorName: '作者甲',
        lastChapterId: '9223372036854775816',
        lastChapterName: '第一章',
        lastReadTime: '2026-07-08 09:00:00'
      }
    ]);

    renderBookshelfPage();

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    fireEvent.click(screen.getByRole('button', { name: '选择 第一序列' }));
    fireEvent.click(screen.getByRole('button', { name: '移出书架' }));

    await waitFor(() => expect(readerApi.removeBookshelf).toHaveBeenCalledWith('9223372036854775806'));
    expect(await screen.findByText('书架还是空的')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '书架' })).toBeInTheDocument();
  });

  it('keeps edit state when removing returns false', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '第一序列',
        authorName: '作者甲',
        lastChapterId: '9223372036854775816',
        lastChapterName: '第一章',
        lastReadTime: '2026-07-08 09:00:00'
      }
    ]);
    readerApi.removeBookshelf.mockResolvedValue(false);

    renderBookshelfPage();

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    fireEvent.click(screen.getByRole('button', { name: '选择 第一序列' }));
    fireEvent.click(screen.getByRole('button', { name: '移出书架' }));

    expect(await screen.findByText('移出书架失败')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '已选择 1 本' })).toBeInTheDocument();
    expect(screen.getByTestId('bookshelf-row')).toHaveTextContent('第一序列');
    expect(screen.queryByText('书架还是空的')).not.toBeInTheDocument();
  });

  it('freezes edit controls while removing selected books', async () => {
    loginReader();
    readerApi.listBookshelf.mockResolvedValue([
      {
        bookshelfId: '9223372036854775802',
        bookId: '9223372036854775806',
        bookName: '第一序列',
        authorName: '作者甲',
        lastChapterId: '9223372036854775816',
        lastChapterName: '第一章',
        lastReadTime: '2026-07-08 09:00:00'
      }
    ]);
    let resolveRemove!: (value: boolean) => void;
    readerApi.removeBookshelf.mockReturnValue(new Promise<boolean>((resolve) => {
      resolveRemove = resolve;
    }));

    renderBookshelfPage();

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    fireEvent.click(screen.getByRole('button', { name: '选择 第一序列' }));
    fireEvent.click(screen.getByRole('button', { name: '移出书架' }));

    expect(await screen.findByRole('button', { name: '移出中' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '完成' }));
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    fireEvent.click(screen.getByRole('button', { name: '取消全选' }));
    fireEvent.click(screen.getByRole('button', { name: '选择 第一序列' }));

    expect(screen.getByRole('heading', { name: '已选择 1 本' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '选择 第一序列' })).toHaveAttribute('aria-pressed', 'true');

    resolveRemove(true);

    expect(await screen.findByText('书架还是空的')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '书架' })).toBeInTheDocument();
  });
});
