import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { StrictMode } from 'react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { ApiError, QUOTE_CHANGED_CODE } from '../api/http';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import type { ReaderChapterPurchaseResult, ReaderChapterSummary } from '../types/reader';
import { ReaderPage } from './ReaderPage';

const readerApi = vi.hoisted(() => ({
  buyBook: vi.fn(),
  buyChapter: vi.fn(),
  getBookHistory: vi.fn(),
  getChapter: vi.fn(),
  getPreference: vi.fn(),
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  listBookChapters: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  registerReader: vi.fn(),
  savePreference: vi.fn(),
  updateHistory: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Button: ({ children, disabled, onClick }: { children: React.ReactNode; disabled?: boolean; onClick?: () => void }) => (
    <button disabled={disabled} type="button" onClick={onClick}>
      {children}
    </button>
  ),
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <section className={className}>{children}</section>
  ),
  Loading: () => <div data-testid="loading">加载中</div>,
  Radio: ({
    options,
    value,
    onChange
  }: {
    options: Array<{ label: string; value: string }>;
    value: string;
    onChange: (value: string) => void;
  }) => (
    <div>
      {options.map((option) => (
        <button
          aria-pressed={option.value === value}
          key={option.value}
          type="button"
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  ),
  Title: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>
}));

const chapter = {
  chapterId: '9223372036854775807',
  bookId: '9223372036854775806',
  chapterNo: 8,
  chapterName: '第八章',
  bookName: '月下长书',
  content: '第一段正文\n第二段正文',
  prevChapterId: '9223372036854775805',
  nextChapterId: '9223372036854775804'
};

const nextChapter = {
  ...chapter,
  chapterId: '9223372036854775804',
  chapterNo: 9,
  chapterName: '第九章',
  content: '下一章正文',
  prevChapterId: '9223372036854775807',
  nextChapterId: ''
};

const prevChapter = {
  ...chapter,
  chapterId: '9223372036854775805',
  chapterNo: 7,
  chapterName: '第七章',
  content: '上一章正文',
  prevChapterId: '',
  nextChapterId: '9223372036854775807'
};

const history = {
  historyId: '9223372036854775803',
  bookId: '9223372036854775806',
  chapterId: '9223372036854775807',
  chapterNo: 8,
  chapterName: '第八章',
  positionType: 'scroll',
  positionValue: 860,
  progressPercent: '48.20',
  lastReadTime: '2026-06-27 10:00:00'
};

const preference = {
  preferenceId: '9223372036854775802',
  fontSize: 18,
  lineHeight: '1.8',
  theme: 'cream',
  readingMode: 'scroll'
};

function LocationProbe() {
  const location = useLocation();
  return <span data-testid="reader-location">{location.pathname}{location.search}|{JSON.stringify(location.state)}</span>;
}

function NavigationControls() {
  const navigate = useNavigate();
  return (
    <div>
      <button type="button" onClick={() => navigate('/read/9223372036854775806', { state: { chapterId: nextChapter.chapterId } })}>切到章节B</button>
      <button type="button" onClick={() => navigate('/read/9223372036854775700', { state: { chapterId: '9223372036854775701' } })}>切换作品</button>
      <button type="button" onClick={() => navigate('/read/9223372036854775806', { state: { chapterId: chapter.chapterId, permanentPurchase: true } })}>新的永久购买意图</button>
      <button type="button" onClick={() => navigate('/read/9223372036854775806', { state: { chapterId: chapter.chapterId } })}>普通阅读章节A</button>
    </div>
  );
}

function BackButton() {
  const navigate = useNavigate();
  return <button type="button" onClick={() => navigate(-1)}>浏览器返回</button>;
}

function renderReaderPage(
  state?: { chapterId?: string; permanentPurchase?: boolean },
  navigationControls = false,
  strictMode = false
) {
  const content = (
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={[{ pathname: '/read/9223372036854775806', state }]}>
        <Routes>
          <Route
            path="/read/:bookId"
            element={(
              <>
                <LocationProbe />
                {navigationControls ? <NavigationControls /> : null}
                <ReaderPage />
              </>
            )}
          />
          <Route path="/books/:bookId" element={<><LocationProbe />{navigationControls ? <NavigationControls /> : null}<div>作品详情页</div></>} />
          <Route path="/auth/login" element={<><LocationProbe /><div>登录页</div></>} />
          <Route path="/me" element={<><LocationProbe /><div>个人中心</div></>} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
  return render(strictMode ? <StrictMode>{content}</StrictMode> : content);
}

function renderReaderHistory(
  initialEntries: Array<string | { pathname: string; state?: Record<string, unknown> }>,
  initialIndex = initialEntries.length - 1
) {
  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={initialEntries} initialIndex={initialIndex}>
        <Routes>
          <Route path="/" element={<><LocationProbe /><div>进入详情之前的页面</div></>} />
          <Route path="/read/:bookId" element={<><LocationProbe /><ReaderPage /></>} />
          <Route path="/books/:bookId" element={<><LocationProbe /><BackButton /><div>作品详情页</div></>} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

function accessChapter(overrides: Partial<ReaderChapterSummary['accessStatus']> = {}): ReaderChapterSummary {
  const accessStatus = {
    bookId: chapter.bookId,
    chapterId: chapter.chapterId,
    chargeMode: 'word_charge' as const,
    readable: false,
    accessReason: 'chapter_purchase_required' as const,
    membershipEntitled: false,
    bookPurchased: false,
    chapterPurchased: false,
    purchasable: true,
    chapterWordCount: 2500,
    pricingWordUnit: 1000,
    pricingCoinUnit: 1,
    chapterPrice: 3,
    ...overrides
  };
  return {
    chapterId: accessStatus.chapterId,
    bookId: accessStatus.bookId,
    chapterNo: 8,
    chapterName: '第八章',
    wordCount: 2500,
    updateTime: '2026-06-27 10:00:00',
    accessStatus,
    productStatus: {
      bookId: accessStatus.bookId,
      chargeMode: accessStatus.chargeMode,
      readable: accessStatus.readable,
      accessReason: accessStatus.accessReason,
      entitled: accessStatus.readable,
      purchased: accessStatus.bookPurchased,
      membershipEntitled: accessStatus.membershipEntitled,
      purchasable: accessStatus.purchasable,
      productId: '9223372036854775000',
      productName: '整书永久阅读',
      priceCoin: '9223372036854775803',
      saleStatus: 'on_sale',
      product: null
    }
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => { resolve = nextResolve; });
  return { promise, resolve };
}

function paidChapterResult(targetChapterId = chapter.chapterId): ReaderChapterPurchaseResult {
  return {
    purchaseStatus: 'paid',
    quote: {
      chapterId: targetChapterId,
      bookId: chapter.bookId,
      wordCount: 2500,
      wordUnit: 1000,
      coinUnit: 1,
      priceCoin: 3
    },
    order: null
  };
}

describe('ReaderPage history restoration', () => {
  beforeEach(() => {
    window.localStorage.clear();
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
    vi.clearAllMocks();
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
    vi.stubGlobal('scrollTo', vi.fn());
    readerApi.getChapter.mockImplementation((nextChapterId: string) => {
      return Promise.resolve(nextChapterId === nextChapter.chapterId ? nextChapter : chapter);
    });
    readerApi.getBookHistory.mockResolvedValue(history);
    readerApi.getPreference.mockResolvedValue(preference);
    readerApi.savePreference.mockResolvedValue(preference);
    readerApi.updateHistory.mockResolvedValue(history);
    const firstSummary = accessChapter({ readable: true, accessReason: 'chapter_owned', chapterPurchased: true });
    firstSummary.chapterName = '人物设定';
    firstSummary.wordCount = 1200;
    const nextSummary = accessChapter({ chapterId: nextChapter.chapterId, readable: true, accessReason: 'chapter_owned', chapterPurchased: true });
    nextSummary.chapterNo = 9;
    nextSummary.chapterName = '第九章';
    nextSummary.wordCount = 1300;
    const previousSummary = accessChapter({ chapterId: prevChapter.chapterId, readable: true, accessReason: 'chapter_owned', chapterPurchased: true });
    previousSummary.chapterNo = 7;
    previousSummary.chapterName = '第七章';
    readerApi.listBookChapters.mockResolvedValue([firstSummary, nextSummary, previousSummary]);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('uses the chapter and book names as the document title and restores it on unmount', async () => {
    document.title = '月白书城';
    const view = renderReaderPage();

    await waitFor(() => expect(document.title).toBe('第八章-月下长书'));
    view.unmount();
    expect(document.title).toBe('月白书城');
  });

  it('restores saved scroll only after the chapter content has rendered', async () => {
    let resolveChapters: (chapters: ReaderChapterSummary[]) => void = () => undefined;
    readerApi.listBookChapters.mockReturnValue(
      new Promise<ReaderChapterSummary[]>((resolve) => {
        resolveChapters = resolve;
      })
    );

    renderReaderPage();

    await waitFor(() => expect(readerApi.getBookHistory).toHaveBeenCalledWith('9223372036854775806'));
    expect(screen.getByTestId('loading')).toBeInTheDocument();
    expect(window.scrollTo).not.toHaveBeenCalled();

    await act(async () => {
      resolveChapters([
        {
          chapterId: '9223372036854775807',
          bookId: '9223372036854775806',
          chapterNo: 8,
          chapterName: '第八章',
          wordCount: 1200,
          updateTime: '2026-06-27 10:00:00',
          accessStatus: {
            bookId: '9223372036854775806',
            chapterId: '9223372036854775807',
            chargeMode: 'login_free',
            readable: true,
            accessReason: 'login_free',
            membershipEntitled: false,
            bookPurchased: false,
            chapterPurchased: false,
            purchasable: false,
            chapterWordCount: null,
            pricingWordUnit: null,
            pricingCoinUnit: null,
            chapterPrice: null
          },
          productStatus: {
            bookId: '9223372036854775806',
            chargeMode: 'login_free',
            readable: true,
            accessReason: 'login_free',
            entitled: false,
            purchased: false,
            membershipEntitled: false,
            purchasable: false,
            productId: null,
            productName: null,
            priceCoin: null,
            saleStatus: null,
            product: null
          }
        }
      ]);
    });

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(window.scrollTo).toHaveBeenCalledWith({ top: 860, behavior: 'instant' });
  });

  it('does not render the fixed chapter header above the reading content', async () => {
    const { container } = renderReaderPage();

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(container.querySelector('.reader-view__top')).not.toBeInTheDocument();
  });

  it('renders the reader bar in scroll mode', async () => {
    renderReaderPage();

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(screen.getByRole('banner', { name: '阅读顶部栏' })).toHaveClass('reader-view__reader-bar');
    expect(screen.getByRole('banner', { name: '阅读顶部栏' })).toHaveTextContent('月下长书');
    expect(screen.getByRole('button', { name: '返回详情' })).toBeInTheDocument();
  });

  it('hides the reader chrome and open dock content when scrolling starts in scroll mode', async () => {
    const { container } = renderReaderPage();

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(container.querySelector('.reader-view__reader-bar')).toBeInTheDocument();
    expect(container.querySelector('.reader-view__toolbar')).toBeInTheDocument();

    Object.defineProperty(document.documentElement, 'scrollHeight', {
      configurable: true,
      value: 5000
    });
    vi.stubGlobal('innerHeight', 800);
    vi.stubGlobal('scrollY', 200);
    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 10_000);

    fireEvent.click(screen.getByRole('button', { name: '目录' }));
    expect(screen.getByRole('dialog', { name: '章节目录' })).toBeInTheDocument();
    fireEvent.scroll(window);

    await waitFor(() => {
      expect(container.querySelector('.reader-view__reader-bar')).not.toBeInTheDocument();
      expect(container.querySelector('.reader-view__toolbar')).not.toBeInTheDocument();
      expect(screen.queryByRole('dialog', { name: '章节目录' })).not.toBeInTheDocument();
    });

    fireEvent.click(container.querySelector('.reader-view__content') as HTMLElement);
    expect(container.querySelector('.reader-view__reader-bar')).toBeInTheDocument();
    expect(container.querySelector('.reader-view__toolbar')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '配色' }));
    expect(container.querySelector('.reader-palette-popover')).toBeInTheDocument();
    fireEvent.scroll(window);
    await waitFor(() => expect(container.querySelector('.reader-palette-popover')).not.toBeInTheDocument());

    fireEvent.click(container.querySelector('.reader-view__content') as HTMLElement);
    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    expect(container.querySelector('.reader-settings-popover')).toBeInTheDocument();
    fireEvent.scroll(window);
    await waitFor(() => expect(container.querySelector('.reader-settings-popover')).not.toBeInTheDocument());
  });

  it('opens settings from the dock without detail return and close actions', async () => {
    renderReaderPage();

    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByRole('button', { name: '设置' }));

    expect(document.querySelector('.reader-settings-popover')).toHaveClass('reader-settings-popover--dock');
    fireEvent.click(screen.getByRole('button', { name: '上下滚动' }));

    const settingsPopover = document.querySelector('.reader-settings-popover') as HTMLElement;
    expect(screen.getByText('翻页阅读')).toBeInTheDocument();
    expect(screen.queryByText('卡片阅读')).not.toBeInTheDocument();
    expect(within(settingsPopover).queryByRole('button', { name: '返回详情' })).not.toBeInTheDocument();
    expect(within(settingsPopover).queryByRole('button', { name: '关闭' })).not.toBeInTheDocument();
  });

  it('attaches scroll-mode palette and settings panels directly above the dock', async () => {
    renderReaderPage();

    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByRole('button', { name: '配色' }));
    expect(document.querySelector('.reader-palette-popover')).toHaveClass('reader-palette-popover--dock');

    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    expect(document.querySelector('.reader-settings-popover')).toHaveClass('reader-settings-popover--dock');
    expect(document.querySelector('.reader-palette-popover')).not.toBeInTheDocument();
  });

  it('keeps the settings card open after choosing a reading mode', async () => {
    renderReaderPage();

    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    fireEvent.click(screen.getByRole('button', { name: '上下滚动' }));
    fireEvent.click(screen.getByRole('button', { name: '翻页阅读' }));

    expect(screen.getByLabelText('字号')).toBeInTheDocument();
    expect(document.querySelector('.reader-settings-popover')).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: '阅读方式' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument();
    expect(readerApi.getChapter).toHaveBeenCalledTimes(1);
  });

  it('appends the next chapter seamlessly after scrolling to the bottom in scroll mode', async () => {
    let resolveNextChapter: (chapter: typeof nextChapter) => void = () => undefined;
    readerApi.getChapter.mockImplementation((nextChapterId: string) => {
      if (nextChapterId === nextChapter.chapterId) {
        return new Promise((resolve) => {
          resolveNextChapter = resolve;
        });
      }
      return Promise.resolve(chapter);
    });
    renderReaderPage();

    await screen.findByText('第一段正文');
    vi.clearAllMocks();
    Object.defineProperty(document.documentElement, 'scrollHeight', {
      configurable: true,
      value: 2000
    });
    vi.stubGlobal('innerHeight', 800);
    vi.stubGlobal('scrollY', 1198);
    const afterRestoreWindow = Date.now() + 10_000;
    vi.spyOn(Date, 'now').mockReturnValue(afterRestoreWindow);

    fireEvent.scroll(window);

    await waitFor(() => expect(readerApi.getChapter).toHaveBeenCalledWith('9223372036854775804'));
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
    expect(screen.queryByTestId('loading')).not.toBeInTheDocument();
    expect(screen.getByText('第一段正文')).toBeInTheDocument();
    expect(window.scrollTo).not.toHaveBeenCalled();

    await act(async () => {
      resolveNextChapter(nextChapter);
    });

    expect(screen.getByText('第一段正文')).toBeInTheDocument();
    expect(await screen.findByText('下一章正文')).toBeInTheDocument();
    expect(screen.getByText('第八章')).toBeInTheDocument();
    expect(screen.getByText('第九章')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
    expect(window.scrollTo).not.toHaveBeenCalledWith({ top: 0, behavior: 'instant' });
  });

  it('appends the next chapter from a short scroll chapter that cannot actually scroll', async () => {
    let resolveNextChapter: (chapter: typeof nextChapter) => void = () => undefined;
    readerApi.getChapter.mockImplementation((nextChapterId: string) => {
      if (nextChapterId === nextChapter.chapterId) {
        return new Promise((resolve) => {
          resolveNextChapter = resolve;
        });
      }
      return Promise.resolve(chapter);
    });
    renderReaderPage();

    await screen.findByText('第一段正文');
    vi.clearAllMocks();
    Object.defineProperty(document.documentElement, 'scrollHeight', {
      configurable: true,
      value: 620
    });
    vi.stubGlobal('innerHeight', 800);
    vi.stubGlobal('scrollY', 0);
    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 10_000);

    fireEvent.wheel(window, { deltaY: 120 });

    await waitFor(() => expect(readerApi.getChapter).toHaveBeenCalledWith('9223372036854775804'));
    expect(screen.getByText('第一段正文')).toBeInTheDocument();

    await act(async () => {
      resolveNextChapter(nextChapter);
    });

    expect(await screen.findByText('下一章正文')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
  });

  it('prepends the previous chapter from a short scroll chapter that cannot actually scroll upward', async () => {
    let resolvePrevChapter: (chapter: typeof prevChapter) => void = () => undefined;
    readerApi.getChapter.mockImplementation((nextChapterId: string) => {
      if (nextChapterId === prevChapter.chapterId) {
        return new Promise((resolve) => {
          resolvePrevChapter = resolve;
        });
      }
      return Promise.resolve(chapter);
    });
    renderReaderPage();

    await screen.findByText('第一段正文');
    vi.clearAllMocks();
    let scrollHeight = 620;
    Object.defineProperty(document.documentElement, 'scrollHeight', {
      configurable: true,
      get: () => scrollHeight
    });
    vi.stubGlobal('innerHeight', 800);
    vi.stubGlobal('scrollY', 0);
    vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 10_000);

    fireEvent.wheel(window, { deltaY: -120 });

    await waitFor(() => expect(readerApi.getChapter).toHaveBeenCalledWith('9223372036854775805'));
    expect(screen.getByText('第一段正文')).toBeInTheDocument();

    await act(async () => {
      scrollHeight = 1040;
      resolvePrevChapter(prevChapter);
    });

    expect(await screen.findByText('上一章正文')).toBeInTheDocument();
    expect(screen.getByText('第一段正文')).toBeInTheDocument();
    expect(window.scrollTo).toHaveBeenCalledWith({ top: 420, behavior: 'instant' });
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
  });

  it('plays a closing state before removing the settings card', async () => {
    renderReaderPage();

    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    expect(document.querySelector('.reader-settings-popover')).not.toHaveClass('is-closing');

    vi.useFakeTimers();
    fireEvent.click(screen.getByRole('button', { name: '设置' }));

    expect(document.querySelector('.reader-settings-popover')).toHaveClass('is-closing');
    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(document.querySelector('.reader-settings-popover')).not.toBeInTheDocument();
  });

  it('shows compact numbered chapter rows in the drawer', async () => {
    renderReaderPage();

    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByRole('button', { name: '目录' }));

    expect(screen.getByRole('button', { name: '第8章：人物设定，已购买' })).toHaveClass('chapter-list__item');
    expect(screen.queryByText('1200 字')).not.toBeInTheDocument();
  });

  it('uses the locally saved preference before remote preference loads', async () => {
    window.localStorage.removeItem(READER_TOKEN_KEY);
    window.localStorage.removeItem(READER_PROFILE_KEY);
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 20, lineHeight: '2.0', theme: 'night', readingMode: 'page' })
    );
    const { container } = renderReaderPage();

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(container.querySelector('.reader-theme--night')).toBeInTheDocument();
    expect(screen.queryByText('加载中')).not.toBeInTheDocument();
  });

  function mockPagedLayout(pageCount: number, waitForColumnWidth = false, pageWidth = 400) {
    vi.spyOn(Element.prototype, 'clientWidth', 'get').mockImplementation(function (this: Element) {
      return this.classList?.contains('reader-pager__clip') ? Math.round(pageWidth) : 0;
    });
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      const width = this.classList?.contains('reader-pager__clip') ? pageWidth : 0;
      return {
        bottom: 0,
        height: 0,
        left: 0,
        right: width,
        top: 0,
        width,
        x: 0,
        y: 0,
        toJSON: () => ({})
      };
    });
    vi.spyOn(Element.prototype, 'scrollWidth', 'get').mockImplementation(function (this: Element) {
      if (!this.classList?.contains('reader-pager__track')) {
        return 0;
      }
      if (waitForColumnWidth && (this as HTMLElement).style.columnWidth !== `${pageWidth}px`) {
        return Math.round(pageWidth);
      }
      return Math.round(pageWidth * pageCount);
    });
  }

  it('remeasures fixed pages after the page column width is applied', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3, true);

    renderReaderPage();
    await screen.findByText('第一段正文');

    expect(await screen.findByText('1 / 3')).toBeInTheDocument();
  });

  it('keeps page gutters outside the paged track so the next page cannot peek in', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({
        fontSize: 18,
        lineHeight: '1.8',
        theme: 'cream',
        readingMode: 'page',
        marginSize: 2,
        indentMode: 'indent'
      })
    );
    vi.stubGlobal('innerWidth', 400);
    mockPagedLayout(3, false, 320);

    renderReaderPage();
    await screen.findByText('第一段正文');

    const pageClip = document.querySelector('.reader-pager__clip') as HTMLElement;
    const pageTrack = document.querySelector('.reader-pager__track') as HTMLElement;
    expect(pageClip).toBeInTheDocument();
    expect(pageTrack.parentElement).toBe(pageClip);
    expect(pageTrack.style.columnWidth).toBe('320px');
    expect(pageTrack.style.paddingLeft).toBe('');
    expect(pageTrack.style.paddingRight).toBe('');

    fireEvent.pointerDown(screen.getByLabelText('阅读右侧区域'), { clientX: 350, clientY: 360 });
    fireEvent.pointerUp(screen.getByLabelText('阅读右侧区域'), { clientX: 350, clientY: 360 });

    expect(await screen.findByText('2 / 3')).toBeInTheDocument();
    expect(pageClip.scrollLeft).toBe(320);
    expect(pageTrack.style.transform).toBe('');
  });

  it('keeps fractional mobile pages aligned after many page turns', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    vi.stubGlobal('innerWidth', 320.4);
    mockPagedLayout(17, false, 320.4);

    renderReaderPage();
    await screen.findByText('第一段正文');
    expect(await screen.findByText('1 / 17')).toBeInTheDocument();

    const pageTurner = document.querySelector('.reader-page-turner') as Element;
    const pageClip = document.querySelector('.reader-pager__clip') as HTMLElement;
    const pageTrack = document.querySelector('.reader-pager__track') as HTMLElement;
    for (let page = 1; page < 15; page += 1) {
      fireEvent.pointerDown(pageTurner, { clientX: 300, clientY: 360 });
      fireEvent.pointerUp(pageTurner, { clientX: 300, clientY: 360 });
    }

    expect(await screen.findByText('15 / 17')).toBeInTheDocument();
    expect(pageTrack.style.columnWidth).toBe('320.4px');
    expect(pageClip.scrollLeft).toBeCloseTo(320.4 * 14);
    expect(pageTrack.style.transform).toBe('');
  });

  it('shows paged reading chrome only in fixed page mode', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    renderReaderPage();

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(screen.getByText('第八章', { selector: '.reader-page-chapter' })).toHaveClass('reader-page-chapter');
    expect(screen.getByLabelText('阅读页码')).toHaveClass('reader-page-count');
    expect(screen.getByLabelText('阅读页码')).toHaveTextContent('1 / 3');
    expect(screen.queryByRole('navigation', { name: '阅读操作' })).not.toBeInTheDocument();
  });

  it('uses the center page zone to toggle the reader dock', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('第一段正文');

    fireEvent.click(screen.getByLabelText('阅读中部区域'));

    expect(screen.getByRole('navigation', { name: '阅读操作' })).toBeInTheDocument();
    expect(screen.getByRole('banner', { name: '阅读顶部栏' })).toHaveTextContent('月下长书');
    expect(screen.getByRole('button', { name: '返回详情' })).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('阅读中部区域'));

    expect(screen.queryByRole('navigation', { name: '阅读操作' })).not.toBeInTheDocument();
  });

  it('keeps the center dock area as a middle rectangle so bottom taps still page', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    vi.stubGlobal('innerWidth', 400);
    vi.stubGlobal('innerHeight', 800);
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('1 / 3');

    const pageTurner = document.querySelector('.reader-page-turner') as Element;
    fireEvent.pointerDown(pageTurner, { clientX: 260, clientY: 760 });
    fireEvent.pointerUp(pageTurner, { clientX: 260, clientY: 760 });

    expect(screen.queryByRole('navigation', { name: '阅读操作' })).not.toBeInTheDocument();
    expect(await screen.findByText('2 / 3')).toBeInTheDocument();

    fireEvent.pointerDown(pageTurner, { clientX: 200, clientY: 400 });
    fireEvent.pointerUp(pageTurner, { clientX: 200, clientY: 400 });

    expect(screen.getByRole('navigation', { name: '阅读操作' })).toBeInTheDocument();
  });

  it('keeps chapter drawer and settings mutually exclusive and toggled by dock buttons', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByLabelText('阅读中部区域'));

    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    expect(screen.getByLabelText('字号')).toBeInTheDocument();
    expect(screen.queryByText('字号')).not.toBeInTheDocument();
    expect(screen.queryByText('章节目录')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '目录' }));
    expect(screen.getByText('章节目录')).toBeInTheDocument();
    expect(screen.getByRole('dialog', { name: '章节目录' })).toHaveClass('reader-drawer--dock');
    expect(document.querySelector('.reader-drawer__panel')).toHaveClass('reader-drawer__panel--dock');
    expect(screen.queryByLabelText('字号')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '目录' }));
    expect(screen.queryByText('章节目录')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    expect(screen.getByLabelText('字号')).toBeInTheDocument();
    vi.useFakeTimers();
    fireEvent.click(screen.getByLabelText('阅读中部区域'));
    expect(document.querySelector('.reader-settings-popover')).toHaveClass('is-closing');
    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(screen.queryByLabelText('字号')).not.toBeInTheDocument();
  });

  it('renders an icon-only reader dock and opens an independent palette panel', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    const { container } = renderReaderPage();
    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByLabelText('阅读中部区域'));

    const dock = screen.getByRole('navigation', { name: '阅读操作' });
    const dockButtons = within(dock).getAllByRole('button');
    expect(dockButtons).toHaveLength(3);
    expect(dockButtons.map((button) => button.getAttribute('aria-label'))).toEqual(['目录', '配色', '设置']);
    expect(within(dock).queryByRole('button', { name: '上一章' })).not.toBeInTheDocument();
    expect(within(dock).queryByRole('button', { name: '下一章' })).not.toBeInTheDocument();
    expect(within(dock).queryByText('目录')).not.toBeInTheDocument();
    expect(within(dock).queryByText('配色')).not.toBeInTheDocument();
    expect(within(dock).queryByText('设置')).not.toBeInTheDocument();

    fireEvent.click(within(dock).getByRole('button', { name: '配色' }));
    expect(within(dock).getByRole('button', { name: '配色' })).toHaveClass('is-active');
    expect(screen.getByRole('group', { name: '配色选择' })).toBeInTheDocument();
    expect(screen.queryByText('章节目录')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('字号')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '夜间' }));
    expect(container.querySelector('.reader-theme--night')).toBeInTheDocument();
    expect(readerApi.savePreference).toHaveBeenCalledWith(
      expect.objectContaining({
        theme: 'night'
      })
    );
    expect(screen.getByRole('button', { name: '夜间' })).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(within(dock).getByRole('button', { name: '目录' }));
    expect(screen.getByText('章节目录')).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: '配色选择' })).not.toBeInTheDocument();
    expect(within(dock).getByRole('button', { name: '目录' })).toHaveClass('is-active');
  });

  it('attaches the settings panel to the page dock and keeps settings controls title-free', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByLabelText('阅读中部区域'));
    fireEvent.click(screen.getByRole('button', { name: '设置' }));

    const popover = document.querySelector('.reader-settings-popover');
    const fontRange = screen.getByLabelText('字号').closest('.reader-setting-range--font');
    const marginInput = screen.getByLabelText('边距大小') as HTMLInputElement;
    const lineHeightInput = screen.getByLabelText('行距松紧') as HTMLInputElement;
    const marginRange = marginInput.closest('.reader-setting-range');
    const lineHeightRange = lineHeightInput.closest('.reader-setting-range');
    const choices = Array.from(document.querySelectorAll('.reader-setting-choice'));
    expect(popover).toHaveClass('reader-settings-popover--dock');
    expect(fontRange).toBeInTheDocument();
    const fontControl = fontRange?.querySelector('.reader-setting-range__control');
    expect(fontControl).toHaveAttribute('data-value', '18');
    expect(fontControl?.querySelector('.reader-setting-range__mark--small')).toHaveTextContent('A');
    expect(fontControl?.querySelector('.reader-setting-range__mark--large')).toHaveTextContent('A');
    expect(marginRange?.querySelector('.reader-setting-range__control')).toHaveAttribute('data-value', '边距');
    expect(marginInput).toHaveAttribute('max', '6');
    expect(marginInput).toHaveAttribute('step', '1');
    expect(marginInput.value).toBe('3');
    expect(marginRange?.querySelector('.reader-setting-range__mark--small')).toHaveTextContent('小');
    expect(marginRange?.querySelector('.reader-setting-range__mark--large')).toHaveTextContent('大');
    expect(lineHeightRange?.querySelector('.reader-setting-range__control')).toHaveAttribute('data-value', '行距');
    expect(lineHeightInput).toHaveAttribute('max', '6');
    expect(lineHeightInput).toHaveAttribute('step', '1');
    expect(lineHeightInput.value).toBe('3');
    expect(lineHeightRange?.querySelector('.reader-setting-range__mark--small')).toHaveTextContent('紧');
    expect(lineHeightRange?.querySelector('.reader-setting-range__mark--large')).toHaveTextContent('松');
    expect(document.querySelector('.reader-setting-range__label')).not.toBeInTheDocument();
    expect(document.querySelector('.reader-setting-range__bubble')).not.toBeInTheDocument();
    expect(document.querySelector('.reader-setting-range__value')).not.toBeInTheDocument();
    expect(screen.queryByText('字号')).not.toBeInTheDocument();
    expect(screen.queryByText('边距')).not.toBeInTheDocument();
    expect(screen.queryByText('行距')).not.toBeInTheDocument();
    expect(screen.queryByText('阅读方式')).not.toBeInTheDocument();
    expect(screen.queryByText('字体选择')).not.toBeInTheDocument();
    expect(choices).toHaveLength(3);
    expect(choices[0].querySelector('span:not(.reader-setting-choice__chevron)')).not.toBeInTheDocument();
    expect(choices[0].querySelector('strong')).toHaveTextContent('系统字体');
    expect(choices[0].querySelector('.reader-setting-choice__chevron')).toHaveTextContent('>');
    expect(choices[1].querySelector('span:not(.reader-setting-choice__chevron)')).not.toBeInTheDocument();
    expect(choices[1].querySelector('strong')).toHaveTextContent('首行缩进');
    expect(choices[1].querySelector('.reader-setting-choice__chevron')).toHaveTextContent('>');
    expect(choices[2].querySelector('span:not(.reader-setting-choice__chevron)')).not.toBeInTheDocument();
    expect(choices[2].querySelector('strong')).toHaveTextContent('翻页阅读');
    expect(choices[2].querySelector('.reader-setting-choice__chevron')).toHaveTextContent('>');
  });

  it('opens each settings choice in an overlay window without resizing the settings card flow', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByLabelText('阅读中部区域'));
    fireEvent.click(screen.getByRole('button', { name: '设置' }));

    const openChoicePanel = (choiceIndex: number, title: string) => {
      const choice = document.querySelectorAll('.reader-setting-choice')[choiceIndex] as HTMLButtonElement;
      fireEvent.click(choice);
      const panel = screen.getByRole('group', { name: title });
      expect(panel).toHaveClass('reader-settings__choice-window');
      expect(panel).toHaveClass('reader-settings__choice-window--full');
      expect(panel.querySelector('.reader-settings__choice-title')).toHaveTextContent(title);
      expect(screen.getByRole('button', { name: `收起${title}` })).toHaveClass('reader-settings__collapse');
      expect(screen.getByLabelText('字号')).toBeInTheDocument();
      expect(screen.getByLabelText('边距大小')).toBeInTheDocument();
      expect(screen.getByLabelText('行距松紧')).toBeInTheDocument();
      expect(document.querySelector('.reader-settings > .reader-settings__choice-panel')).not.toBeInTheDocument();
    };

    openChoicePanel(0, '字体选择');
    expect(screen.getByRole('button', { name: '宋体' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '收起字体选择' }));
    expect(screen.queryByRole('group', { name: '字体选择' })).not.toBeInTheDocument();

    openChoicePanel(1, '首行缩进');
    expect(screen.getByRole('button', { name: '无缩进' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '收起首行缩进' }));
    expect(screen.queryByRole('group', { name: '首行缩进' })).not.toBeInTheDocument();

    openChoicePanel(2, '阅读方式');
    expect(screen.getByRole('button', { name: '上下滚动' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '收起阅读方式' }));
    expect(screen.queryByRole('group', { name: '阅读方式' })).not.toBeInTheDocument();
  });

  it('prevents the settings choice overlay from passing pointer events to the range controls', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('第一段正文');
    fireEvent.click(screen.getByLabelText('阅读中部区域'));
    fireEvent.click(screen.getByRole('button', { name: '设置' }));

    const fontSizeInput = screen.getByLabelText('字号') as HTMLInputElement;
    fireEvent.click(document.querySelectorAll('.reader-setting-choice')[0]);
    fireEvent.pointerDown(screen.getByRole('group', { name: '字体选择' }));
    fireEvent.pointerUp(screen.getByRole('group', { name: '字体选择' }));
    fireEvent.click(screen.getByRole('button', { name: '收起字体选择' }));

    expect(screen.queryByRole('group', { name: '字体选择' })).not.toBeInTheDocument();
    expect(fontSizeInput.value).toBe('18');
  });

  it('applies local margin and first-line indent preferences to reading paragraphs', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    readerApi.getChapter.mockResolvedValue({
      ...chapter,
      content: '第一段正文\n\n第二段正文'
    });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({
        fontSize: 18,
        lineHeight: '1.8',
        theme: 'cream',
        readingMode: 'page',
        marginSize: 2,
        indentMode: 'indent'
      })
    );
    mockPagedLayout(3);

    renderReaderPage();

    const paragraphs = await screen.findAllByText(/正文/);
    expect(paragraphs).toHaveLength(2);
    expect(paragraphs[0]).toHaveClass('reader-paragraph');
    expect(paragraphs[0].closest('.reader-pager__viewport')).toHaveAttribute('data-margin-size', '2');
    expect(paragraphs[0].closest('.reader-pager__track')).toHaveClass('has-indent');
  });

  it('turns fixed reader pages with horizontal scrolling without scrolling the document', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    vi.stubGlobal('innerWidth', 400);
    vi.stubGlobal('scrollY', 0);
    vi.stubGlobal('scrollBy', vi.fn());
    mockPagedLayout(3);

    renderReaderPage();
    await screen.findByText('第一段正文');
    expect(await screen.findByText('1 / 3')).toBeInTheDocument();

    const pageTurner = document.querySelector('.reader-page-turner') as Element;
    const pageClip = document.querySelector('.reader-pager__clip') as HTMLElement;
    const pageTrack = document.querySelector('.reader-pager__track') as HTMLElement;
    expect(pageClip.scrollLeft).toBe(0);
    expect(pageTrack.style.transform).toBe('');

    fireEvent.pointerDown(pageTurner, { clientX: 320, clientY: 360 });
    fireEvent.pointerUp(pageTurner, { clientX: 320, clientY: 360 });

    expect(window.scrollBy).not.toHaveBeenCalled();
    expect(await screen.findByText('2 / 3')).toBeInTheDocument();
    expect(pageClip.scrollLeft).toBe(400);
    expect(pageTrack.style.transform).toBe('');
    expect(readerApi.getChapter).toHaveBeenCalledTimes(1);
  });

  it('loads the next fixed-reading chapter asynchronously without changing the book URL', async () => {
    readerApi.getPreference.mockResolvedValue({ ...preference, readingMode: 'page' });
    window.localStorage.setItem(
      'moonbook.reader.preference',
      JSON.stringify({ fontSize: 18, lineHeight: '1.8', theme: 'cream', readingMode: 'page' })
    );
    vi.stubGlobal('innerWidth', 400);
    vi.stubGlobal('scrollBy', vi.fn());
    mockPagedLayout(2);
    let resolveNextChapter: (chapter: typeof nextChapter) => void = () => undefined;
    readerApi.getChapter.mockImplementation((nextChapterId: string) => {
      if (nextChapterId === nextChapter.chapterId) {
        return new Promise((resolve) => {
          resolveNextChapter = resolve;
        });
      }
      return Promise.resolve(chapter);
    });

    renderReaderPage();
    await screen.findByText('1 / 2');

    const pageTurner = document.querySelector('.reader-page-turner') as Element;
    fireEvent.pointerDown(pageTurner, { clientX: 320, clientY: 360 });
    fireEvent.pointerUp(pageTurner, { clientX: 320, clientY: 360 });
    expect(await screen.findByText('2 / 2')).toBeInTheDocument();
    expect(readerApi.getChapter).toHaveBeenCalledTimes(1);

    fireEvent.pointerDown(pageTurner, { clientX: 320, clientY: 360 });
    fireEvent.pointerUp(pageTurner, { clientX: 320, clientY: 360 });

    await waitFor(() => expect(readerApi.getChapter).toHaveBeenCalledWith('9223372036854775804'));
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
    expect(screen.queryByTestId('loading')).not.toBeInTheDocument();
    expect(screen.getByText('第一段正文')).toBeInTheDocument();

    await act(async () => {
      resolveNextChapter(nextChapter);
    });

    expect(await screen.findByText('下一章正文')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
    expect(readerApi.updateHistory).toHaveBeenCalledWith(
      '9223372036854775806',
      expect.objectContaining({
        chapterId: '9223372036854775807',
        positionType: 'page',
        positionValue: 1
      })
    );
  });
});

describe('ReaderPage purchase access flow', () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.localStorage.setItem(READER_TOKEN_KEY, 'reader.jwt.token');
    window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify({
      readerId: '9223372036854775801', username: 'reader', nickname: '读者', status: 'enabled'
    }));
    vi.clearAllMocks();
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { callback(0); return 1; });
    vi.stubGlobal('scrollTo', vi.fn());
    readerApi.getBookHistory.mockResolvedValue(history);
    readerApi.getPreference.mockResolvedValue(preference);
    readerApi.savePreference.mockResolvedValue(preference);
    readerApi.updateHistory.mockResolvedValue(history);
    readerApi.getChapter.mockResolvedValue(chapter);
    readerApi.getWallet.mockResolvedValue({ readerId: '9223372036854775801', rechargeCoinBalance: 8, bonusCoinBalance: 2 });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('buys a chapter only after explicit confirmation, refreshes state, and then loads content', async () => {
    const locked = accessChapter({ chapterPrice: '9223372036854775803' });
    readerApi.listBookChapters.mockResolvedValue([locked]);
    readerApi.buyChapter.mockResolvedValue({
      purchaseStatus: 'paid',
      quote: { chapterId: chapter.chapterId, bookId: chapter.bookId, wordCount: 2500, wordUnit: 1000, coinUnit: 1, priceCoin: '9223372036854775803' },
      order: {}
    });

    renderReaderPage();
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 9,223,372,036,854,775,803 币' }));

    await waitFor(() => expect(readerApi.buyChapter).toHaveBeenCalledWith(
      '9223372036854775807', '9223372036854775803', expect.any(String)
    ));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(readerApi.listBookChapters).toHaveBeenCalledTimes(2);
    expect(readerApi.getWallet).toHaveBeenCalledOnce();
    expect(readerApi.listBookChapters.mock.invocationCallOrder[1]).toBeLessThan(readerApi.getChapter.mock.invocationCallOrder[0]);
  });

  it('cancels without buying or fetching protected content', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    renderReaderPage();
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('opens purchase confirmation when selecting a locked chapter after readable content', async () => {
    const readable = accessChapter({ readable: true, accessReason: 'chapter_owned', chapterPurchased: true });
    const locked = accessChapter({ chapterId: nextChapter.chapterId, chapterPrice: 4 });
    locked.chapterNo = 9;
    locked.chapterName = '第九章';
    readerApi.listBookChapters.mockResolvedValue([readable, locked]);
    renderReaderPage();
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '目录' }));
    fireEvent.click(screen.getByRole('button', { name: /第9章：第九章/ }));
    expect(await screen.findByRole('button', { name: '确认支付 4 币' })).toBeInTheDocument();
    expect(readerApi.getChapter).toHaveBeenCalledOnce();
  });

  it('requires a new confirmation and request id after the quote changes', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    readerApi.buyChapter
      .mockResolvedValueOnce({ purchaseStatus: 'quote_changed', quote: { chapterId: chapter.chapterId, bookId: chapter.bookId, wordCount: 3500, wordUnit: 1000, coinUnit: 1, priceCoin: 4 }, order: null })
      .mockResolvedValueOnce({ purchaseStatus: 'paid', quote: { chapterId: chapter.chapterId, bookId: chapter.bookId, wordCount: 3500, wordUnit: 1000, coinUnit: 1, priceCoin: 4 }, order: {} });
    renderReaderPage();

    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    expect(await screen.findByText('价格已更新，请重新确认')).toBeInTheDocument();
    expect(readerApi.buyChapter).toHaveBeenCalledTimes(1);
    const firstRequestId = readerApi.buyChapter.mock.calls[0][2];
    fireEvent.click(screen.getByRole('button', { name: '确认支付 4 币' }));
    await waitFor(() => expect(readerApi.buyChapter).toHaveBeenCalledTimes(2));
    expect(readerApi.buyChapter.mock.calls[1][2]).not.toBe(firstRequestId);
  });

  it('shows a backend purchase error, keeps content locked, and allows retry', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    readerApi.buyChapter.mockRejectedValueOnce(new Error('余额不足')).mockResolvedValueOnce({
      purchaseStatus: 'already_owned',
      quote: { chapterId: chapter.chapterId, bookId: chapter.bookId, wordCount: 2500, wordUnit: 1000, coinUnit: 1, priceCoin: 3 },
      order: null
    });
    renderReaderPage();

    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    expect(await screen.findByText('余额不足')).toBeInTheDocument();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    const firstRequestId = readerApi.buyChapter.mock.calls[0][2];
    fireEvent.click(screen.getByRole('button', { name: '确认支付 3 币' }));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(readerApi.buyChapter.mock.calls[1][2]).toBe(firstRequestId);
  });

  it('locks duplicate clicks while a chapter purchase is pending', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    readerApi.buyChapter.mockReturnValue(new Promise(() => undefined));
    renderReaderPage();
    const confirm = await screen.findByRole('button', { name: '确认支付 3 币' });
    fireEvent.click(confirm);
    fireEvent.click(confirm);
    expect(readerApi.buyChapter).toHaveBeenCalledOnce();
    expect(confirm).toBeDisabled();
  });

  it.each(['paid', 'already_owned', 'free'] as const)('refreshes rights and reads content for %s', async (purchaseStatus) => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    readerApi.buyChapter.mockResolvedValue({
      purchaseStatus,
      quote: { chapterId: chapter.chapterId, bookId: chapter.bookId, wordCount: 2500, wordUnit: 1000, coinUnit: 1, priceCoin: 3 },
      order: null
    });
    renderReaderPage();
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
  });

  it('reads membership-entitled content directly unless permanent purchase was requested', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]);
    const view = renderReaderPage();
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /确认支付/ })).not.toBeInTheDocument();
    view.unmount();

    renderReaderPage({ chapterId: chapter.chapterId, permanentPurchase: true });
    expect(await screen.findByRole('button', { name: '确认支付 3 币' })).toBeInTheDocument();
    expect(readerApi.getChapter).toHaveBeenCalledOnce();
  });

  it('redirects login-required access with the exact reader target', async () => {
    window.localStorage.clear();
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ accessReason: 'login_required', purchasable: false })]);
    readerApi.getBookHistory.mockResolvedValue(null);
    renderReaderPage({ chapterId: chapter.chapterId });
    expect(await screen.findByText('登录页')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/auth/login');
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('offers membership action without fetching content', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ accessReason: 'membership_required', chargeMode: 'membership_only', purchasable: false, chapterPrice: null })]);
    renderReaderPage();
    fireEvent.click(await screen.findByRole('button', { name: '开通会员' }));
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/me');
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('fails unsupported mode closed without fetching, buying or redirecting', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({
      accessReason: 'unsupported_mode',
      readable: false,
      membershipEntitled: true,
      purchasable: true
    })]);

    renderReaderPage();

    expect(await screen.findByText('章节暂不可读')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
  });

  it('fails an unknown access reason closed without choosing an action', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({
      accessReason: 'future_reason',
      readable: false,
      purchasable: true
    })]);

    renderReaderPage();

    expect(await screen.findByText('章节暂不可读')).toBeInTheDocument();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
  });

  it.each(['future_mode', null, ''] as const)('fails wire charge mode %s closed even for member-readable state', async (chargeMode) => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({
      chargeMode,
      accessReason: 'membership',
      readable: true,
      membershipEntitled: true,
      purchasable: true
    })]);

    renderReaderPage({ chapterId: chapter.chapterId, permanentPurchase: true });

    expect(await screen.findByText('章节暂不可读')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/read/9223372036854775806');
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /确认支付|确认整书购买/ })).not.toBeInTheDocument();
  });

  it.each([
    ['book reason', { chargeMode: 'future_mode', accessReason: 'book_owned', bookPurchased: true }],
    ['chapter reason', { chargeMode: null, accessReason: 'chapter_owned', chapterPurchased: true }],
    ['book flag', { chargeMode: '', accessReason: 'future_reason', bookPurchased: true }],
    ['chapter flag', { chargeMode: 'future_mode', accessReason: 'future_reason', chapterPurchased: true }]
  ])('loads content before validating unknown wire state for permanent ownership by %s', async (_label, access) => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ ...access, readable: false })]);

    renderReaderPage({ chapterId: chapter.chapterId });

    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(readerApi.getChapter).toHaveBeenCalledWith(chapter.chapterId);
    expect(readerApi.buyBook).not.toHaveBeenCalled();
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
  });

  it('confirms a whole-book purchase with the exact string id and prevents duplicate requests', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ accessReason: 'book_purchase_required', chargeMode: 'fixed_price', chapterPrice: null })]);
    readerApi.buyBook.mockReturnValue(new Promise(() => undefined));
    renderReaderPage();
    const confirm = await screen.findByRole('button', { name: '确认整书购买 9,223,372,036,854,775,803 币' });
    fireEvent.click(confirm);
    fireEvent.click(confirm);
    expect(readerApi.buyBook).toHaveBeenCalledOnce();
    expect(readerApi.buyBook).toHaveBeenCalledWith('9223372036854775806', '9223372036854775803');
    expect(confirm).toBeDisabled();
  });

  it('does not offer whole-book purchase when product wire reason is unknown', async () => {
    const locked = accessChapter({
      accessReason: 'book_purchase_required',
      chargeMode: 'fixed_price',
      chapterPrice: null
    });
    locked.productStatus.accessReason = 'future_reason';
    readerApi.listBookChapters.mockResolvedValue([locked]);

    renderReaderPage();

    expect(await screen.findByText('本书暂不可购买')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /确认整书购买/ })).not.toBeInTheDocument();
    expect(readerApi.buyBook).not.toHaveBeenCalled();
  });

  it('refreshes access and wallet before loading content after a whole-book purchase', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ accessReason: 'book_purchase_required', chargeMode: 'fixed_price', chapterPrice: null })]);
    readerApi.buyBook.mockResolvedValue({});
    renderReaderPage();
    fireEvent.click(await screen.findByRole('button', { name: '确认整书购买 9,223,372,036,854,775,803 币' }));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(readerApi.listBookChapters).toHaveBeenCalledTimes(2);
    expect(readerApi.getWallet).toHaveBeenCalledOnce();
    expect(readerApi.listBookChapters.mock.invocationCallOrder[1]).toBeLessThan(readerApi.getChapter.mock.invocationCallOrder[0]);
  });

  it.each([
    ['涨价', '9223372036854775803', '9223372036854775807', '9,223,372,036,854,775,807'],
    ['降价', '9223372036854775803', '8', '8']
  ])('refreshes a whole-book %s and requires a new purchase intent', async (_label, oldPrice, newPrice, displayPrice) => {
    const oldChapter = accessChapter({ accessReason: 'book_purchase_required', chargeMode: 'fixed_price', chapterPrice: null });
    oldChapter.productStatus.priceCoin = oldPrice;
    const refreshed = accessChapter({ accessReason: 'book_purchase_required', chargeMode: 'fixed_price', chapterPrice: null });
    refreshed.productStatus.priceCoin = newPrice;
    readerApi.listBookChapters
      .mockResolvedValueOnce([oldChapter])
      .mockResolvedValueOnce([refreshed])
      .mockResolvedValueOnce([accessChapter()]);
    readerApi.buyBook
      .mockRejectedValueOnce(new ApiError(QUOTE_CHANGED_CODE, '作品报价已变化，请确认新价格'))
      .mockResolvedValueOnce({});
    renderReaderPage();

    fireEvent.click(await screen.findByRole('button', { name: /确认整书购买/ }));
    const refreshedConfirm = await screen.findByRole('button', { name: `确认整书购买 ${displayPrice} 币` });
    expect(readerApi.buyBook).toHaveBeenCalledTimes(1);
    expect(readerApi.buyBook).toHaveBeenLastCalledWith('9223372036854775806', oldPrice);
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    expect(readerApi.getWallet).toHaveBeenCalledOnce();

    fireEvent.click(refreshedConfirm);
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(readerApi.buyBook).toHaveBeenLastCalledWith('9223372036854775806', newPrice);
  });

  it.each([
    ['切换收费模式', { chargeMode: 'membership_only' as const, accessReason: 'membership_required' as const, purchasable: false }],
    ['商品下架', { chargeMode: 'fixed_price' as const, accessReason: 'book_purchase_required' as const, purchasable: false }],
    ['并发获得权益', { chargeMode: 'fixed_price' as const, accessReason: 'book_owned' as const, readable: true, bookPurchased: true, purchasable: false }]
  ])('clears the stale whole-book prompt when refresh reports %s', async (_label, overrides) => {
    const oldChapter = accessChapter({ accessReason: 'book_purchase_required', chargeMode: 'fixed_price', chapterPrice: null });
    const refreshed = accessChapter({ ...overrides, chapterPrice: null });
    refreshed.productStatus.priceCoin = null;
    refreshed.productStatus.saleStatus = 'off_sale';
    readerApi.listBookChapters.mockResolvedValueOnce([oldChapter]).mockResolvedValueOnce([refreshed]);
    readerApi.buyBook.mockRejectedValueOnce(
      new ApiError(QUOTE_CHANGED_CODE, '作品报价已变化，请确认新价格')
    );
    renderReaderPage();

    fireEvent.click(await screen.findByRole('button', { name: /确认整书购买/ }));

    expect(await screen.findByText('整书购买状态已变化，请重新选择')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /确认整书购买/ })).not.toBeInTheDocument();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
    expect(readerApi.getWallet).toHaveBeenCalledOnce();
  });

  it('fails closed when the selected chapter has no structured access status', async () => {
    const invalid = accessChapter();
    delete (invalid as Partial<ReaderChapterSummary>).accessStatus;
    readerApi.listBookChapters.mockResolvedValue([invalid]);
    renderReaderPage();
    expect(await screen.findByText('章节权限信息缺失')).toBeInTheDocument();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('keeps purchased content visible when only the directory refresh fails', async () => {
    readerApi.listBookChapters
      .mockResolvedValueOnce([accessChapter()])
      .mockRejectedValueOnce(new Error('目录刷新失败'));
    readerApi.buyChapter.mockResolvedValue({
      purchaseStatus: 'paid',
      quote: { chapterId: chapter.chapterId, bookId: chapter.bookId, wordCount: 2500, wordUnit: 1000, coinUnit: 1, priceCoin: 3 },
      order: {}
    });
    renderReaderPage();
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('章节状态刷新失败');
  });

  it('does not publish a pending chapter purchase after routing to another chapter in the same book', async () => {
    const purchase = deferred<ReaderChapterPurchaseResult>();
    const lockedA = accessChapter();
    const readableB = accessChapter({ chapterId: nextChapter.chapterId, readable: true, accessReason: 'chapter_owned', chapterPurchased: true });
    readableB.chapterNo = 9;
    readableB.chapterName = '第九章';
    readerApi.listBookChapters.mockResolvedValue([lockedA, readableB]);
    readerApi.getChapter.mockImplementation((id: string) => Promise.resolve(id === nextChapter.chapterId ? nextChapter : chapter));
    readerApi.buyChapter.mockReturnValue(purchase.promise);
    renderReaderPage(undefined, true);
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    fireEvent.click(screen.getByRole('button', { name: '切到章节B' }));
    expect(await screen.findByText('下一章正文')).toBeInTheDocument();

    await act(async () => purchase.resolve(paidChapterResult()));
    expect(screen.getByText('下一章正文')).toBeInTheDocument();
    expect(readerApi.getChapter).not.toHaveBeenCalledWith(chapter.chapterId);
  });

  it('does not let an obsolete purchase finally unlock the new chapter purchase', async () => {
    const purchaseA = deferred<ReaderChapterPurchaseResult>();
    const purchaseB = deferred<ReaderChapterPurchaseResult>();
    const lockedA = accessChapter();
    const lockedB = accessChapter({ chapterId: nextChapter.chapterId, chapterPrice: 4 });
    lockedB.chapterNo = 9;
    lockedB.chapterName = '第九章';
    readerApi.listBookChapters.mockResolvedValue([lockedA, lockedB]);
    readerApi.buyChapter.mockReturnValueOnce(purchaseA.promise).mockReturnValueOnce(purchaseB.promise);
    renderReaderPage(undefined, true);
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    fireEvent.click(screen.getByRole('button', { name: '切到章节B' }));
    const confirmB = await screen.findByRole('button', { name: '确认支付 4 币' });
    fireEvent.click(confirmB);
    expect(readerApi.buyChapter).toHaveBeenCalledTimes(2);
    await act(async () => purchaseA.resolve(paidChapterResult()));
    expect(confirmB).toBeDisabled();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('does not publish a pending chapter purchase after routing to another book', async () => {
    const purchase = deferred<ReaderChapterPurchaseResult>();
    const otherContent = { ...nextChapter, chapterId: '9223372036854775701', bookId: '9223372036854775700', content: '另一本书正文' };
    const otherSummary = accessChapter({ bookId: otherContent.bookId, chapterId: otherContent.chapterId, readable: true, accessReason: 'chapter_owned', chapterPurchased: true });
    readerApi.listBookChapters.mockImplementation((id: string) => Promise.resolve(id === otherContent.bookId ? [otherSummary] : [accessChapter()]));
    readerApi.getChapter.mockImplementation((id: string) => Promise.resolve(id === otherContent.chapterId ? otherContent : chapter));
    readerApi.buyChapter.mockReturnValue(purchase.promise);
    renderReaderPage(undefined, true);
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    fireEvent.click(screen.getByRole('button', { name: '切换作品' }));
    expect(await screen.findByText('另一本书正文')).toBeInTheDocument();
    await act(async () => purchase.resolve(paidChapterResult()));
    expect(readerApi.getChapter).not.toHaveBeenCalledWith(chapter.chapterId);
  });

  it('does not fetch purchased content after a pending chapter purchase unmounts', async () => {
    const purchase = deferred<ReaderChapterPurchaseResult>();
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    readerApi.buyChapter.mockReturnValue(purchase.promise);
    const view = renderReaderPage();
    fireEvent.click(await screen.findByRole('button', { name: '确认支付 3 币' }));
    view.unmount();
    await act(async () => purchase.resolve(paidChapterResult()));
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('does not publish a pending whole-book purchase after routing to another chapter', async () => {
    const purchase = deferred<Record<string, never>>();
    const lockedA = accessChapter({ chargeMode: 'fixed_price', accessReason: 'book_purchase_required', chapterPrice: null });
    const readableB = accessChapter({ chapterId: nextChapter.chapterId, chargeMode: 'fixed_price', readable: true, accessReason: 'book_owned', bookPurchased: true });
    readableB.chapterNo = 9;
    readableB.chapterName = '第九章';
    readerApi.listBookChapters.mockResolvedValue([lockedA, readableB]);
    readerApi.getChapter.mockImplementation((id: string) => Promise.resolve(id === nextChapter.chapterId ? nextChapter : chapter));
    readerApi.buyBook.mockReturnValue(purchase.promise);
    renderReaderPage(undefined, true);
    fireEvent.click(await screen.findByRole('button', { name: /确认整书购买/ }));
    fireEvent.click(screen.getByRole('button', { name: '切到章节B' }));
    expect(await screen.findByText('下一章正文')).toBeInTheDocument();
    await act(async () => purchase.resolve({}));
    expect(readerApi.getChapter).not.toHaveBeenCalledWith(chapter.chapterId);
  });

  it('consumes permanent purchase state once and handles a later navigation as a new intent', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]);
    renderReaderPage({ chapterId: chapter.chapterId, permanentPurchase: true }, true);
    expect(await screen.findByRole('button', { name: '确认支付 3 币' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId('reader-location')).not.toHaveTextContent('permanentPurchase'));
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    expect(await screen.findByText('作品详情页')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).not.toHaveTextContent('permanentPurchase');
    fireEvent.click(screen.getByRole('button', { name: '普通阅读章节A' }));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '确认支付 3 币' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '新的永久购买意图' }));
    expect(await screen.findByRole('button', { name: '确认支付 3 币' })).toBeInTheDocument();
  });

  it('handles a new permanent purchase intent for the currently open member-readable chapter', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]);
    renderReaderPage({ chapterId: chapter.chapterId }, true);
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '新的永久购买意图' }));
    expect(await screen.findByRole('button', { name: '确认支付 3 币' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId('reader-location')).not.toHaveTextContent('permanentPurchase'));
  });

  it('does not apply an obsolete permanent purchase intent after returning to ordinary reading', async () => {
    const chaptersRequest = deferred<ReaderChapterSummary[]>();
    readerApi.listBookChapters.mockReturnValue(chaptersRequest.promise);
    renderReaderPage({ chapterId: chapter.chapterId, permanentPurchase: true }, true);
    fireEvent.click(screen.getByRole('button', { name: '普通阅读章节A' }));
    await act(async () => chaptersRequest.resolve([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]));
    expect(await screen.findByText('第一段正文')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '确认支付 3 币' })).not.toBeInTheDocument();
  });

  it('consumes one permanent purchase navigation intent once under StrictMode', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]);
    renderReaderPage({ chapterId: chapter.chapterId, permanentPurchase: true }, false, true);
    expect(await screen.findByRole('button', { name: '确认支付 3 币' })).toBeInTheDocument();
    expect(screen.getAllByRole('dialog', { name: '购买本章后阅读' })).toHaveLength(1);
    await waitFor(() => expect(screen.getByTestId('reader-location')).not.toHaveTextContent('permanentPurchase'));
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
  });

  it('applies permanent purchase only to the chapter named by the navigation state', async () => {
    const memberA = accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true, chapterPrice: 3 });
    const memberB = accessChapter({ chapterId: nextChapter.chapterId, readable: true, accessReason: 'membership', membershipEntitled: true, chapterPrice: 4 });
    memberB.chapterNo = 9;
    readerApi.listBookChapters.mockResolvedValue([memberA, memberB]);
    renderReaderPage({ chapterId: nextChapter.chapterId, permanentPurchase: true });
    expect(await screen.findByRole('button', { name: '确认支付 4 币' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '确认支付 3 币' })).not.toBeInTheDocument();
  });

  it('pops back to the originating detail entry without creating a duplicate detail history entry', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]);
    renderReaderHistory([
      '/',
      '/books/9223372036854775806',
      {
        pathname: '/read/9223372036854775806',
        state: {
          chapterId: chapter.chapterId,
          permanentPurchase: true,
          purchaseReturn: { pathname: '/books/9223372036854775806', search: '', hash: '' }
        }
      }
    ]);
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(await screen.findByText('作品详情页')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '浏览器返回' }));
    expect(await screen.findByText('进入详情之前的页面')).toBeInTheDocument();
  });

  it('falls back safely to replacing the matching detail for a direct permanent reader entry', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter({ readable: true, accessReason: 'membership', membershipEntitled: true })]);
    renderReaderHistory([{
      pathname: '/read/9223372036854775806',
      state: { chapterId: chapter.chapterId, permanentPurchase: true }
    }]);
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(await screen.findByText('作品详情页')).toBeInTheDocument();
    expect(screen.getByTestId('reader-location')).toHaveTextContent('/books/9223372036854775806');
  });

  it('cancels a focused non-pending purchase with Escape without buying or fetching content', async () => {
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    renderReaderPage();
    const confirm = await screen.findByRole('button', { name: '确认支付 3 币' });
    expect(confirm).toHaveFocus();
    fireEvent.keyDown(confirm, { key: 'Escape' });
    expect(await screen.findByText('作品详情页')).toBeInTheDocument();
    expect(readerApi.buyChapter).not.toHaveBeenCalled();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });

  it('keeps a pending focused purchase locked when Escape is pressed', async () => {
    const purchase = deferred<ReaderChapterPurchaseResult>();
    readerApi.listBookChapters.mockResolvedValue([accessChapter()]);
    readerApi.buyChapter.mockReturnValue(purchase.promise);
    renderReaderPage();
    const confirm = await screen.findByRole('button', { name: '确认支付 3 币' });
    fireEvent.click(confirm);
    fireEvent.keyDown(confirm, { key: 'Escape' });
    expect(confirm).toBeDisabled();
    expect(screen.getByRole('dialog', { name: '购买本章后阅读' })).toBeInTheDocument();
    expect(readerApi.buyChapter).toHaveBeenCalledOnce();
    expect(readerApi.getChapter).not.toHaveBeenCalled();
  });
});
