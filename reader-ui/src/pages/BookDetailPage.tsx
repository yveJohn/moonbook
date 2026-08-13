import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { Loading } from 'animal-island-ui';
import { addBookshelf, buyBook, getBook, getBookHistory, likeBook, listBookChapters, unlikeBook } from '../api/reader';
import { isApiErrorCode, QUOTE_CHANGED_CODE } from '../api/http';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import { ChapterAccessStatus, getChapterAccessLabel } from '../components/ChapterDrawer';
import { isBookChargeMode } from '../types/reader';
import type {
  ReaderBookDetail,
  ReaderBookProductStatus,
  ReaderChapterAccess,
  ReaderChapterSummary,
  ReaderReadingHistory
} from '../types/reader';
import { formatReaderLong } from '../utils/formatReaderLong';
import { keepId } from '../utils/id';
import { useSsrData } from '../seo/SsrDataContext';
import {
  canInteractWithChapter,
  hasPermanentBookAccess,
  hasSupportedConfiguredAccess
} from '../utils/readerAccess';

const CHARGE_MODE_LABELS = {
  word_charge: '按字收费',
  membership_only: '仅限会员',
  login_free: '登录免费',
  fixed_price: '整本购买'
} as const;

function canPurchaseFixedBook(status: ReaderBookProductStatus) {
  return hasSupportedConfiguredAccess(status)
    && status.chargeMode === 'fixed_price'
    && status.purchasable
    && !status.purchased
    && status.priceCoin !== null
    && (status.accessReason === 'book_purchase_required'
      || (status.accessReason === 'membership' && status.membershipEntitled));
}

function canPurchaseWordChapter(status: ReaderChapterAccess) {
  return hasSupportedConfiguredAccess(status)
    && status.chargeMode === 'word_charge'
    && status.purchasable
    && !status.bookPurchased
    && !status.chapterPurchased
    && status.chapterPrice !== null
    && (status.accessReason === 'chapter_purchase_required'
      || (status.accessReason === 'membership' && status.membershipEntitled));
}

function targetReaderLocation(bookId: string, chapterId: string, permanentPurchase = false) {
  return {
    pathname: `/read/${bookId}`,
    state: { chapterId, ...(permanentPurchase ? { permanentPurchase: true } : {}) }
  };
}

function HeartIcon({ filled = false }: { filled?: boolean }) {
  if (filled) {
    return (
      <svg viewBox="0 0 24 24" width="30" height="30" fill="currentColor" aria-hidden="true">
        <path d="M12 21s-7.5-4.6-10-9.3C.4 8.3 2 5 5.2 5c1.9 0 3.2 1 3.8 2.1.6.9.7.9 1 .9.3 0 .4 0 1-.9C11.6 6 12.9 5 14.8 5 18 5 19.6 8.3 18 11.7 15.5 16.4 12 21 12 21Z" />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 24 24" width="30" height="30" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M19.5 12.6 12 20l-7.5-7.4C2.2 10.3 2 6.7 4.6 4.9c2.1-1.4 4.8-.7 6.1 1.4L12 8.4l1.3-2.1c1.3-2.1 4-2.8 6.1-1.4 2.6 1.8 2.4 5.4.1 7.7Z" />
    </svg>
  );
}

function ShelfIcon() {
  return (
    <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M5 4.8h5.4c1 0 1.8.8 1.8 1.8v13c-.5-.8-1.3-1.2-2.3-1.2H5.8A1.8 1.8 0 0 1 4 16.6v-10a1.8 1.8 0 0 1 1-1.8Z" />
      <path d="M19 4.8h-5.4c-1 0-1.8.8-1.8 1.8v13c.5-.8 1.3-1.2 2.3-1.2h4.1a1.8 1.8 0 0 0 1.8-1.8v-10a1.8 1.8 0 0 0-1-1.8Z" />
      <path d="M8 8h1.5M15 8h1.5" />
    </svg>
  );
}

function BookIcon() {
  return (
    <svg viewBox="0 0 24 24" width="28" height="28" fill="none" aria-hidden="true">
      <path d="M5 4.5h5.6c1.1 0 2 .9 2 2V20c-.5-.8-1.4-1.3-2.4-1.3H5a1 1 0 0 1-1-1V5.5a1 1 0 0 1 1-1Z" fill="currentColor" />
      <path d="M19 4.5h-5.6c-1.1 0-2 .9-2 2V20c.5-.8 1.4-1.3 2.4-1.3H19a1 1 0 0 0 1-1V5.5a1 1 0 0 0-1-1Z" fill="currentColor" />
    </svg>
  );
}

function formatChapterOrdinal(chapter: ReaderChapterSummary, index: number) {
  const ordinal = chapter.chapterNo > 0 ? chapter.chapterNo : index + 1;
  return `第${ordinal}章`;
}

export function BookDetailPage() {
  const initial = useSsrData().bookDetail;
  const navigate = useNavigate();
  const location = useLocation();
  const { bookId } = useParams();
  const safeBookId = keepId(bookId);
  const { isAuthenticated, refreshWallet, sessionReady, token } = useReaderAuth();
  const [book, setBook] = useState<ReaderBookDetail | null>(initial?.book || null);
  const [chapters, setChapters] = useState<ReaderChapterSummary[]>(initial?.chapters || []);
  const [history, setHistory] = useState<ReaderReadingHistory | null>(null);
  const [loading, setLoading] = useState(!initial);
  const [error, setError] = useState('');
  const [shelfSaving, setShelfSaving] = useState(false);
  const [likeSaving, setLikeSaving] = useState(false);
  const [descriptionOpen, setDescriptionOpen] = useState(false);
  const [bookPurchaseSaving, setBookPurchaseSaving] = useState(false);
  const [purchaseError, setPurchaseError] = useState('');
  const mountedRef = useRef(true);
  const purchaseRequestSequence = useRef(0);
  const currentBookIdRef = useRef(safeBookId);
  const loadedBookIdRef = useRef(initial?.book.bookId || '');
  const skipInitialRequestRef = useRef(Boolean(initial && initial.book.bookId === safeBookId));

  useLayoutEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      purchaseRequestSequence.current += 1;
    };
  }, []);

  useLayoutEffect(() => {
    currentBookIdRef.current = safeBookId;
    purchaseRequestSequence.current += 1;
  }, [safeBookId]);

  useEffect(() => {
    setBookPurchaseSaving(false);
    setPurchaseError('');
  }, [safeBookId]);

  useEffect(() => {
    if (!sessionReady) {
      return undefined;
    }
    if (skipInitialRequestRef.current && !token) {
      skipInitialRequestRef.current = false;
      return undefined;
    }
    skipInitialRequestRef.current = false;
    let alive = true;
    async function load() {
      if (loadedBookIdRef.current !== safeBookId) {
        setLoading(true);
      }
      setError('');
      try {
        const [bookDetail, chapterRows] = await Promise.all([getBook(safeBookId), listBookChapters(safeBookId)]);
        if (!alive) {
          return;
        }
        loadedBookIdRef.current = bookDetail.bookId;
        setBook(bookDetail);
        setChapters(chapterRows);
      } catch (nextError) {
        if (alive) {
          setError(nextError instanceof Error ? nextError.message : '作品加载失败');
        }
      } finally {
        if (alive) {
          setLoading(false);
        }
      }
    }
    if (safeBookId) {
      load();
    }
    return () => {
      alive = false;
    };
  }, [safeBookId, sessionReady, token]);

  useEffect(() => {
    if (!sessionReady || !token || !safeBookId) {
      setHistory(null);
      return;
    }
    let alive = true;
    getBookHistory(safeBookId)
      .then((nextHistory) => {
        if (alive) {
          setHistory(nextHistory);
        }
      })
      .catch(() => {
        if (alive) {
          setHistory(null);
        }
      });
    return () => {
      alive = false;
    };
  }, [safeBookId, sessionReady, token]);

  const firstChapterId = chapters[0]?.chapterId || '';
  const continueChapterId = useMemo(
    () => history?.chapterId || book?.readingHistory?.chapterId || firstChapterId || book?.lastChapterId || '',
    [book?.lastChapterId, book?.readingHistory?.chapterId, firstChapterId, history?.chapterId]
  );
  const continueReaderLocation = continueChapterId ? targetReaderLocation(safeBookId, continueChapterId) : null;
  const bookDescription = book?.bookDesc?.trim() || '';
  const currentReaderLocation = location;
  const bookAccessSupported = book
    ? hasPermanentBookAccess(book.productStatus) || hasSupportedConfiguredAccess(book.productStatus)
    : false;
  const bookPurchaseAvailable = book ? canPurchaseFixedBook(book.productStatus) : false;

  const goToLogin = () => {
    navigate(`/auth/login?redirect=${encodeURIComponent(`/books/${safeBookId}`)}`);
  };

  const toggleLike = async () => {
    if (!isAuthenticated) {
      goToLogin();
      return;
    }
    if (!book || likeSaving) {
      return;
    }
    setLikeSaving(true);
    try {
      const nextLike = book.liked ? await unlikeBook(safeBookId) : await likeBook(safeBookId);
      setBook((current) => (current ? { ...current, liked: nextLike.liked, likeCount: nextLike.likeCount } : current));
    } finally {
      setLikeSaving(false);
    }
  };

  const addToShelf = async () => {
    if (!isAuthenticated) {
      goToLogin();
      return;
    }
    if (book?.inBookshelf) {
      return;
    }
    setShelfSaving(true);
    try {
      await addBookshelf(safeBookId);
      setBook((current) => (current ? { ...current, inBookshelf: true } : current));
    } finally {
      setShelfSaving(false);
    }
  };

  const purchaseBook = async () => {
    if (!book || bookPurchaseSaving || !canPurchaseFixedBook(book.productStatus)) {
      return;
    }
    if (!isAuthenticated) {
      goToLogin();
      return;
    }
    const expectedPrice = book.productStatus.priceCoin;
    if (expectedPrice === null) {
      return;
    }
    const formattedPrice = formatReaderLong(expectedPrice);
    if (!window.confirm(`确认使用 ${formattedPrice} 币永久购买《${book.bookName}》？`)) {
      return;
    }
    const purchaseBookId = book.bookId;
    const requestSequence = ++purchaseRequestSequence.current;
    const canPublish = () => mountedRef.current
      && purchaseRequestSequence.current === requestSequence
      && currentBookIdRef.current === purchaseBookId;
    setBookPurchaseSaving(true);
    setPurchaseError('');
    try {
      await buyBook(purchaseBookId, expectedPrice);
      if (!canPublish()) return;
      const [bookResult, chaptersResult] = await Promise.allSettled([
        getBook(purchaseBookId),
        listBookChapters(purchaseBookId),
        refreshWallet()
      ]);
      if (!canPublish()) return;
      if (bookResult.status === 'fulfilled') {
        setBook(bookResult.value);
      }
      if (chaptersResult.status === 'fulfilled') {
        setChapters(chaptersResult.value);
      }
      if (bookResult.status === 'rejected' || chaptersResult.status === 'rejected') {
        setPurchaseError('购买成功，状态刷新失败');
      }
    } catch (nextError) {
      if (!canPublish()) return;
      if (isApiErrorCode(nextError, QUOTE_CHANGED_CODE)) {
        const [bookResult, chaptersResult] = await Promise.allSettled([
          getBook(purchaseBookId),
          listBookChapters(purchaseBookId),
          refreshWallet()
        ]);
        if (!canPublish()) return;
        if (bookResult.status === 'fulfilled') setBook(bookResult.value);
        if (chaptersResult.status === 'fulfilled') setChapters(chaptersResult.value);
        setPurchaseError(bookResult.status === 'fulfilled'
          ? '价格已更新，请重新确认'
          : '价格已变化，最新报价刷新失败');
        return;
      }
      setPurchaseError(nextError instanceof Error ? nextError.message : '购买失败');
    } finally {
      if (canPublish()) setBookPurchaseSaving(false);
    }
  };

  const openPermanentChapterPurchase = (chapter: ReaderChapterSummary) => {
    if (!canPurchaseWordChapter(chapter.accessStatus)) {
      return;
    }
    const target = targetReaderLocation(safeBookId, chapter.chapterId, true);
    navigate(target.pathname, {
      state: {
        ...target.state,
        from: target,
        purchaseReturn: {
          pathname: location.pathname,
          search: location.search,
          hash: location.hash
        }
      }
    });
  };

  if (loading) {
    return <Loading />;
  }

  return (
    <AppShell
      back
      title={book?.bookName}
      onBack={() => navigate((location.state as { readerReturnToDetail?: boolean } | null)?.readerReturnToDetail ? -2 : -1)}
      footer={
        book ? (
          <div
            className={`detail-actionbar${book.inBookshelf ? ' is-in-shelf' : ''}`}
            style={{ gridTemplateColumns: book.inBookshelf ? '1fr 1fr 5fr' : '1fr 3fr 3fr' }}
          >
            <button
              type="button"
              className={`detail-actionbar__icon${book.liked ? ' is-active' : ''}`}
              disabled={likeSaving}
              onClick={toggleLike}
              aria-label={book.liked ? '取消点赞' : '点赞'}
            >
              <HeartIcon filled={book.liked} />
              <span>{book.likeCount ?? 0}</span>
            </button>
            <button
              type="button"
              className={`detail-actionbar__shelf${book.inBookshelf ? ' is-active' : ''}`}
              disabled={shelfSaving || book.inBookshelf}
              onClick={addToShelf}
              aria-label={book.inBookshelf ? '已在书架' : '加入书架'}
            >
              <ShelfIcon />
              {book.inBookshelf ? null : <span>加入书架</span>}
            </button>
            {continueReaderLocation && bookAccessSupported ? (
              <Link
                to={continueReaderLocation.pathname}
                state={{ ...continueReaderLocation.state, from: continueReaderLocation }}
                className="detail-actionbar__primary"
              >
                <BookIcon />
                <span>开始阅读</span>
              </Link>
            ) : null}
          </div>
        ) : undefined
      }
    >
      <main className="reader-page reader-page--narrow book-detail-page">
        {error ? <p className="notice notice--error">{error}</p> : null}
        {book ? (
          <>
            <section className="detail-head">
              <div className="detail-head__main">
                <div className="detail-head__title-row">
                  <h1 className="page-title detail-head__title">{book.bookName}</h1>
                  <span className="book-row__tag">{book.categoryName || '未分类'}</span>
                </div>
                {book.productStatus || book.subCategories?.length ? (
                  <div className="detail-head__meta">
                    <span className="book-row__sub-tag">
                      {isBookChargeMode(book.productStatus.chargeMode)
                        ? CHARGE_MODE_LABELS[book.productStatus.chargeMode]
                        : '收费状态不可用'}
                    </span>
                    {hasPermanentBookAccess(book.productStatus) ? (
                      <span className="book-row__sub-tag">已购买</span>
                    ) : !bookAccessSupported ? (
                      <span className="book-row__sub-tag">暂不可读</span>
                    ) : book.productStatus.membershipEntitled ? (
                      <span className="book-row__sub-tag">会员可读</span>
                    ) : book.productStatus.purchasable ? (
                      <span className="book-row__sub-tag">{formatReaderLong(book.productStatus.priceCoin ?? 0)}金币</span>
                    ) : null}
                    {book.subCategories?.length ? (
                      <span className="book-row__sub-tags detail-head__sub-tags" aria-label="副分类">
                        {book.subCategories.map((category) => (
                          <span className="book-row__sub-tag" key={category.categoryCode}>
                            {category.categoryName || category.categoryCode}
                          </span>
                        ))}
                      </span>
                    ) : null}
                  </div>
                ) : null}
                {bookPurchaseAvailable ? (
                  <div className="detail-purchase-actions">
                    <button
                      type="button"
                      className="detail-purchase-action"
                      disabled={bookPurchaseSaving}
                      onClick={purchaseBook}
                      aria-label={`永久购买整本 ${formatReaderLong(book.productStatus.priceCoin ?? 0)} 币`}
                    >
                      {bookPurchaseSaving ? '购买中' : '永久购买整本'}
                    </button>
                  </div>
                ) : book.productStatus.chargeMode === 'membership_only' && book.productStatus.accessReason === 'login_required' ? (
                  <div className="detail-purchase-actions">
                    <Link
                      className="detail-purchase-action"
                      to={`/auth/login?redirect=${encodeURIComponent(location.pathname)}`}
                      state={{ from: currentReaderLocation }}
                    >登录后开通会员</Link>
                  </div>
                ) : book.productStatus.chargeMode === 'membership_only' && book.productStatus.accessReason === 'membership_required' ? (
                  <div className="detail-purchase-actions">
                    <Link className="detail-purchase-action" to="/me">开通会员</Link>
                  </div>
                ) : null}
                {purchaseError ? <p className="notice notice--error" role="alert">{purchaseError}</p> : null}
                {bookDescription ? (
                  <div className="detail-description">
                    <p className="detail-description__text">{bookDescription}</p>
                    {bookDescription.length > 72 ? (
                      <button type="button" className="detail-description__more" onClick={() => setDescriptionOpen(true)}>
                        查看全部
                      </button>
                    ) : null}
                  </div>
                ) : null}
              </div>
            </section>

            <section className="content-section">
              <div className="detail-tabs" role="tablist">
                <button
                  type="button"
                  role="tab"
                  aria-selected="true"
                  className="detail-tab is-active"
                >
                  章节
                </button>
              </div>

              <div
                className="detail-chapter-list"
                style={{ alignSelf: 'start', alignContent: 'start', maxHeight: 'clamp(152px, 42dvh, 360px)' }}
              >
                {chapters.map((chapter, index) => {
                    const ordinal = formatChapterOrdinal(chapter, index);
                    const label = `${ordinal}：${chapter.chapterName}`;
                    const accessLabel = getChapterAccessLabel(chapter);
                    const target = targetReaderLocation(safeBookId, chapter.chapterId);
                    const interactive = canInteractWithChapter(chapter.accessStatus);
                    return (
                      <div className="chapter-row-wrap" key={chapter.chapterId}>
                        {interactive ? (
                          <Link
                            aria-label={`${label}${accessLabel ? `，${accessLabel}` : ''}`}
                            className="chapter-row"
                            to={target.pathname}
                            state={{ ...target.state, from: target }}
                          >
                            <span>{label}</span>
                            <ChapterAccessStatus chapter={chapter} />
                          </Link>
                        ) : (
                          <div
                            aria-disabled="true"
                            aria-label={`${label}${accessLabel ? `，${accessLabel}` : ''}`}
                            className="chapter-row"
                            role="link"
                          >
                            <span>{label}</span>
                            <ChapterAccessStatus chapter={chapter} />
                          </div>
                        )}
                        {canPurchaseWordChapter(chapter.accessStatus) ? (
                            <button
                              type="button"
                              className="chapter-row__purchase"
                              aria-label={`永久购买${ordinal}`}
                              onClick={() => openPermanentChapterPurchase(chapter)}
                            >
                              永久购买
                            </button>
                          ) : null}
                      </div>
                    );
                })}
              </div>
            </section>
            {descriptionOpen && bookDescription ? (
              <div className="detail-description-modal" role="presentation">
                <section className="detail-description-modal__card" role="dialog" aria-label="作品简介">
                  <div className="detail-description-modal__head">
                    <h2>作品简介</h2>
                    <button type="button" onClick={() => setDescriptionOpen(false)} aria-label="关闭简介">
                      ×
                    </button>
                  </div>
                  <p>{bookDescription}</p>
                </section>
              </div>
            ) : null}
          </>
        ) : null}
      </main>
    </AppShell>
  );
}
