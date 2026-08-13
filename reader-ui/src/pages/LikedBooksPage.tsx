import { useCallback, useEffect, useRef, useState } from 'react';
import { ChevronRight, Heart } from 'lucide-react';
import { Link, useNavigate } from 'react-router-dom';
import { listLikedBooks } from '../api/reader';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import type { ReaderLikedBook } from '../types/reader';
import { formatReaderLong } from '../utils/formatReaderLong';

const LIKED_BOOKS_LOAD_ERROR = '赞过的图书加载失败';

function likedTime(value: string) {
  const normalized = value.trim();
  if (!normalized) return { label: '时间待确认' };
  const date = new Date(normalized.replace(' ', 'T'));
  if (Number.isNaN(date.getTime())) return { label: '时间待确认' };
  return {
    dateTime: date.toISOString(),
    label: date.toLocaleString('zh-CN', { hour12: false })
  };
}

function LikedBookSkeleton() {
  return (
    <div className="liked-book-row liked-book-row--skeleton" data-testid="liked-book-skeleton" aria-hidden="true">
      <span className="liked-book-skeleton__icon" />
      <span className="liked-book-row__body">
        <span className="liked-book-skeleton__title" />
        <span className="liked-book-skeleton__description" />
        <span className="liked-book-skeleton__meta" />
      </span>
      <span className="liked-book-skeleton__chevron" />
    </div>
  );
}

function LikedBookRow({ book }: { book: ReaderLikedBook }) {
  const authorName = book.authorName?.trim() || '佚名';
  const categoryName = book.categoryName?.trim() || '未分类';
  const description = book.bookDesc?.trim();
  const time = likedTime(book.likedAt);

  return (
    <Link
      className="liked-book-row"
      data-testid="liked-book-row"
      to={`/books/${book.bookId}`}
    >
      <span className="liked-book-row__heart" aria-hidden="true">
        <Heart size={18} strokeWidth={2} fill="currentColor" />
      </span>
      <span className="liked-book-row__body">
        <span className="liked-book-row__heading">
          <strong className="liked-book-row__title">{book.bookName}</strong>
          <span className="liked-book-row__author">{authorName}</span>
        </span>
        {description ? <span className="liked-book-row__description">{description}</span> : null}
        <span className="liked-book-row__meta">
          <span>{categoryName}</span>
          <span>{formatReaderLong(book.wordCount)} 字</span>
          <span className="liked-book-row__likes">
            <Heart size={13} strokeWidth={2} aria-hidden="true" />
            {formatReaderLong(book.likeCount)}
          </span>
          <time className="liked-book-row__time" dateTime={time.dateTime}>赞于 {time.label}</time>
        </span>
      </span>
      <ChevronRight className="liked-book-row__chevron" size={20} strokeWidth={2} aria-hidden="true" />
    </Link>
  );
}

export function LikedBooksPage() {
  const navigate = useNavigate();
  const { sessionReady, token } = useReaderAuth();
  const [books, setBooks] = useState<ReaderLikedBook[]>([]);
  const [booksIdentity, setBooksIdentity] = useState(token);
  const [loading, setLoading] = useState(Boolean(token));
  const [error, setError] = useState('');
  const mountedRef = useRef(true);
  const requestSequence = useRef(0);
  const identityRef = useRef(token);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestSequence.current += 1;
    };
  }, []);

  const loadBooks = useCallback(async (identity: string) => {
    const sequence = ++requestSequence.current;
    const canPublish = () => mountedRef.current
      && requestSequence.current === sequence
      && identityRef.current === identity;
    setLoading(true);
    setError('');
    try {
      const nextBooks = await listLikedBooks();
      if (canPublish()) {
        setBooks(nextBooks);
        setBooksIdentity(identity);
      }
    } catch {
      if (canPublish()) {
        setError(LIKED_BOOKS_LOAD_ERROR);
      }
    } finally {
      if (canPublish()) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (!sessionReady) return;
    const identity = token;
    identityRef.current = identity;
    requestSequence.current += 1;
    setBooks([]);
    setBooksIdentity(identity);
    setError('');
    if (!identity) {
      setLoading(false);
      navigate('/auth/login?redirect=/me/likes', { replace: true });
      return;
    }
    setLoading(true);
    void loadBooks(identity);
  }, [loadBooks, navigate, sessionReady, token]);

  const identityChanging = identityRef.current !== token;
  const visibleBooks = booksIdentity === token ? books : [];
  const visibleLoading = Boolean(token) && (identityChanging || loading);
  const visibleError = identityChanging ? '' : error;

  return (
    <AppShell active="me" title="赞过" back onBack={() => navigate('/me')}>
      <main className="reader-page reader-page--narrow liked-books-page">
        {visibleLoading ? (
          <div className="liked-books-list" role="status" aria-live="polite" aria-label="赞过的图书加载中">
            <span className="liked-books-sr-only">赞过的图书加载中</span>
            <LikedBookSkeleton />
            <LikedBookSkeleton />
            <LikedBookSkeleton />
          </div>
        ) : null}

        {!visibleLoading && visibleError ? (
          <div className="liked-books-state liked-books-state--error" role="alert">
            <p>{visibleError}</p>
            <button type="button" onClick={() => { void loadBooks(token); }}>重新加载</button>
          </div>
        ) : null}

        {!visibleLoading && !visibleError && visibleBooks.length === 0 ? (
          <p className="liked-books-state">暂无赞过的图书</p>
        ) : null}

        {!visibleLoading && !visibleError && visibleBooks.length > 0 ? (
          <div className="liked-books-list">
            {visibleBooks.map((book) => <LikedBookRow key={book.likeId} book={book} />)}
          </div>
        ) : null}
      </main>
    </AppShell>
  );
}
