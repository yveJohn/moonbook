import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { Card, Loading } from 'animal-island-ui';
import {
  buyBook,
  buyChapter,
  getChapter,
  getBookHistory,
  getPreference,
  listBookChapters,
  savePreference,
  updateHistory
} from '../api/reader';
import { isApiErrorCode, QUOTE_CHANGED_CODE } from '../api/http';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { ChapterDrawer } from '../components/ChapterDrawer';
import { ReaderLayout } from '../components/ReaderLayout';
import { ReaderPalettePanel } from '../components/ReaderPalettePanel';
import { ReaderSettingsPanel } from '../components/ReaderSettingsPanel';
import type {
  ReaderChapterAccess,
  ReaderChapterContent,
  ReaderChapterSummary,
  ReaderBookProductStatus,
  ReaderLocalPreference,
  ReaderLongValue
} from '../types/reader';
import { isBookChargeMode, isReaderAccessReason } from '../types/reader';
import { formatReaderLong } from '../utils/formatReaderLong';
import { keepId } from '../utils/id';
import {
  loadLocalReaderPreference,
  saveLocalReaderPreference,
  toLocalReaderPreference,
  toPreferencePayload
} from '../utils/readerPreference';
import { currentScrollTop, progressPercent, restorableScrollTop } from '../utils/readerProgress';
import { hasPermanentChapterAccess } from '../utils/readerAccess';

const pageTurnAnimationMs = 280;
const settingsCloseAnimationMs = 180;
const scrollAutoNextThresholdPx = 28;

function canPurchaseReaderBook(status: ReaderBookProductStatus) {
  return status.chargeMode === 'fixed_price'
    && isReaderAccessReason(status.accessReason)
    && status.purchasable
    && !status.purchased
    && status.priceCoin !== null
    && (status.accessReason === 'book_purchase_required'
      || (status.accessReason === 'membership' && status.membershipEntitled));
}

interface ReaderRouteState {
  chapterId?: string;
  permanentPurchase?: boolean;
  purchaseReturn?: unknown;
}

interface ReaderPurchaseReturn {
  pathname: string;
  search: string;
  hash: string;
}

interface ReaderAccessPrompt {
  kind: 'chapter' | 'book' | 'membership';
  chapter: ReaderChapterSummary;
  price: ReaderLongValue | null;
  quoteChanged?: boolean;
}

interface ReaderAccessTarget {
  bookId: string;
  chapterId: string;
  version: number;
}

function createPurchaseRequestId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `chapter-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function safePurchaseReturn(value: unknown, bookId: string): ReaderPurchaseReturn | null {
  if (!value || typeof value !== 'object') return null;
  const candidate = value as { pathname?: unknown; search?: unknown; hash?: unknown };
  if (candidate.pathname !== `/books/${bookId}`) return null;
  const search = typeof candidate.search === 'string' && (candidate.search === '' || candidate.search.startsWith('?'))
    ? candidate.search
    : '';
  const hash = typeof candidate.hash === 'string' && (candidate.hash === '' || candidate.hash.startsWith('#'))
    ? candidate.hash
    : '';
  return { pathname: candidate.pathname, search, hash };
}

function readerFontFamily(fontFamily: ReaderLocalPreference['fontFamily']) {
  if (fontFamily === 'serif') {
    return "'Noto Serif SC', 'Songti SC', SimSun, serif";
  }
  if (fontFamily === 'hei') {
    return "'Noto Sans SC', 'PingFang SC', 'Microsoft YaHei', sans-serif";
  }
  if (fontFamily === 'kai') {
    return "'Kaiti SC', KaiTi, STKaiti, serif";
  }
  return undefined;
}

function pageTapZone(clientX: number, clientY: number): 'prev' | 'center' | 'next' {
  const width = window.innerWidth || 1;
  const height = window.innerHeight || 1;
  const xRatio = clientX / width;
  const yRatio = clientY / height;
  if (xRatio >= 0.32 && xRatio <= 0.68 && yRatio >= 0.28 && yRatio <= 0.72) {
    return 'center';
  }
  return xRatio < 0.5 ? 'prev' : 'next';
}

function scrollMaxTop() {
  return Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
}

function isNearScrollTop() {
  return currentScrollTop() <= scrollAutoNextThresholdPx;
}

function isNearScrollBottom() {
  return scrollMaxTop() - currentScrollTop() <= scrollAutoNextThresholdPx;
}

function splitChapterParagraphs(content: string | null | undefined) {
  if (!content) {
    return [];
  }
  return content.split(/\n+/).map((line) => line.trim()).filter(Boolean);
}

export function ReaderPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { bookId } = useParams();
  const safeBookId = keepId(bookId);
  const routeState = location.state as ReaderRouteState | null;
  const routeChapterId = keepId(routeState?.chapterId);
  const routePermanentPurchase = routeState?.permanentPurchase === true;
  const purchaseReturn = useMemo(() => safePurchaseReturn(routeState?.purchaseReturn, safeBookId), [routeState?.purchaseReturn, safeBookId]);
  const { isAuthenticated, refreshWallet } = useReaderAuth();
  const [chapter, setChapter] = useState<ReaderChapterContent | null>(null);
  const [scrollChapters, setScrollChapters] = useState<ReaderChapterContent[]>([]);
  const [chapters, setChapters] = useState<ReaderChapterSummary[]>([]);
  const [preference, setPreference] = useState<ReaderLocalPreference>(() => loadLocalReaderPreference());
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsClosing, setSettingsClosing] = useState(false);
  const [controlsVisible, setControlsVisible] = useState(false);
  const [scrollControlsVisible, setScrollControlsVisible] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [accessPrompt, setAccessPrompt] = useState<ReaderAccessPrompt | null>(null);
  const [purchasePending, setPurchasePending] = useState(false);
  const [purchaseError, setPurchaseError] = useState('');
  const [accessNotice, setAccessNotice] = useState('');
  const purchasePendingRef = useRef(false);
  const purchaseRequestSequence = useRef(0);
  const accessTargetRef = useRef<ReaderAccessTarget>({ bookId: '', chapterId: '', version: 0 });
  const handledPermanentIntentKey = useRef('');
  const routePermanentPurchaseRef = useRef(routePermanentPurchase);
  routePermanentPurchaseRef.current = routePermanentPurchase;
  const mountedRef = useRef(true);
  const chapterRequestIdRef = useRef<string | null>(null);
  const promptConfirmRef = useRef<HTMLButtonElement | null>(null);
  const reportTimer = useRef<number | null>(null);
  const programmaticScrollAt = useRef(0);
  const pendingRestoreTop = useRef<number | null>(null);
  const pagePointerStart = useRef<{ x: number; y: number } | null>(null);
  const pagePointerHandled = useRef(false);
  const pagerViewportRef = useRef<HTMLDivElement | null>(null);
  const pagerTrackRef = useRef<HTMLDivElement | null>(null);
  const turnTimer = useRef<number | null>(null);
  const settingsCloseTimer = useRef<number | null>(null);
  const scrollAutoNextChapter = useRef<string | null>(null);
  const scrollAutoPrevChapter = useRef<string | null>(null);
  const chapterRequestSeq = useRef(0);
  const pendingPrependScrollHeight = useRef<number | null>(null);
  const touchStartY = useRef<number | null>(null);
  const [pageIndex, setPageIndex] = useState(0);
  const [pageCount, setPageCount] = useState(1);
  const [pageWidth, setPageWidth] = useState(0);
  const [turnDirection, setTurnDirection] = useState<'prev' | 'next' | null>(null);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      purchaseRequestSequence.current += 1;
    };
  }, []);

  useEffect(() => {
    if (!chapter?.chapterName || !chapter.bookName) {
      return;
    }
    const previousTitle = document.title;
    document.title = `${chapter.chapterName}-${chapter.bookName}`;
    return () => {
      document.title = previousTitle;
    };
  }, [chapter?.bookName, chapter?.chapterName]);

  const activateAccessTarget = useCallback((targetBookId: string, targetChapterId: string, forceNewVersion = false) => {
    const current = accessTargetRef.current;
    if (!forceNewVersion && current.bookId === targetBookId && current.chapterId === targetChapterId) return current;
    const nextTarget = { bookId: targetBookId, chapterId: targetChapterId, version: current.version + 1 };
    accessTargetRef.current = nextTarget;
    purchaseRequestSequence.current += 1;
    purchasePendingRef.current = false;
    chapterRequestIdRef.current = null;
    setPurchasePending(false);
    setPurchaseError('');
    setAccessNotice('');
    setAccessPrompt(null);
    return nextTarget;
  }, []);

  useLayoutEffect(() => {
    const previous = accessTargetRef.current;
    const nextRouteChapterId = routeChapterId || '';
    if (previous.bookId === safeBookId && previous.chapterId === nextRouteChapterId) return;
    activateAccessTarget(safeBookId, nextRouteChapterId);
    setChapter(null);
    setScrollChapters([]);
  }, [activateAccessTarget, routeChapterId, safeBookId]);

  const clearPermanentRouteState = useCallback((chapterId: string) => {
    navigate(location.pathname, {
      replace: true,
      state: { chapterId, ...(purchaseReturn ? { purchaseReturn } : {}) }
    });
  }, [location.pathname, navigate, purchaseReturn]);

  const cancelPurchase = useCallback(() => {
    if (purchasePendingRef.current) return;
    purchaseRequestSequence.current += 1;
    chapterRequestIdRef.current = null;
    setAccessPrompt(null);
    if (purchaseReturn) {
      navigate(-1);
      return;
    }
    navigate(`/books/${safeBookId}`, { replace: true });
  }, [navigate, purchaseReturn, safeBookId]);

  const inspectChapterAccess = useCallback((summary: ReaderChapterSummary, forcePermanentPurchase = false) => {
    const status: ReaderChapterAccess | undefined = summary.accessStatus;
    if (!status) {
      setError('章节权限信息缺失');
      return false;
    }
    if (hasPermanentChapterAccess(status)) {
      setAccessPrompt(null);
      setPurchaseError('');
      return true;
    }
    if (!isBookChargeMode(status.chargeMode)
      || !isReaderAccessReason(status.accessReason)
      || status.accessReason === 'unsupported_mode') {
      setAccessPrompt(null);
      setPurchaseError('');
      chapterRequestIdRef.current = null;
      setError('章节暂不可读');
      return false;
    }
    const canPurchaseChapter = status.chargeMode === 'word_charge'
      && status.purchasable
      && status.chapterPrice !== null
      && (status.accessReason === 'chapter_purchase_required'
        || (status.accessReason === 'membership' && status.membershipEntitled));
    if (forcePermanentPurchase && canPurchaseChapter) {
      setAccessPrompt({ kind: 'chapter', chapter: summary, price: status.chapterPrice });
      setPurchaseError('');
      chapterRequestIdRef.current = null;
      clearPermanentRouteState(summary.chapterId);
      return false;
    }
    if (status.readable) return true;
    if (status.accessReason === 'login_required') {
      const from = { pathname: `/read/${safeBookId}`, search: '', hash: '', state: { chapterId: summary.chapterId } };
      navigate(`/auth/login?redirect=${encodeURIComponent(`/read/${safeBookId}`)}`, { replace: true, state: { from } });
      return false;
    }
    if (status.accessReason === 'membership_required') {
      setAccessPrompt({ kind: 'membership', chapter: summary, price: null });
      return false;
    }
    if (status.accessReason === 'book_purchase_required') {
      if (status.chargeMode !== 'fixed_price'
        || !status.purchasable
        || !canPurchaseReaderBook(summary.productStatus)) {
        setError('本书暂不可购买');
        return false;
      }
      setAccessPrompt({ kind: 'book', chapter: summary, price: summary.productStatus.priceCoin });
      setPurchaseError('');
      return false;
    }
    if (status.accessReason === 'chapter_purchase_required') {
      if (status.chargeMode !== 'word_charge' || !status.purchasable || status.chapterPrice === null) {
        setError('章节暂不可购买');
        return false;
      }
      setAccessPrompt({ kind: 'chapter', chapter: summary, price: status.chapterPrice });
      setPurchaseError('');
      chapterRequestIdRef.current = null;
      return false;
    }
    setError('章节暂不可读');
    return false;
  }, [clearPermanentRouteState, navigate, safeBookId]);

  useEffect(() => {
    if (accessPrompt) promptConfirmRef.current?.focus();
  }, [accessPrompt]);

  useEffect(() => {
    if (!routePermanentPurchase || handledPermanentIntentKey.current === location.key || !routeChapterId) return;
    const summary = chapters.find((item) => item.chapterId === routeChapterId);
    if (!summary) return;
    handledPermanentIntentKey.current = location.key;
    activateAccessTarget(safeBookId, routeChapterId, true);
    inspectChapterAccess(summary, true);
  }, [activateAccessTarget, chapters, inspectChapterAccess, location.key, routeChapterId, routePermanentPurchase, safeBookId]);

  useEffect(() => {
    let alive = true;

    async function load() {
      setLoading(true);
      setError('');
      pendingRestoreTop.current = null;
      chapterRequestSeq.current += 1;
      try {
        const [chapterRows, readingHistory] = await Promise.all([
          listBookChapters(safeBookId),
          isAuthenticated ? getBookHistory(safeBookId).catch(() => null) : Promise.resolve(null)
        ]);
        if (!alive) return;
        const initialChapterId = routeChapterId || readingHistory?.chapterId || chapterRows[0]?.chapterId || '';
        if (!initialChapterId) {
          throw new Error('章节不可访问');
        }
        const initialSummary = chapterRows.find((item) => item.chapterId === initialChapterId);
        if (!initialSummary) {
          throw new Error('章节权限信息缺失');
        }
        activateAccessTarget(safeBookId, initialChapterId);
        if (!inspectChapterAccess(initialSummary, routePermanentPurchaseRef.current)) {
          if (alive) setChapters(chapterRows);
          return;
        }
        const nextChapter = await getChapter(initialChapterId);
        if (!alive) {
          return;
        }
        pendingRestoreTop.current = restorableScrollTop(nextChapter, readingHistory) ?? 0;
        setChapters(chapterRows);
        setChapter(nextChapter);
        setScrollChapters([nextChapter]);
      } catch (nextError) {
        if (alive) {
          pendingRestoreTop.current = null;
          setError(nextError instanceof Error ? nextError.message : '章节加载失败');
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
  }, [activateAccessTarget, inspectChapterAccess, isAuthenticated, routeChapterId, safeBookId]);

  useLayoutEffect(() => {
    if (loading || !chapter || pendingRestoreTop.current === null) {
      return;
    }
    if (preference.readingMode === 'page') {
      pendingRestoreTop.current = null;
      const frame = window.requestAnimationFrame(() => {
        programmaticScrollAt.current = Date.now();
        window.scrollTo({ top: 0, behavior: 'instant' });
      });
      return () => window.cancelAnimationFrame(frame);
    }
    const restoreTop = pendingRestoreTop.current;
    pendingRestoreTop.current = null;
    const frame = window.requestAnimationFrame(() => {
      programmaticScrollAt.current = Date.now();
      window.scrollTo({ top: restoreTop, behavior: 'instant' });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [chapter, loading, preference.readingMode]);

  useLayoutEffect(() => {
    if (pendingPrependScrollHeight.current === null) {
      return;
    }
    const previousScrollHeight = pendingPrependScrollHeight.current;
    pendingPrependScrollHeight.current = null;
    const frame = window.requestAnimationFrame(() => {
      const addedHeight = Math.max(0, document.documentElement.scrollHeight - previousScrollHeight);
      if (addedHeight <= 0) {
        return;
      }
      programmaticScrollAt.current = Date.now();
      window.scrollTo({ top: currentScrollTop() + addedHeight, behavior: 'instant' });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [scrollChapters]);

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }
    getPreference()
      .then((readerPreference) => {
        setPreference((currentPreference) => {
          const nextPreference = toLocalReaderPreference({ ...currentPreference, ...readerPreference });
          saveLocalReaderPreference(nextPreference);
          return nextPreference;
        });
      })
      .catch(() => undefined);
  }, [isAuthenticated]);

  const reportChapterHistory = useCallback(
    (targetChapter: ReaderChapterContent | null | undefined, positionValue: number, nextProgressPercent = progressPercent()) => {
      if (!isAuthenticated || !targetChapter?.bookId || !targetChapter.chapterId) {
        return;
      }
      updateHistory(targetChapter.bookId, {
        chapterId: targetChapter.chapterId,
        chapterNo: targetChapter.chapterNo,
        positionType: preference.readingMode,
        positionValue,
        progressPercent: nextProgressPercent
      }).catch(() => undefined);
    },
    [isAuthenticated, preference.readingMode]
  );

  const reportHistory = useCallback(
    (positionValue: number, nextProgressPercent = progressPercent()) => {
      reportChapterHistory(chapter, positionValue, nextProgressPercent);
    },
    [chapter, reportChapterHistory]
  );

  const currentPositionValue = useCallback(() => {
    return preference.readingMode === 'page' ? pageIndex : currentScrollTop();
  }, [pageIndex, preference.readingMode]);

  const currentReaderProgressPercent = useCallback(() => {
    if (preference.readingMode !== 'page') {
      return progressPercent();
    }
    if (pageCount <= 1) {
      return '0.00';
    }
    return ((pageIndex / (pageCount - 1)) * 100).toFixed(2);
  }, [pageCount, pageIndex, preference.readingMode]);

  const activeScrollChapters = useMemo(() => {
    if (scrollChapters.length) {
      return scrollChapters;
    }
    return chapter ? [chapter] : [];
  }, [chapter, scrollChapters]);
  const firstScrollChapter = activeScrollChapters[0] ?? null;
  const lastScrollChapter = activeScrollChapters[activeScrollChapters.length - 1] ?? null;

  const loadReaderChapter = useCallback(
    async (nextChapterId: string) => {
      if (!nextChapterId || nextChapterId === chapter?.chapterId) {
        return;
      }
      activateAccessTarget(safeBookId, nextChapterId);
      const requestSeq = chapterRequestSeq.current + 1;
      chapterRequestSeq.current = requestSeq;
      try {
        const summary = chapters.find((item) => item.chapterId === nextChapterId);
        if (!summary) {
          setError('章节权限信息缺失');
          return;
        }
        if (!inspectChapterAccess(summary)) return;
        const nextChapter = await getChapter(nextChapterId);
        if (chapterRequestSeq.current !== requestSeq) {
          return;
        }
        pendingRestoreTop.current = 0;
        setChapter(nextChapter);
        setScrollChapters([nextChapter]);
      } catch (nextError) {
        if (chapterRequestSeq.current === requestSeq) {
          setError(nextError instanceof Error ? nextError.message : '章节加载失败');
        }
      }
    },
    [activateAccessTarget, chapter?.chapterId, chapters, inspectChapterAccess, safeBookId]
  );

  const appendScrollChapter = useCallback(
    async (nextChapterId: string) => {
      if (!nextChapterId || activeScrollChapters.some((loadedChapter) => loadedChapter.chapterId === nextChapterId)) {
        return;
      }
      activateAccessTarget(safeBookId, nextChapterId);
      const requestSeq = chapterRequestSeq.current + 1;
      chapterRequestSeq.current = requestSeq;
      try {
        const summary = chapters.find((item) => item.chapterId === nextChapterId);
        if (!summary) {
          setError('章节权限信息缺失');
          return;
        }
        if (!inspectChapterAccess(summary)) return;
        const nextChapter = await getChapter(nextChapterId);
        if (chapterRequestSeq.current !== requestSeq) {
          return;
        }
        pendingRestoreTop.current = null;
        setScrollChapters((currentChapters) => {
          const baseChapters = currentChapters.length ? currentChapters : chapter ? [chapter] : [];
          if (baseChapters.some((loadedChapter) => loadedChapter.chapterId === nextChapter.chapterId)) {
            return baseChapters;
          }
          return [...baseChapters, nextChapter];
        });
        setChapter(nextChapter);
      } catch (nextError) {
        if (chapterRequestSeq.current === requestSeq) {
          setError(nextError instanceof Error ? nextError.message : '章节加载失败');
        }
      }
    },
    [activateAccessTarget, activeScrollChapters, chapter, chapters, inspectChapterAccess, safeBookId]
  );

  const prependScrollChapter = useCallback(
    async (prevChapterId: string) => {
      if (!prevChapterId || activeScrollChapters.some((loadedChapter) => loadedChapter.chapterId === prevChapterId)) {
        return;
      }
      activateAccessTarget(safeBookId, prevChapterId);
      const requestSeq = chapterRequestSeq.current + 1;
      chapterRequestSeq.current = requestSeq;
      const previousScrollHeight = document.documentElement.scrollHeight;
      try {
        const summary = chapters.find((item) => item.chapterId === prevChapterId);
        if (!summary) {
          setError('章节权限信息缺失');
          return;
        }
        if (!inspectChapterAccess(summary)) return;
        const prevChapter = await getChapter(prevChapterId);
        if (chapterRequestSeq.current !== requestSeq) {
          return;
        }
        pendingRestoreTop.current = null;
        pendingPrependScrollHeight.current = previousScrollHeight;
        setScrollChapters((currentChapters) => {
          const baseChapters = currentChapters.length ? currentChapters : chapter ? [chapter] : [];
          if (baseChapters.some((loadedChapter) => loadedChapter.chapterId === prevChapter.chapterId)) {
            pendingPrependScrollHeight.current = null;
            return baseChapters;
          }
          return [prevChapter, ...baseChapters];
        });
      } catch (nextError) {
        if (chapterRequestSeq.current === requestSeq) {
          setError(nextError instanceof Error ? nextError.message : '章节加载失败');
        }
      }
    },
    [activateAccessTarget, activeScrollChapters, chapter, chapters, inspectChapterAccess, safeBookId]
  );

  const requestNextScrollChapter = useCallback(
    (sourceChapter: ReaderChapterContent | null | undefined) => {
      if (
        !sourceChapter?.chapterId ||
        !sourceChapter.nextChapterId ||
        scrollAutoNextChapter.current === sourceChapter.chapterId
      ) {
        return false;
      }
      scrollAutoNextChapter.current = sourceChapter.chapterId;
      reportChapterHistory(sourceChapter, currentScrollTop());
      void appendScrollChapter(sourceChapter.nextChapterId);
      return true;
    },
    [appendScrollChapter, reportChapterHistory]
  );

  const requestPrevScrollChapter = useCallback(
    (sourceChapter: ReaderChapterContent | null | undefined) => {
      if (
        !sourceChapter?.chapterId ||
        !sourceChapter.prevChapterId ||
        scrollAutoPrevChapter.current === sourceChapter.chapterId
      ) {
        return false;
      }
      scrollAutoPrevChapter.current = sourceChapter.chapterId;
      reportChapterHistory(sourceChapter, currentScrollTop());
      void prependScrollChapter(sourceChapter.prevChapterId);
      return true;
    },
    [prependScrollChapter, reportChapterHistory]
  );

  const goChapter = useCallback(
    (nextChapterId: string) => {
      if (nextChapterId) {
        reportHistory(currentPositionValue(), currentReaderProgressPercent());
        void loadReaderChapter(nextChapterId);
      }
    },
    [currentPositionValue, currentReaderProgressPercent, loadReaderChapter, reportHistory]
  );

  useEffect(() => {
    const handleScroll = () => {
      if (preference.readingMode === 'page') {
        return;
      }
      if (Date.now() - programmaticScrollAt.current < 800) {
        return;
      }
      setScrollControlsVisible(false);
      setDrawerOpen(false);
      setPaletteOpen(false);
      if (settingsCloseTimer.current !== null) {
        window.clearTimeout(settingsCloseTimer.current);
        settingsCloseTimer.current = null;
      }
      setSettingsOpen(false);
      setSettingsClosing(false);
      if (
        lastScrollChapter?.chapterId &&
        lastScrollChapter.nextChapterId &&
        isNearScrollBottom()
      ) {
        if (requestNextScrollChapter(lastScrollChapter)) {
          return;
        }
      }
      if (
        firstScrollChapter?.chapterId &&
        firstScrollChapter.prevChapterId &&
        isNearScrollTop()
      ) {
        if (requestPrevScrollChapter(firstScrollChapter)) {
          return;
        }
      }
      if (reportTimer.current !== null) {
        return;
      }
      reportTimer.current = window.setTimeout(() => {
        reportTimer.current = null;
        reportHistory(currentScrollTop());
      }, 3500);
    };
    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', handleScroll);
      if (reportTimer.current !== null) {
        window.clearTimeout(reportTimer.current);
      }
    };
  }, [
    firstScrollChapter,
    lastScrollChapter,
    preference.readingMode,
    reportHistory,
    requestNextScrollChapter,
    requestPrevScrollChapter
  ]);

  useEffect(() => {
    const handleWheel = (event: WheelEvent) => {
      if (preference.readingMode === 'page') {
        return;
      }
      if (Date.now() - programmaticScrollAt.current < 800 || Math.abs(event.deltaY) < 1) {
        return;
      }
      if (event.deltaY > 0 && isNearScrollBottom()) {
        requestNextScrollChapter(lastScrollChapter);
        return;
      }
      if (event.deltaY < 0 && isNearScrollTop()) {
        requestPrevScrollChapter(firstScrollChapter);
      }
    };
    window.addEventListener('wheel', handleWheel, { passive: true });
    return () => window.removeEventListener('wheel', handleWheel);
  }, [
    firstScrollChapter,
    lastScrollChapter,
    preference.readingMode,
    requestNextScrollChapter,
    requestPrevScrollChapter
  ]);

  useEffect(() => {
    const handleTouchStart = (event: TouchEvent) => {
      touchStartY.current = event.touches[0]?.clientY ?? null;
    };
    const handleTouchMove = (event: TouchEvent) => {
      if (preference.readingMode === 'page' || touchStartY.current === null) {
        return;
      }
      if (Date.now() - programmaticScrollAt.current < 800) {
        return;
      }
      const currentTouchY = event.touches[0]?.clientY;
      if (typeof currentTouchY !== 'number') {
        return;
      }
      const deltaY = touchStartY.current - currentTouchY;
      if (Math.abs(deltaY) < 24) {
        return;
      }
      if (deltaY > 0 && isNearScrollBottom()) {
        requestNextScrollChapter(lastScrollChapter);
        touchStartY.current = currentTouchY;
        return;
      }
      if (deltaY < 0 && isNearScrollTop()) {
        requestPrevScrollChapter(firstScrollChapter);
        touchStartY.current = currentTouchY;
      }
    };
    window.addEventListener('touchstart', handleTouchStart, { passive: true });
    window.addEventListener('touchmove', handleTouchMove, { passive: true });
    return () => {
      window.removeEventListener('touchstart', handleTouchStart);
      window.removeEventListener('touchmove', handleTouchMove);
    };
  }, [
    firstScrollChapter,
    lastScrollChapter,
    preference.readingMode,
    requestNextScrollChapter,
    requestPrevScrollChapter
  ]);

  const goBookDetail = useCallback(() => {
    if (!chapter?.bookId) {
      return;
    }
    navigate(`/books/${chapter.bookId}`, { replace: true, state: { readerReturnToDetail: true } });
  }, [chapter?.bookId, navigate]);

  const clearSettingsCloseTimer = useCallback(() => {
    if (settingsCloseTimer.current !== null) {
      window.clearTimeout(settingsCloseTimer.current);
      settingsCloseTimer.current = null;
    }
  }, []);

  const closeSettingsImmediately = useCallback(() => {
    clearSettingsCloseTimer();
    setSettingsClosing(false);
    setSettingsOpen(false);
  }, [clearSettingsCloseTimer]);

  const closeSettingsPanel = useCallback(() => {
    clearSettingsCloseTimer();
    if (!settingsOpen) {
      setSettingsClosing(false);
      return;
    }
    setSettingsClosing(true);
    settingsCloseTimer.current = window.setTimeout(() => {
      settingsCloseTimer.current = null;
      setSettingsOpen(false);
      setSettingsClosing(false);
    }, settingsCloseAnimationMs);
  }, [clearSettingsCloseTimer, settingsOpen]);

  const openSettingsPanel = useCallback(() => {
    clearSettingsCloseTimer();
    setDrawerOpen(false);
    setPaletteOpen(false);
    setControlsVisible(true);
    setSettingsOpen(true);
    setSettingsClosing(false);
  }, [clearSettingsCloseTimer]);

  const closeReaderMenus = useCallback(() => {
    setDrawerOpen(false);
    setPaletteOpen(false);
    closeSettingsPanel();
  }, [closeSettingsPanel]);

  const toggleChapterDrawer = useCallback(() => {
    closeSettingsImmediately();
    setPaletteOpen(false);
    setDrawerOpen((value) => !value);
  }, [closeSettingsImmediately]);

  const togglePalettePanel = useCallback(() => {
    closeSettingsImmediately();
    setDrawerOpen(false);
    setControlsVisible(true);
    setPaletteOpen((value) => !value);
  }, [closeSettingsImmediately]);

  const toggleSettingsPanel = useCallback(() => {
    setDrawerOpen(false);
    setPaletteOpen(false);
    if (settingsOpen && !settingsClosing) {
      closeSettingsPanel();
      return;
    }
    openSettingsPanel();
  }, [closeSettingsPanel, openSettingsPanel, settingsClosing, settingsOpen]);

  const showPageTurn = useCallback((direction: 'prev' | 'next') => {
    setTurnDirection(direction);
    if (turnTimer.current !== null) {
      window.clearTimeout(turnTimer.current);
    }
    turnTimer.current = window.setTimeout(() => {
      turnTimer.current = null;
      setTurnDirection(null);
    }, pageTurnAnimationMs);
  }, []);

  const turnReaderPage = useCallback(
    (direction: 'prev' | 'next') => {
      if (direction === 'next') {
        if (pageIndex >= pageCount - 1) {
          goChapter(chapter?.nextChapterId || '');
          return;
        }
        setPageIndex((value) => Math.min(value + 1, pageCount - 1));
        showPageTurn('next');
        return;
      }

      if (pageIndex <= 0) {
        goChapter(chapter?.prevChapterId || '');
        return;
      }
      setPageIndex((value) => Math.max(value - 1, 0));
      showPageTurn('prev');
    },
    [chapter?.nextChapterId, chapter?.prevChapterId, goChapter, pageCount, pageIndex, showPageTurn]
  );

  const handlePagePointerDown = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    pagePointerStart.current = { x: event.clientX, y: event.clientY };
  }, []);

  const handlePageTap = useCallback(
    (zone: 'prev' | 'center' | 'next') => {
      if (drawerOpen || paletteOpen || settingsOpen) {
        closeReaderMenus();
        return;
      }
      if (zone === 'center') {
        setControlsVisible((value) => !value);
        return;
      }
      turnReaderPage(zone);
    },
    [closeReaderMenus, drawerOpen, paletteOpen, settingsOpen, turnReaderPage]
  );

  const handlePagePointerUp = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      const start = pagePointerStart.current;
      pagePointerStart.current = null;
      if (!start) {
        return;
      }
      const deltaX = event.clientX - start.x;
      const deltaY = event.clientY - start.y;
      if (Math.abs(deltaX) >= 50 && Math.abs(deltaX) > Math.abs(deltaY)) {
        pagePointerHandled.current = true;
        if (drawerOpen || paletteOpen || settingsOpen) {
          closeReaderMenus();
          return;
        }
        turnReaderPage(deltaX < 0 ? 'next' : 'prev');
        return;
      }
      if (Math.abs(deltaX) <= 10 && Math.abs(deltaY) <= 10) {
        pagePointerHandled.current = true;
        handlePageTap(pageTapZone(event.clientX, event.clientY));
      }
    },
    [closeReaderMenus, drawerOpen, handlePageTap, paletteOpen, settingsOpen, turnReaderPage]
  );

  const handlePageZoneClick = useCallback(
    (zone: 'prev' | 'center' | 'next') => {
      if (pagePointerHandled.current) {
        pagePointerHandled.current = false;
        return;
      }
      handlePageTap(zone);
    },
    [handlePageTap]
  );

  const measurePagedContent = useCallback(() => {
    const viewport = pagerViewportRef.current;
    const track = pagerTrackRef.current;
    if (!viewport || !track) {
      return;
    }
    const renderedPageWidth = viewport.getBoundingClientRect().width;
    const nextPageWidth = Math.max(renderedPageWidth || viewport.clientWidth, 0);
    if (nextPageWidth <= 0) {
      setPageWidth(0);
      setPageCount(1);
      setPageIndex(0);
      return;
    }
    const nextPageCount = Math.max(1, Math.round(track.scrollWidth / nextPageWidth));
    setPageWidth(nextPageWidth);
    setPageCount(nextPageCount);
    setPageIndex((value) => Math.min(value, nextPageCount - 1));
  }, []);

  useLayoutEffect(() => {
    setPageIndex(0);
    setPageCount(1);
    setTurnDirection(null);
  }, [
    chapter?.chapterId,
    preference.fontSize,
    preference.indentMode,
    preference.lineHeight,
    preference.marginSize,
    preference.readingMode
  ]);

  useLayoutEffect(() => {
    if (preference.readingMode !== 'page' || pageWidth <= 0) {
      return;
    }
    const viewport = pagerViewportRef.current;
    if (viewport) {
      viewport.scrollLeft = pageIndex * pageWidth;
    }
  }, [pageIndex, pageWidth, preference.readingMode]);

  useEffect(() => {
    scrollAutoNextChapter.current = null;
    scrollAutoPrevChapter.current = null;
    setControlsVisible(false);
    setDrawerOpen(false);
    setPaletteOpen(false);
    closeSettingsImmediately();
  }, [chapter?.chapterId, closeSettingsImmediately]);

  useEffect(() => {
    if (settingsOpen && !settingsClosing && preference.readingMode === 'page') {
      setControlsVisible(true);
    }
  }, [preference.readingMode, settingsClosing, settingsOpen]);

  useLayoutEffect(() => {
    if (preference.readingMode !== 'page' || loading || !chapter) {
      return;
    }
    const frame = window.requestAnimationFrame(measurePagedContent);
    window.addEventListener('resize', measurePagedContent);
    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined' && pagerViewportRef.current) {
      resizeObserver = new ResizeObserver(() => measurePagedContent());
      resizeObserver.observe(pagerViewportRef.current);
    }
    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener('resize', measurePagedContent);
      resizeObserver?.disconnect();
    };
  }, [
    chapter,
    chapter?.content,
    loading,
    measurePagedContent,
    pageWidth,
    preference.fontSize,
    preference.indentMode,
    preference.lineHeight,
    preference.marginSize,
    preference.readingMode
  ]);

  useEffect(() => {
    return () => {
      if (turnTimer.current !== null) {
        window.clearTimeout(turnTimer.current);
      }
      if (settingsCloseTimer.current !== null) {
        window.clearTimeout(settingsCloseTimer.current);
      }
    };
  }, []);

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && accessPrompt) {
        event.preventDefault();
        if (!purchasePendingRef.current) cancelPurchase();
        return;
      }
      const target = event.target as HTMLElement | null;
      if (target && ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'].includes(target.tagName)) {
        return;
      }
      if (event.key === 'Escape') {
        setDrawerOpen(false);
        setPaletteOpen(false);
        setSettingsOpen(false);
        return;
      }
      if (event.key === 'ArrowLeft') {
        if (preference.readingMode === 'page') {
          event.preventDefault();
          turnReaderPage('prev');
          return;
        }
        goChapter(chapter?.prevChapterId || '');
        return;
      }
      if (event.key === 'ArrowRight') {
        if (preference.readingMode === 'page') {
          event.preventDefault();
          turnReaderPage('next');
          return;
        }
        goChapter(chapter?.nextChapterId || '');
        return;
      }
      if (event.key === ' ' && preference.readingMode === 'scroll') {
        event.preventDefault();
        window.scrollBy({ top: window.innerHeight * 0.8, behavior: 'smooth' });
      }
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [accessPrompt, cancelPurchase, chapter?.nextChapterId, chapter?.prevChapterId, goChapter, preference.readingMode, turnReaderPage]);

  const updatePreference = (nextPreference: ReaderLocalPreference) => {
    const normalizedPreference = toLocalReaderPreference(nextPreference);
    setPreference(normalizedPreference);
    saveLocalReaderPreference(normalizedPreference);
    if (isAuthenticated) {
      savePreference(toPreferencePayload(normalizedPreference)).catch(() => undefined);
    }
  };

  const publishPurchasedChapter = async (target: ReaderChapterSummary, canPublish: () => boolean) => {
    const [chapterRowsResult] = await Promise.allSettled([
      listBookChapters(target.bookId),
      refreshWallet()
    ]);
    if (!canPublish()) return;
    if (chapterRowsResult.status === 'fulfilled') setChapters(chapterRowsResult.value);
    try {
      const nextChapter = await getChapter(target.chapterId);
      if (!canPublish()) return;
      pendingRestoreTop.current = 0;
      setChapter(nextChapter);
      setScrollChapters([nextChapter]);
      setAccessPrompt(null);
      if (chapterRowsResult.status === 'rejected') {
        setAccessNotice('章节状态刷新失败，正文已加载');
      }
    } catch (nextError) {
      if (canPublish()) setPurchaseError(nextError instanceof Error ? `购买完成，${nextError.message}` : '购买完成，正文加载失败');
    }
  };

  const confirmChapterPurchase = async () => {
    if (!accessPrompt || accessPrompt.kind !== 'chapter' || accessPrompt.price === null || purchasePendingRef.current) return;
    purchasePendingRef.current = true;
    setPurchasePending(true);
    setPurchaseError('');
    const requestId = chapterRequestIdRef.current ?? createPurchaseRequestId();
    chapterRequestIdRef.current = requestId;
    const requestSequence = ++purchaseRequestSequence.current;
    const targetSnapshot = accessTargetRef.current;
    const canPublish = () => mountedRef.current
      && purchaseRequestSequence.current === requestSequence
      && accessTargetRef.current.bookId === targetSnapshot.bookId
      && accessTargetRef.current.chapterId === targetSnapshot.chapterId
      && accessTargetRef.current.version === targetSnapshot.version;
    try {
      const result = await buyChapter(accessPrompt.chapter.chapterId, accessPrompt.price, requestId);
      if (!canPublish()) return;
      if (result.purchaseStatus === 'quote_changed') {
        chapterRequestIdRef.current = null;
        setAccessPrompt({
          ...accessPrompt,
          price: result.quote.priceCoin,
          quoteChanged: true
        });
        return;
      }
      chapterRequestIdRef.current = null;
      await publishPurchasedChapter(accessPrompt.chapter, canPublish);
    } catch (nextError) {
      if (canPublish()) setPurchaseError(nextError instanceof Error ? nextError.message : '章节购买失败');
    } finally {
      if (canPublish()) {
        purchasePendingRef.current = false;
        setPurchasePending(false);
      }
    }
  };

  const confirmBookPurchase = async () => {
    if (!accessPrompt || accessPrompt.kind !== 'book' || purchasePendingRef.current) return;
    purchasePendingRef.current = true;
    setPurchasePending(true);
    setPurchaseError('');
    const requestSequence = ++purchaseRequestSequence.current;
    const targetSnapshot = accessTargetRef.current;
    const canPublish = () => mountedRef.current
      && purchaseRequestSequence.current === requestSequence
      && accessTargetRef.current.bookId === targetSnapshot.bookId
      && accessTargetRef.current.chapterId === targetSnapshot.chapterId
      && accessTargetRef.current.version === targetSnapshot.version;
    try {
      if (accessPrompt.price === null) return;
      await buyBook(accessPrompt.chapter.bookId, accessPrompt.price);
      if (!canPublish()) return;
      await publishPurchasedChapter(accessPrompt.chapter, canPublish);
    } catch (nextError) {
      if (!canPublish()) return;
      if (isApiErrorCode(nextError, QUOTE_CHANGED_CODE)) {
        const [chapterRowsResult] = await Promise.allSettled([
          listBookChapters(accessPrompt.chapter.bookId),
          refreshWallet()
        ]);
        if (!canPublish()) return;
        if (chapterRowsResult.status === 'fulfilled') {
          setChapters(chapterRowsResult.value);
          const refreshedChapter = chapterRowsResult.value.find(
            (row) => row.chapterId === accessPrompt.chapter.chapterId
          );
          const refreshedPrice = refreshedChapter?.productStatus.priceCoin;
          if (refreshedChapter
            && refreshedChapter.accessStatus.chargeMode === 'fixed_price'
            && refreshedChapter.accessStatus.accessReason === 'book_purchase_required'
            && refreshedChapter.accessStatus.purchasable
            && canPurchaseReaderBook(refreshedChapter.productStatus)
            && refreshedPrice !== null
            && refreshedPrice !== undefined) {
            setAccessPrompt({
              kind: 'book',
              chapter: refreshedChapter,
              price: refreshedPrice,
              quoteChanged: true
            });
            return;
          }
        }
        setAccessPrompt(null);
        setChapter(null);
        setScrollChapters([]);
        setError(chapterRowsResult.status === 'fulfilled'
          ? '整书购买状态已变化，请重新选择'
          : '价格已变化，最新报价刷新失败');
        return;
      }
      setPurchaseError(nextError instanceof Error ? nextError.message : '整书购买失败');
    } finally {
      if (canPublish()) {
        purchasePendingRef.current = false;
        setPurchasePending(false);
      }
    }
  };

  const paragraphs = useMemo(() => {
    return splitChapterParagraphs(chapter?.content);
  }, [chapter?.content]);

  const pagerTrackStyle = useMemo<CSSProperties>(
    () => ({
      columnWidth: pageWidth > 0 ? `${pageWidth}px` : undefined,
      fontFamily: readerFontFamily(preference.fontFamily),
      fontSize: `${preference.fontSize}px`,
      lineHeight: preference.lineHeight
    }),
    [pageWidth, preference.fontFamily, preference.fontSize, preference.lineHeight]
  );
  const copyStyle = useMemo<CSSProperties>(
    () => ({
      fontFamily: readerFontFamily(preference.fontFamily),
      fontSize: `${preference.fontSize}px`,
      lineHeight: preference.lineHeight
    }),
    [preference.fontFamily, preference.fontSize, preference.lineHeight]
  );

  const readerRootClass = [
    'reader-theme',
    `reader-theme--${preference.theme}`,
    preference.readingMode === 'page' ? 'is-paged' : '',
    controlsVisible ? 'has-reader-chrome' : ''
  ].filter(Boolean).join(' ');
  const pagerTrackClass = preference.indentMode === 'indent' ? 'reader-pager__track has-indent' : 'reader-pager__track';
  const copyClass = preference.indentMode === 'indent' ? 'reader-copy has-indent' : 'reader-copy';
  const settingsPopoverClass = [
    'reader-settings-popover',
    'reader-settings-popover--dock',
    settingsClosing ? 'is-closing' : ''
  ].filter(Boolean).join(' ');
  const palettePopoverClass = [
    'reader-palette-popover',
    'reader-palette-popover--dock'
  ].filter(Boolean).join(' ');
  const activeTool = drawerOpen
    ? 'chapters'
    : paletteOpen
      ? 'palette'
      : settingsOpen && !settingsClosing
        ? 'settings'
        : null;

  if (loading) {
    return <Loading />;
  }

  if (accessPrompt) {
    const formattedPrice = accessPrompt.price === null ? '' : formatReaderLong(accessPrompt.price);
    return (
      <main className="reader-access-page">
        <section
          className="reader-access-panel"
          role="dialog"
          aria-modal="true"
          aria-labelledby="reader-access-title"
          aria-describedby="reader-access-description"
        >
          <h1 id="reader-access-title">
            {accessPrompt.kind === 'membership' ? '会员章节' : accessPrompt.kind === 'book' ? '购买整书后阅读' : '购买本章后阅读'}
          </h1>
          <p id="reader-access-description">
            {accessPrompt.kind === 'membership'
              ? '该章节需要有效会员权益。'
              : accessPrompt.kind === 'book'
                ? `本书永久阅读价格为 ${formattedPrice} 币。`
                : `本章永久阅读价格为 ${formattedPrice} 币。`}
          </p>
          {accessPrompt.quoteChanged ? <p className="notice notice--warning" role="status">价格已更新，请重新确认</p> : null}
          {purchaseError ? <p className="notice notice--error" role="alert">{purchaseError}</p> : null}
          <div className="reader-access-panel__actions">
            {accessPrompt.kind === 'membership' ? (
              <button ref={promptConfirmRef} type="button" className="reader-access-panel__primary" onClick={() => navigate('/me', { state: { focus: 'membership' } })}>
                开通会员
              </button>
            ) : accessPrompt.kind === 'book' ? (
              <button ref={promptConfirmRef} type="button" className="reader-access-panel__primary" disabled={purchasePending} onClick={() => { void confirmBookPurchase(); }}>
                {`确认整书购买 ${formattedPrice} 币`}
              </button>
            ) : (
              <button ref={promptConfirmRef} type="button" className="reader-access-panel__primary" disabled={purchasePending} onClick={() => { void confirmChapterPurchase(); }}>
                {`确认支付 ${formattedPrice} 币`}
              </button>
            )}
            <button type="button" className="reader-access-panel__cancel" disabled={purchasePending} onClick={cancelPurchase}>取消</button>
          </div>
        </section>
      </main>
    );
  }

  if (error || !chapter) {
    return (
      <main className="reader-page reader-page--narrow">
        <Card className="empty-card">{error || '章节不可访问'}</Card>
      </main>
    );
  }

  return (
    <div className={readerRootClass}>
      {accessNotice ? <div className="reader-access-notice" role="status">{accessNotice}</div> : null}
      {preference.readingMode === 'page' ? (
        <div className="reader-page-chapter">{chapter.chapterName}</div>
      ) : null}
      <ReaderLayout
        activeTool={activeTool}
        controlsVisible={preference.readingMode === 'page' ? controlsVisible : scrollControlsVisible}
        mode={preference.readingMode}
        title={chapter.bookName}
        onBackToDetail={goBookDetail}
        onContentClick={preference.readingMode === 'scroll' ? () => setScrollControlsVisible(true) : undefined}
        onOpenChapters={toggleChapterDrawer}
        onOpenPalette={togglePalettePanel}
        onOpenSettings={toggleSettingsPanel}
      >
        {preference.readingMode === 'page' ? (
          <>
            <h1>{chapter.chapterName}</h1>
            <div className={turnDirection ? `reader-pager is-turning-${turnDirection}` : 'reader-pager'}>
              <div className="reader-pager__viewport" data-margin-size={preference.marginSize}>
                <div className="reader-pager__clip" ref={pagerViewportRef}>
                  <div className={pagerTrackClass} ref={pagerTrackRef} style={pagerTrackStyle}>
                    {paragraphs.map((paragraph, index) => (
                      <p className="reader-paragraph" key={`${chapter.chapterId}-${index}`}>{paragraph}</p>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </>
        ) : (
          activeScrollChapters.map((scrollChapter) => (
            <section className="reader-scroll-chapter" key={scrollChapter.chapterId}>
              <h1>{scrollChapter.chapterName}</h1>
              <div className={copyClass} style={copyStyle}>
                {splitChapterParagraphs(scrollChapter.content).map((paragraph, index) => (
                  <p className="reader-paragraph" key={`${scrollChapter.chapterId}-${index}`}>{paragraph}</p>
                ))}
              </div>
            </section>
          ))
        )}
      </ReaderLayout>
      {preference.readingMode === 'page' ? (
        <div className="reader-page-count" aria-label="阅读页码">
          {pageIndex + 1} / {pageCount}
        </div>
      ) : null}
      {preference.readingMode === 'page' ? (
        <div
          className="reader-page-turner"
          onPointerDown={handlePagePointerDown}
          onPointerUp={handlePagePointerUp}
        >
          <button type="button" className="reader-page-turner__zone reader-page-turner__zone--left" aria-label="阅读左侧区域" onClick={() => handlePageZoneClick('prev')} />
          <button type="button" className="reader-page-turner__zone reader-page-turner__zone--center" aria-label="阅读中部区域" onClick={() => handlePageZoneClick('center')} />
          <button type="button" className="reader-page-turner__zone reader-page-turner__zone--right" aria-label="阅读右侧区域" onClick={() => handlePageZoneClick('next')} />
        </div>
      ) : null}
      <ChapterDrawer
        chapters={chapters}
        currentChapterId={chapter.chapterId}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onSelect={(nextChapterId) => {
          setDrawerOpen(false);
          goChapter(nextChapterId);
        }}
      />
      {settingsOpen ? (
        <div className={settingsPopoverClass}>
          <ReaderSettingsPanel
            preference={preference}
            onChange={updatePreference}
          />
        </div>
      ) : null}
      {paletteOpen ? (
        <div className={palettePopoverClass}>
          <ReaderPalettePanel
            preference={preference}
            onChange={updatePreference}
          />
        </div>
      ) : null}
    </div>
  );
}
