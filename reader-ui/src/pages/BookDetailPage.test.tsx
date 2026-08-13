import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider, useReaderAuth } from '../auth/ReaderAuthContext';
import { READER_SESSION_CHANGED_EVENT } from '../auth/session';
import { ApiError, QUOTE_CHANGED_CODE } from '../api/http';
import { SsrDataProvider } from '../seo/SsrDataContext';
import type { ReaderBookDetail, ReaderChapterSummary } from '../types/reader';
import { BookDetailPage } from './BookDetailPage';

const readerApi = vi.hoisted(() => ({
  addBookshelf: vi.fn(),
  buyBook: vi.fn(),
  getBook: vi.fn(),
  getBookHistory: vi.fn(),
  getWallet: vi.fn(),
  likeBook: vi.fn(),
  listBookChapters: vi.fn(),
  unlikeBook: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Loading: () => <div data-testid="loading">加载中</div>
}));

function LocationProbe() {
  const location = useLocation();
  return <pre data-testid="location-state">{JSON.stringify(location.state)}</pre>;
}

function WalletProbe() {
  const { wallet } = useReaderAuth();
  return <output data-testid="wallet-balance">{wallet?.rechargeCoinBalance ?? ''}</output>;
}

function BookSwitcher() {
  const navigate = useNavigate();
  return <button type="button" onClick={() => navigate('/books/9223372036854775809')}>切换作品</button>;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => { resolve = nextResolve; });
  return { promise, resolve };
}

function renderBookDetailPage(
  initialEntries: Array<string | { pathname: string; state?: Record<string, unknown> }> = ['/books/9223372036854775806'],
  initialIndex = initialEntries.length - 1
) {
  return render(
    <ReaderAuthProvider>
      <WalletProbe />
      <MemoryRouter initialEntries={initialEntries} initialIndex={initialIndex}>
        <BookSwitcher />
        <Routes>
          <Route path="/" element={<div>进入详情之前的页面</div>} />
          <Route path="/read/:bookId" element={<LocationProbe />} />
          <Route path="/books/:bookId" element={<BookDetailPage />} />
          <Route path="/me" element={<div>会员中心</div>} />
          <Route path="/auth/login" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

function renderHydratedBookDetailPage(book: ReaderBookDetail, chapters: ReaderChapterSummary[]) {
  return render(
    <ReaderAuthProvider deferClientSession>
      <SsrDataProvider data={{ bookDetail: { book, chapters } }}>
        <MemoryRouter initialEntries={['/books/9223372036854775806']}>
          <Routes>
            <Route path="/books/:bookId" element={<BookDetailPage />} />
            <Route path="/auth/login" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </SsrDataProvider>
    </ReaderAuthProvider>
  );
}

function mockBookWithTwoChapters() {
  const productStatus = {
    bookId: '9223372036854775806',
    chargeMode: 'login_free' as const,
    readable: false,
    accessReason: 'login_required' as const,
    entitled: false,
    purchased: false,
    membershipEntitled: false,
    purchasable: false,
    productId: null,
    productName: null,
    priceCoin: null,
    saleStatus: null,
    product: null
  };
  const chapterAccess = (chapterId: string) => ({
    bookId: '9223372036854775806',
    chapterId,
    chargeMode: 'login_free' as const,
    readable: false,
    accessReason: 'login_required' as const,
    membershipEntitled: false,
    bookPurchased: false,
    chapterPurchased: false,
    purchasable: false,
    chapterWordCount: null,
    pricingWordUnit: null,
    pricingCoinUnit: null,
    chapterPrice: null
  });
  const book = {
    bookId: '9223372036854775806',
    bookName: '月下长书',
    authorName: '青衫',
    bookDesc: '一座旧城，一段旧事。',
    categoryCode: 'history',
    categoryName: '历史',
    bookStatus: '1',
    wordCount: 3600,
    likeCount: 0,
    commentCount: 0,
    favoriteCount: 0,
    subCategories: [
      { categoryCode: 'suspense', categoryName: '悬疑', sort: 0 },
      { categoryCode: 'ensemble', categoryName: '群像', sort: 1 }
    ],
    lastChapterId: '9223372036854775808',
    lastChapterName: '第二章 月色',
    lastChapterUpdateTime: '2026-06-28 12:00:00',
    featured: 0,
    featuredNote: '',
    visitCount: 0,
    liked: false,
    inBookshelf: false,
    readingHistory: null,
    productStatus
  };
  readerApi.getBook.mockResolvedValue(book);
  readerApi.listBookChapters.mockResolvedValue([
    {
      chapterId: '9223372036854775807',
      bookId: '9223372036854775806',
      chapterNo: 1,
      chapterName: '第一章 旧城',
      wordCount: 1800,
      updateTime: '2026-06-28 10:00:00',
      accessStatus: chapterAccess('9223372036854775807'),
      productStatus
    },
    {
      chapterId: '9223372036854775808',
      bookId: '9223372036854775806',
      chapterNo: 2,
      chapterName: '第二章 月色',
      wordCount: 1800,
      updateTime: '2026-06-28 12:00:00',
      accessStatus: chapterAccess('9223372036854775808'),
      productStatus
    }
  ]);
  return book;
}

function mockAuthenticatedReader() {
  window.localStorage.setItem('readerToken', 'test-token');
  window.localStorage.setItem(
    'readerProfile',
    JSON.stringify({ readerId: '1', username: 'reader', nickname: '读者', status: 'enabled' })
  );
}

describe('BookDetailPage chapters', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.getBookHistory.mockResolvedValue(null);
    readerApi.getWallet.mockResolvedValue({});
    mockBookWithTwoChapters();
  });

  afterEach(() => {
    cleanup();
  });

  it('shows compact numbered chapter rows', async () => {
    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    const firstChapter = screen.getByRole('link', { name: '第1章：第一章 旧城，免费' });
    const secondChapter = screen.getByRole('link', { name: '第2章：第二章 月色，免费' });
    expect(firstChapter).toHaveClass('chapter-row');
    expect(secondChapter).toHaveClass('chapter-row');
    expect(firstChapter).toHaveAttribute('href', '/read/9223372036854775806');
    expect(secondChapter).toHaveAttribute('href', '/read/9223372036854775806');
    expect(screen.queryByText('1800 字')).not.toBeInTheDocument();
  });

  it('shows chapters by default and hides the comments tab', async () => {
    renderBookDetailPage();

    expect(await screen.findByRole('tab', { name: '章节' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('tab', { name: '评论' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: '第1章：第一章 旧城，免费' })).toBeInTheDocument();
  });

  it('refreshes anonymous SSR chapter access after restoring an authenticated session', async () => {
    mockAuthenticatedReader();
    const initialBook = mockBookWithTwoChapters();
    const initialChapters = await readerApi.listBookChapters();
    const anonymousBook = {
      ...initialBook,
      productStatus: {
        ...initialBook.productStatus,
        chargeMode: 'word_charge' as const,
        accessReason: 'login_required' as const
      }
    };
    const anonymousChapters = initialChapters.map((chapter) => ({
      ...chapter,
      accessStatus: {
        ...chapter.accessStatus,
        chargeMode: 'word_charge' as const,
        accessReason: 'login_required' as const,
        readable: false,
        chapterPrice: 3
      }
    }));
    const authenticatedBook = {
      ...anonymousBook,
      productStatus: {
        ...anonymousBook.productStatus,
        accessReason: 'chapter_purchase_required' as const
      }
    };
    const authenticatedChapters = anonymousChapters.map((chapter) => ({
      ...chapter,
      accessStatus: {
        ...chapter.accessStatus,
        accessReason: 'chapter_purchase_required' as const,
        purchasable: true
      }
    }));
    const bookRequest = deferred<ReaderBookDetail>();
    const chaptersRequest = deferred<ReaderChapterSummary[]>();
    readerApi.getBook.mockReturnValueOnce(bookRequest.promise);
    readerApi.listBookChapters.mockReturnValueOnce(chaptersRequest.promise);

    renderHydratedBookDetailPage(anonymousBook, anonymousChapters);

    expect(screen.getAllByText('暂不可读')).toHaveLength(2);
    expect(screen.queryByTestId('loading')).not.toBeInTheDocument();
    await waitFor(() => {
      expect(readerApi.getBook).toHaveBeenCalledWith('9223372036854775806');
      expect(readerApi.listBookChapters).toHaveBeenCalledWith('9223372036854775806');
    });

    await act(async () => {
      bookRequest.resolve(authenticatedBook);
      chaptersRequest.resolve(authenticatedChapters);
      await Promise.all([bookRequest.promise, chaptersRequest.promise]);
    });

    expect(await screen.findAllByText('3 币')).toHaveLength(2);
    expect(screen.queryByText('暂不可读')).not.toBeInTheDocument();
  });

  it('refreshes detail permissions and history when the reader token changes', async () => {
    mockAuthenticatedReader();
    renderBookDetailPage();

    await waitFor(() => {
      expect(readerApi.getBook).toHaveBeenCalledTimes(1);
      expect(readerApi.listBookChapters).toHaveBeenCalledTimes(1);
      expect(readerApi.getBookHistory).toHaveBeenCalledTimes(1);
    });

    window.localStorage.setItem('readerToken', 'reader-token-b');
    window.localStorage.setItem(
      'readerProfile',
      JSON.stringify({ readerId: '2', username: 'reader-b', nickname: '读者B', status: 'enabled' })
    );
    act(() => window.dispatchEvent(new Event(READER_SESSION_CHANGED_EVENT)));

    await waitFor(() => {
      expect(readerApi.getBook).toHaveBeenCalledTimes(2);
      expect(readerApi.listBookChapters).toHaveBeenCalledTimes(2);
      expect(readerApi.getBookHistory).toHaveBeenCalledTimes(2);
    });
  });

  it('leaves the route-owned SEO title unchanged', async () => {
    document.title = '月白书城';
    const view = renderBookDetailPage();

    await screen.findAllByRole('heading', { name: '月下长书' });
    expect(document.title).toBe('月白书城');
    view.unmount();
    expect(document.title).toBe('月白书城');
  });

  it('shows sub-category chips in the detail header', async () => {
    renderBookDetailPage();

    expect(await screen.findByText('悬疑')).toBeInTheDocument();
    expect(screen.getByText('群像')).toBeInTheDocument();
  });

  it('places the primary category after the title and keeps sub-categories on their own row', async () => {
    renderBookDetailPage();

    await screen.findByText('历史');
    const title = screen
      .getAllByRole('heading', { name: '月下长书' })
      .find((heading) => heading.classList.contains('detail-head__title'));
    const primaryCategory = screen.getByText('历史');
    const subCategoryGroup = screen.getByLabelText('副分类');

    const titleRow = title?.closest('.detail-head__title-row') ?? null;
    expect(titleRow).toBeInTheDocument();
    expect(titleRow).toContainElement(primaryCategory);
    expect(titleRow).not.toContainElement(subCategoryGroup);
    expect(subCategoryGroup.closest('.detail-head__meta')).toBeInTheDocument();
  });

  it('shows membership readable status in the detail header', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'word_charge',
        readable: true,
        accessReason: 'membership',
        entitled: true,
        purchased: false,
        membershipEntitled: true,
        purchasable: true,
        productId: '4001',
        productName: '测试作品',
        priceCoin: 100,
        saleStatus: 'on_sale',
        product: null
      }
    });

    renderBookDetailPage();

    expect(await screen.findByText('会员可读')).toBeInTheDocument();
  });

  it('formats a serialized long book price without losing precision', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'fixed_price',
        accessReason: 'book_purchase_required',
        entitled: false,
        purchased: false,
        membershipEntitled: false,
        purchasable: true,
        productId: '9223372036854775807',
        productName: '测试作品',
        priceCoin: '9223372036854775807',
        saleStatus: 'on_sale',
        product: null
      }
    });

    renderBookDetailPage();

    expect(await screen.findByText('9,223,372,036,854,775,807金币')).toBeInTheDocument();
  });

  it('keeps the chapter list height bounded and top-aligned', async () => {
    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    const firstChapter = await screen.findByRole('link', { name: '第1章：第一章 旧城，免费' });
    const chapterList = firstChapter.closest<HTMLElement>('.detail-chapter-list');
    expect(chapterList).toHaveStyle({
      alignSelf: 'start',
      alignContent: 'start',
      maxHeight: 'clamp(152px, 42dvh, 360px)'
    });
  });

  it('skips the reader page when going back from a detail page opened by reader chrome', async () => {
    renderBookDetailPage([
      '/',
      '/read/9223372036854775806',
      { pathname: '/books/9223372036854775806', state: { readerReturnToDetail: true } }
    ]);

    fireEvent.click(await screen.findByRole('button', { name: '返回' }));

    expect(await screen.findByText('进入详情之前的页面')).toBeInTheDocument();
  });

  it('moves like, shelf and reading actions to the bottom bar with 1:3:3 ratio before adding shelf', async () => {
    renderBookDetailPage();

    await screen.findByRole('button', { name: '点赞' });
    const actionbar = document.querySelector<HTMLElement>('.detail-actionbar');
    expect(actionbar).toHaveStyle({ gridTemplateColumns: '1fr 3fr 3fr' });
    expect(screen.getByRole('button', { name: '点赞' })).toHaveTextContent('0');
    expect(screen.getByRole('button', { name: '加入书架' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '开始阅读' })).toHaveAttribute('href', '/read/9223372036854775806');
    expect(document.querySelector('.detail-head__actions')).not.toBeInTheDocument();
  });

  it('switches the shelf action to an icon and 1:1:5 ratio after joining the shelf', async () => {
    mockAuthenticatedReader();
    readerApi.addBookshelf.mockResolvedValue({
      bookshelfId: '1',
      bookId: '9223372036854775806',
      bookName: '月下长书',
      authorName: '青衫',
      lastChapterId: '',
      lastChapterName: '',
      lastReadTime: ''
    });
    renderBookDetailPage();

    fireEvent.click(await screen.findByRole('button', { name: '加入书架' }));

    expect(readerApi.addBookshelf).toHaveBeenCalledWith('9223372036854775806');
    expect(await screen.findByRole('button', { name: '已在书架' })).toBeInTheDocument();
    expect(document.querySelector<HTMLElement>('.detail-actionbar')).toHaveStyle({ gridTemplateColumns: '1fr 1fr 5fr' });
  });

  it('likes the book through the backend API and updates the displayed count', async () => {
    mockAuthenticatedReader();
    readerApi.likeBook.mockResolvedValue({
      likeId: '9001',
      bookId: '9223372036854775806',
      liked: true,
      likeCount: 1
    });
    renderBookDetailPage();

    fireEvent.click(await screen.findByRole('button', { name: '点赞' }));

    expect(readerApi.likeBook).toHaveBeenCalledWith('9223372036854775806');
    expect(await screen.findByRole('button', { name: '取消点赞' })).toHaveTextContent('1');
  });

  it('shows a three-line description preview and opens a full description card', async () => {
    const book = mockBookWithTwoChapters();
    const longDesc = '旧城的钟声在夜里回荡，少年循着失落书页走进尘封多年的藏书楼。'.repeat(6);
    readerApi.getBook.mockResolvedValueOnce({ ...book, bookDesc: longDesc });

    renderBookDetailPage();

    expect(await screen.findByText(longDesc)).toHaveClass('detail-description__text');
    fireEvent.click(screen.getByRole('button', { name: '查看全部' }));

    const dialog = screen.getByRole('dialog', { name: '作品简介' });
    expect(dialog).toHaveTextContent(longDesc);
    fireEvent.click(screen.getByRole('button', { name: '关闭简介' }));
    expect(screen.queryByRole('dialog', { name: '作品简介' })).not.toBeInTheDocument();
  });

  it('hides the description block when the book description is blank', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({ ...book, bookDesc: '   ' });

    const { container } = renderBookDetailPage();

    await screen.findByRole('button', { name: '点赞' });

    expect(container.querySelector('.detail-description')).not.toBeInTheDocument();
    expect(screen.queryByText('暂无简介')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '查看全部' })).not.toBeInTheDocument();
  });

  it.each([
    ['word_charge', '按字收费'],
    ['membership_only', '仅限会员'],
    ['login_free', '登录免费'],
    ['fixed_price', '整本购买']
  ] as const)('shows the %s charge mode label', async (chargeMode, label) => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: { ...book.productStatus, chargeMode }
    });

    renderBookDetailPage();

    expect(await screen.findByText(label)).toBeInTheDocument();
  });

  it('shows free, priced, purchased and membership-readable chapter states', async () => {
    const book = mockBookWithTwoChapters();
    const baseChapter = (await readerApi.listBookChapters())?.[0];
    readerApi.listBookChapters.mockResolvedValueOnce([
      {
        ...baseChapter,
        chapterId: '7001',
        chapterNo: 1,
        accessStatus: { ...baseChapter.accessStatus, chapterId: '7001', readable: true, accessReason: 'free_chapter', chapterPrice: 0 }
      },
      {
        ...baseChapter,
        chapterId: '7002',
        chapterNo: 2,
        accessStatus: { ...baseChapter.accessStatus, chapterId: '7002', chargeMode: 'word_charge', accessReason: 'chapter_purchase_required', purchasable: true, chapterPrice: '9223372036854775807' }
      },
      {
        ...baseChapter,
        chapterId: '7003',
        chapterNo: 3,
        accessStatus: { ...baseChapter.accessStatus, chapterId: '7003', chargeMode: 'word_charge', readable: true, accessReason: 'chapter_owned', chapterPurchased: true }
      },
      {
        ...baseChapter,
        chapterId: '7004',
        chapterNo: 4,
        accessStatus: { ...baseChapter.accessStatus, chapterId: '7004', chargeMode: 'word_charge', readable: true, accessReason: 'membership', membershipEntitled: true, purchasable: true, chapterPrice: 3 }
      }
    ]);
    readerApi.getBook.mockResolvedValueOnce({ ...book, productStatus: { ...book.productStatus, chargeMode: 'word_charge' } });

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    expect(screen.getByText('免费')).toBeInTheDocument();
    expect(screen.getByText('9,223,372,036,854,775,807 币')).toBeInTheDocument();
    expect(screen.getByText('已购买')).toBeInTheDocument();
    expect(screen.getByText('会员可读')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '第1章：第一章 旧城，免费' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '第2章：第一章 旧城，9,223,372,036,854,775,807 币' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '第3章：第一章 旧城，已购买' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '第4章：第一章 旧城，会员可读' })).toBeInTheDocument();
  });

  it('shows login-free chapters as free when no chapter price exists', async () => {
    const chapters = await readerApi.listBookChapters();
    readerApi.listBookChapters.mockResolvedValueOnce([
      {
        ...chapters[0],
        accessStatus: {
          ...chapters[0].accessStatus,
          chargeMode: 'login_free',
          readable: true,
          accessReason: 'login_free',
          chapterPrice: null
        }
      }
    ]);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    expect(screen.getByText('免费')).toBeInTheDocument();
  });

  it('shows unsupported book and chapter states without login, membership or purchase actions', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        accessReason: 'unsupported_mode',
        membershipEntitled: true,
        purchasable: true,
        priceCoin: '9223372036854775807'
      }
    });
    const chapters = await readerApi.listBookChapters();
    readerApi.listBookChapters.mockResolvedValueOnce([{
      ...chapters[0],
      accessStatus: {
        ...chapters[0].accessStatus,
        accessReason: 'unsupported_mode',
        membershipEntitled: true,
        purchasable: true,
        chapterPrice: '9223372036854775807'
      }
    }]);

    renderBookDetailPage();

    expect(await screen.findAllByText('暂不可读')).not.toHaveLength(0);
    fireEvent.click(screen.getByRole('tab', { name: '章节' }));
    expect(screen.getAllByText('暂不可读')).not.toHaveLength(0);
    const disabledChapter = screen.getByRole('link', { name: /第1章：第一章 旧城，暂不可读/ });
    expect(disabledChapter).toHaveAttribute('aria-disabled', 'true');
    expect(disabledChapter).not.toHaveAttribute('href');
    expect(disabledChapter).not.toHaveAttribute('tabindex');
    fireEvent.click(disabledChapter);
    fireEvent.keyDown(disabledChapter, { key: 'Enter' });
    expect(screen.queryByTestId('location-state')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /登录|会员/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /购买/ })).not.toBeInTheDocument();
  });

  it('fails an unknown book access reason closed instead of inferring membership access', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        accessReason: 'future_reason',
        membershipEntitled: true
      }
    });

    renderBookDetailPage();

    expect(await screen.findByText('暂不可读')).toBeInTheDocument();
    expect(screen.queryByText('会员可读')).not.toBeInTheDocument();
  });

  it('does not offer fixed-price purchase for an unknown wire reason', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'fixed_price',
        accessReason: 'future_reason',
        purchasable: true,
        priceCoin: 12
      }
    });

    renderBookDetailPage();

    expect(await screen.findByText('暂不可读')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '永久购买整本 12 币' })).not.toBeInTheDocument();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
  });

  it('does not offer permanent word purchase for member flags with an unknown wire reason', async () => {
    const chapters = await readerApi.listBookChapters();
    readerApi.listBookChapters.mockResolvedValueOnce([{
      ...chapters[0],
      accessStatus: {
        ...chapters[0].accessStatus,
        chargeMode: 'word_charge',
        accessReason: 'future_reason',
        readable: true,
        membershipEntitled: true,
        purchasable: true,
        chapterPrice: 3
      }
    }]);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    expect(screen.getByText('暂不可读')).toBeInTheDocument();
    const disabledChapter = screen.getByRole('link', { name: /暂不可读/ });
    expect(disabledChapter).toHaveAttribute('aria-disabled', 'true');
    expect(disabledChapter).not.toHaveAttribute('href');
    expect(disabledChapter).not.toHaveAttribute('tabindex');
    expect(screen.queryByRole('button', { name: /永久购买/ })).not.toBeInTheDocument();
  });

  it.each([
    ['book ownership', { chargeMode: 'future_mode', accessReason: 'book_owned', bookPurchased: true }],
    ['chapter ownership', { chargeMode: null, accessReason: 'chapter_owned', chapterPurchased: true }]
  ])('keeps chapter navigation interactive for unknown mode with %s', async (_label, access) => {
    const chapters = await readerApi.listBookChapters();
    readerApi.listBookChapters.mockResolvedValueOnce([{
      ...chapters[0],
      accessStatus: { ...chapters[0].accessStatus, ...access, readable: false }
    }]);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));
    fireEvent.click(screen.getByRole('link', { name: /已购买/ }));

    expect(await screen.findByTestId('location-state')).toHaveTextContent('"chapterId":"9223372036854775807"');
  });

  it('keeps the book reading entry for unknown mode with permanent ownership', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'future_mode',
        accessReason: 'book_owned',
        purchased: true,
        readable: false
      }
    });

    renderBookDetailPage();

    expect(await screen.findByText('已购买')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '开始阅读' })).toBeInTheDocument();
  });

  it.each(['future_mode', null, ''] as const)('fails book wire mode %s closed without actions', async (chargeMode) => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode,
        accessReason: 'membership',
        readable: true,
        membershipEntitled: true,
        purchasable: true,
        priceCoin: 12
      }
    });

    renderBookDetailPage();

    expect(await screen.findByText('暂不可读')).toBeInTheDocument();
    expect(screen.queryByText('会员可读')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /购买/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /登录|会员/ })).not.toBeInTheDocument();
  });

  it('buys a fixed-price book only after confirmation and refreshes reader state', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValue({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'fixed_price',
        accessReason: 'book_purchase_required',
        purchasable: true,
        productId: '9223372036854775807',
        productName: '月下长书永久阅读',
        priceCoin: '9223372036854775807',
        saleStatus: 'on_sale'
      }
    });
    readerApi.buyBook.mockResolvedValue({});
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 9,223,372,036,854,775,807 币' }));

    expect(confirmSpy).toHaveBeenCalledOnce();
    await waitFor(() => expect(readerApi.buyBook).toHaveBeenCalledWith(
      '9223372036854775806',
      '9223372036854775807'
    ));
    await waitFor(() => {
      expect(readerApi.getBook).toHaveBeenCalledTimes(2);
      expect(readerApi.listBookChapters).toHaveBeenCalledTimes(2);
      expect(readerApi.getWallet).toHaveBeenCalledOnce();
    });
    confirmSpy.mockRestore();
  });

  it('does not buy a fixed-price book when confirmation is cancelled', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'fixed_price', accessReason: 'book_purchase_required', purchasable: true, priceCoin: 12 }
    });
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 12 币' }));

    expect(readerApi.buyBook).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it.each([
    ['涨价', '12', '15'],
    ['降价', '12', '9']
  ])('refreshes a %s quote and requires a second explicit confirmation', async (_label, oldPrice, newPrice) => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    const pricedBook = (priceCoin: string) => ({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'fixed_price' as const,
        accessReason: 'book_purchase_required' as const,
        purchasable: true,
        priceCoin
      }
    });
    readerApi.getBook
      .mockResolvedValueOnce(pricedBook(oldPrice))
      .mockResolvedValueOnce(pricedBook(newPrice))
      .mockResolvedValueOnce(pricedBook(newPrice));
    readerApi.buyBook
      .mockRejectedValueOnce(new ApiError(QUOTE_CHANGED_CODE, '作品报价已变化，请确认新价格'))
      .mockResolvedValueOnce({});
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderBookDetailPage();

    fireEvent.click(await screen.findByRole('button', { name: `永久购买整本 ${oldPrice} 币` }));
    expect(await screen.findByRole('button', { name: `永久购买整本 ${newPrice} 币` })).toBeInTheDocument();
    expect(readerApi.buyBook).toHaveBeenCalledTimes(1);
    expect(readerApi.buyBook).toHaveBeenLastCalledWith('9223372036854775806', oldPrice);
    expect(screen.getByRole('alert')).toHaveTextContent('价格已更新，请重新确认');

    fireEvent.click(screen.getByRole('button', { name: `永久购买整本 ${newPrice} 币` }));
    await waitFor(() => expect(readerApi.buyBook).toHaveBeenCalledTimes(2));
    expect(readerApi.buyBook).toHaveBeenLastCalledWith('9223372036854775806', newPrice);
    expect(confirmSpy).toHaveBeenCalledTimes(2);
    confirmSpy.mockRestore();
  });

  it('keeps refreshed purchase rights when the wallet refresh fails', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    const purchasableBook = {
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'fixed_price' as const, accessReason: 'book_purchase_required' as const, purchasable: true, priceCoin: 12 }
    };
    readerApi.getBook
      .mockResolvedValueOnce(purchasableBook)
      .mockResolvedValueOnce({
        ...purchasableBook,
        productStatus: { ...purchasableBook.productStatus, purchased: true, entitled: true, purchasable: false, accessReason: 'book_owned' }
      });
    readerApi.buyBook.mockResolvedValue({});
    readerApi.getWallet.mockRejectedValueOnce(new Error('钱包暂时不可用'));
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 12 币' }));

    expect(await screen.findByText('已购买')).toBeInTheDocument();
    expect(screen.queryByText('购买失败')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '永久购买整本 12 币' })).not.toBeInTheDocument();
    confirmSpy.mockRestore();
  });

  it('offers membership entry to an authenticated non-member', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'membership_only', accessReason: 'membership_required' }
    });

    renderBookDetailPage();

    expect(await screen.findByRole('link', { name: '开通会员' })).toHaveAttribute('href', '/me');
  });

  it('asks an anonymous membership-only reader to log in and preserves the detail return location', async () => {
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'membership_only', accessReason: 'login_required' }
    });

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('link', { name: '登录后开通会员' }));

    const state = await screen.findByTestId('location-state');
    expect(state).toHaveTextContent('"from":{"pathname":"/books/9223372036854775806"');
  });

  it('keeps membership reading and permanent chapter purchase as separate actions', async () => {
    mockAuthenticatedReader();
    const chapters = await readerApi.listBookChapters();
    readerApi.listBookChapters.mockResolvedValueOnce([
      {
        ...chapters[0],
        accessStatus: {
          ...chapters[0].accessStatus,
          chargeMode: 'word_charge',
          readable: true,
          accessReason: 'membership',
          membershipEntitled: true,
          chapterPurchased: false,
          purchasable: true,
          chapterPrice: 3
        }
      }
    ]);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    expect(screen.getByText('会员可读')).toBeInTheDocument();
    const permanentPurchase = screen.getByRole('button', { name: '永久购买第1章' });
    expect(permanentPurchase).toBeEnabled();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
    fireEvent.click(permanentPurchase);
    expect(await screen.findByTestId('location-state')).toHaveTextContent('"purchaseReturn":{"pathname":"/books/9223372036854775806"');
  });

  it('keeps active membership and voluntary fixed-price purchase as separate actions', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValueOnce({
      ...book,
      productStatus: {
        ...book.productStatus,
        chargeMode: 'fixed_price',
        accessReason: 'membership',
        readable: true,
        membershipEntitled: true,
        purchasable: true,
        priceCoin: 12
      }
    });
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 12 币' }));

    expect(confirmSpy).toHaveBeenCalledOnce();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it('offers permanent word purchase for chapter-purchase-required access', async () => {
    const chapters = await readerApi.listBookChapters();
    readerApi.listBookChapters.mockResolvedValueOnce([{
      ...chapters[0],
      accessStatus: {
        ...chapters[0].accessStatus,
        chargeMode: 'word_charge',
        accessReason: 'chapter_purchase_required',
        readable: false,
        membershipEntitled: false,
        purchasable: true,
        chapterPrice: 3
      }
    }]);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));

    expect(screen.getByRole('button', { name: '永久购买第1章' })).toBeEnabled();
  });

  it('preserves the current reader location when an anonymous reader opens a chapter', async () => {
    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('tab', { name: '章节' }));
    fireEvent.click(screen.getByRole('link', { name: '第1章：第一章 旧城，免费' }));

    expect(await screen.findByTestId('location-state')).toHaveTextContent('"chapterId":"9223372036854775807"');
    expect(screen.getByTestId('location-state')).toHaveTextContent('"from":{"pathname":"/read/9223372036854775806"');
    expect(screen.getByTestId('location-state')).toHaveTextContent('"state":{"chapterId":"9223372036854775807"');
  });

  it('publishes the refreshed wallet to auth state after a fixed-price purchase', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValue({
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'fixed_price', accessReason: 'book_purchase_required', purchasable: true, priceCoin: 12 }
    });
    readerApi.buyBook.mockResolvedValue({});
    readerApi.getWallet.mockResolvedValueOnce({
      readerId: '9223372036854775801',
      rechargeCoinBalance: '9223372036854775795',
      bonusCoinBalance: 0,
      totalRechargeCoinIncome: 0,
      totalBonusCoinIncome: 0,
      totalRechargeCoinExpense: 0,
      totalBonusCoinExpense: 0,
      expiringBonusCoin: 0
    });
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 12 币' }));

    expect(await screen.findByTestId('wallet-balance')).toHaveTextContent('9223372036854775795');
    confirmSpy.mockRestore();
  });

  it('locks a pending fixed-price purchase against duplicate clicks', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValue({
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'fixed_price', accessReason: 'book_purchase_required', purchasable: true, priceCoin: 12 }
    });
    readerApi.buyBook.mockReturnValue(new Promise(() => undefined));
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderBookDetailPage();
    const purchase = await screen.findByRole('button', { name: '永久购买整本 12 币' });
    fireEvent.click(purchase);
    fireEvent.click(purchase);

    expect(purchase).toBeDisabled();
    expect(readerApi.buyBook).toHaveBeenCalledOnce();
    expect(confirmSpy).toHaveBeenCalledOnce();
    confirmSpy.mockRestore();
  });

  it('invalidates a pending purchase when navigating to another book', async () => {
    mockAuthenticatedReader();
    const firstBook = mockBookWithTwoChapters();
    const fixedFirstBook = {
      ...firstBook,
      productStatus: { ...firstBook.productStatus, chargeMode: 'fixed_price' as const, accessReason: 'book_purchase_required' as const, purchasable: true, priceCoin: 12 }
    };
    const secondBook = {
      ...fixedFirstBook,
      bookId: '9223372036854775809',
      bookName: '新月之书',
      productStatus: { ...fixedFirstBook.productStatus, bookId: '9223372036854775809', priceCoin: 22 }
    };
    readerApi.getBook.mockImplementation((bookId: string) => Promise.resolve(bookId === secondBook.bookId ? secondBook : fixedFirstBook));
    readerApi.listBookChapters.mockResolvedValue([]);
    const purchaseRequest = deferred<Record<string, never>>();
    readerApi.buyBook.mockReturnValueOnce(purchaseRequest.promise);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 12 币' }));
    fireEvent.click(screen.getByRole('button', { name: '切换作品' }));

    expect((await screen.findAllByText('新月之书')).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: '永久购买整本 22 币' })).toBeEnabled();
    await act(async () => { purchaseRequest.resolve({}); await purchaseRequest.promise; });
    expect(screen.getAllByText('新月之书').length).toBeGreaterThan(0);
    expect(readerApi.getBook).toHaveBeenCalledTimes(2);
    confirmSpy.mockRestore();
  });

  it('does not refresh purchase state after unmounting a pending detail page', async () => {
    mockAuthenticatedReader();
    const book = mockBookWithTwoChapters();
    readerApi.getBook.mockResolvedValue({
      ...book,
      productStatus: { ...book.productStatus, chargeMode: 'fixed_price', accessReason: 'book_purchase_required', purchasable: true, priceCoin: 12 }
    });
    const purchaseRequest = deferred<Record<string, never>>();
    readerApi.buyBook.mockReturnValueOnce(purchaseRequest.promise);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const view = renderBookDetailPage();
    fireEvent.click(await screen.findByRole('button', { name: '永久购买整本 12 币' }));
    view.unmount();

    await act(async () => { purchaseRequest.resolve({}); await purchaseRequest.promise; });
    expect(readerApi.getBook).toHaveBeenCalledOnce();
    expect(readerApi.listBookChapters).toHaveBeenCalledOnce();
    confirmSpy.mockRestore();
  });
});
