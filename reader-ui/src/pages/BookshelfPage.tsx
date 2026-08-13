import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Card, Loading } from 'animal-island-ui';
import { listBookshelf, listHistory, removeBookshelf } from '../api/reader';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import type { ReaderBookshelf, ReaderReadingHistory } from '../types/reader';

interface BookshelfEntry {
  shelf: ReaderBookshelf;
  history?: ReaderReadingHistory;
}

function timeValue(value: string) {
  const time = value ? new Date(value.replace(' ', 'T')).getTime() : 0;
  return Number.isNaN(time) ? 0 : time;
}

function chapterText(entry: BookshelfEntry) {
  return entry.history?.chapterName || entry.shelf.lastChapterName || '未开始阅读';
}

function progressValue(value: ReaderReadingHistory['progressPercent'] | undefined) {
  if (value === undefined || value === null || value === '') {
    return null;
  }
  const numeric = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(numeric)) {
    return null;
  }
  return Math.min(100, Math.max(0, Math.round(numeric)));
}

function continueState(entry: BookshelfEntry) {
  const chapterId = entry.history?.chapterId || entry.shelf.lastChapterId;
  return chapterId ? { chapterId } : undefined;
}

function mergeShelfHistory(shelves: ReaderBookshelf[], histories: ReaderReadingHistory[]): BookshelfEntry[] {
  const historyByBookId = new Map(histories.map((history) => [history.bookId, history]));
  return shelves
    .map((shelf) => ({ shelf, history: historyByBookId.get(shelf.bookId) }))
    .sort((first, second) => timeValue(second.shelf.lastReadTime) - timeValue(first.shelf.lastReadTime));
}

function BookshelfRow({
  disabled = false,
  editing = false,
  entry,
  onToggle,
  selected = false
}: {
  disabled?: boolean;
  editing?: boolean;
  entry: BookshelfEntry;
  onToggle?: () => void;
  selected?: boolean;
}) {
  const progress = progressValue(entry.history?.progressPercent);
  const chapter = chapterText(entry);
  const metaLabel = [chapter, entry.shelf.lastReadTime].filter(Boolean).join(' ');

  if (editing) {
    return (
      <button
        className={`bookshelf-row bookshelf-row--editing${selected ? ' is-selected' : ''}`}
        data-testid="bookshelf-row"
        type="button"
        aria-pressed={selected}
        aria-label={`选择 ${entry.shelf.bookName}`}
        disabled={disabled}
        onClick={onToggle}
      >
        <span className="bookshelf-row__check" aria-hidden="true">{selected ? '✓' : ''}</span>
        <span className="bookshelf-row__main">
          <span className="bookshelf-row__title">{entry.shelf.bookName}</span>
          <span className="bookshelf-row__meta">{chapter}</span>
          {entry.shelf.lastReadTime ? <span className="bookshelf-row__time">{entry.shelf.lastReadTime}</span> : null}
          {progress !== null ? (
            <span className="bookshelf-progress" aria-label={`已读 ${progress}%`}>
              <span style={{ width: `${progress}%` }} />
            </span>
          ) : null}
          {progress !== null ? <span className="bookshelf-row__progress-text">已读 {progress}%</span> : null}
        </span>
      </button>
    );
  }

  return (
    <div className="bookshelf-row" data-testid="bookshelf-row">
      <Link className="bookshelf-row__main" to={`/books/${entry.shelf.bookId}`} aria-label={`${entry.shelf.bookName} ${metaLabel}`}>
        <h2 className="bookshelf-row__title">{entry.shelf.bookName}</h2>
        <p className="bookshelf-row__meta">{chapter}</p>
        {entry.shelf.lastReadTime ? <p className="bookshelf-row__time">{entry.shelf.lastReadTime}</p> : null}
        {progress !== null ? (
          <div className="bookshelf-progress" aria-label={`已读 ${progress}%`}>
            <span style={{ width: `${progress}%` }} />
          </div>
        ) : null}
        {progress !== null ? <p className="bookshelf-row__progress-text">已读 {progress}%</p> : null}
      </Link>
      <Link
        className="bookshelf-row__continue"
        to={`/read/${entry.shelf.bookId}`}
        state={continueState(entry)}
        aria-label={`继续阅读 ${entry.shelf.bookName}`}
      >
        继续
      </Link>
    </div>
  );
}

function BookshelfEditFooter({
  disabled,
  onCancel,
  onRemove,
  removing,
  controlsDisabled = false
}: {
  disabled: boolean;
  onCancel: () => void;
  onRemove: () => void;
  removing: boolean;
  controlsDisabled?: boolean;
}) {
  return (
    <div className="bookshelf-editbar">
      <button type="button" className="bookshelf-editbar__cancel" disabled={controlsDisabled} onClick={onCancel}>
        取消
      </button>
      <button type="button" className="bookshelf-editbar__remove" disabled={disabled || removing} onClick={onRemove}>
        {removing ? '移出中' : '移出书架'}
      </button>
    </div>
  );
}

export function BookshelfPage() {
  const navigate = useNavigate();
  const { isAuthenticated, sessionReady } = useReaderAuth();
  const [shelves, setShelves] = useState<ReaderBookshelf[]>([]);
  const [histories, setHistories] = useState<ReaderReadingHistory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [selectedBookIds, setSelectedBookIds] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    if (sessionReady && !isAuthenticated) {
      navigate('/auth/login?redirect=/shelf', { replace: true });
    }
  }, [isAuthenticated, navigate, sessionReady]);

  useEffect(() => {
    if (!sessionReady || !isAuthenticated) {
      return;
    }
    let alive = true;
    setLoading(true);
    setError('');
    Promise.all([listBookshelf(), listHistory()])
      .then(([nextShelves, nextHistories]) => {
        if (!alive) {
          return;
        }
        setShelves(nextShelves);
        setHistories(nextHistories);
      })
      .catch(() => {
        if (alive) {
          setError('书架加载失败');
        }
      })
      .finally(() => {
        if (alive) {
          setLoading(false);
        }
      });
    return () => {
      alive = false;
    };
  }, [isAuthenticated, sessionReady]);

  const entries = useMemo(() => mergeShelfHistory(shelves, histories), [histories, shelves]);
  const selectedCount = selectedBookIds.size;
  const allSelected = entries.length > 0 && selectedCount === entries.length;

  const exitEditMode = () => {
    if (removing) {
      return;
    }
    setEditing(false);
    setSelectedBookIds(new Set());
  };

  const toggleBook = (bookId: string) => {
    if (removing) {
      return;
    }
    setSelectedBookIds((current) => {
      const next = new Set(current);
      if (next.has(bookId)) {
        next.delete(bookId);
      } else {
        next.add(bookId);
      }
      return next;
    });
  };

  const toggleAll = () => {
    if (removing) {
      return;
    }
    setSelectedBookIds(allSelected ? new Set() : new Set(entries.map((entry) => entry.shelf.bookId)));
  };

  const removeSelected = async () => {
    if (!selectedBookIds.size || removing) {
      return;
    }
    const ids = [...selectedBookIds];
    setRemoving(true);
    setError('');
    try {
      const results = await Promise.all(ids.map((bookId) => removeBookshelf(bookId)));
      if (results.some((result) => result !== true)) {
        throw new Error('remove bookshelf failed');
      }
      setShelves((current) => current.filter((shelf) => !ids.includes(shelf.bookId)));
      exitEditMode();
    } catch {
      setError('移出书架失败');
    } finally {
      setRemoving(false);
    }
  };

  return (
    <AppShell
      active="shelf"
      title={editing ? `已选择 ${selectedCount} 本` : '书架'}
      rightSlot={
        entries.length > 0 ? (
          <button
            type="button"
            className="app-bar__link"
            disabled={editing && removing}
            onClick={editing ? exitEditMode : () => setEditing(true)}
          >
            {editing ? '完成' : '编辑'}
          </button>
        ) : null
      }
      footer={
        editing ? (
          <BookshelfEditFooter
            disabled={selectedCount === 0}
            removing={removing}
            controlsDisabled={removing}
            onCancel={exitEditMode}
            onRemove={removeSelected}
          />
        ) : undefined
      }
    >
      <main className="reader-page reader-page--narrow bookshelf-page">
        {error ? <p className="notice notice--error">{error}</p> : null}
        {loading ? <Loading /> : null}
        {!loading && entries.length > 0 ? (
          <>
            <div className="bookshelf-status">
              <span>{editing ? '选择要移出书架的作品' : '按最近阅读排序'}</span>
              {editing ? (
                <button type="button" className="bookshelf-status__action" disabled={removing} onClick={toggleAll}>
                  {allSelected ? '取消全选' : '全选'}
                </button>
              ) : (
                <span>共 {entries.length} 本</span>
              )}
            </div>
            <div className="bookshelf-list">
              {entries.map((entry) => (
                <BookshelfRow
                  key={entry.shelf.bookId}
                  disabled={removing}
                  editing={editing}
                  entry={entry}
                  selected={selectedBookIds.has(entry.shelf.bookId)}
                  onToggle={() => toggleBook(entry.shelf.bookId)}
                />
              ))}
            </div>
          </>
        ) : null}
        {!loading && !error && entries.length === 0 ? (
          <Card className="bookshelf-empty">
            <strong>书架还是空的</strong>
            <Link to="/books">去书库逛逛</Link>
          </Card>
        ) : null}
      </main>
    </AppShell>
  );
}
