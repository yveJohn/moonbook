import { Link } from 'react-router-dom';
import type { ReaderBookCatalogItem, ReaderBookSummary } from '../types/reader';

type BookTextCardBook = ReaderBookCatalogItem | ReaderBookSummary;

interface BookTextCardProps {
  book: BookTextCardBook;
  /** Kept for API compatibility; the whole row is now tappable. */
  actionLabel?: string;
  showDescription?: boolean;
  to?: string;
  onAction?: () => void;
}

function formatCount(value: number, unit: string) {
  if (!value) {
    return `0 ${unit}`;
  }
  if (value >= 10000) {
    return `${Math.round(value / 1000) / 10} 万${unit}`;
  }
  return `${value} ${unit}`;
}

export function BookTextCard({ book, showDescription = false, to, onAction }: BookTextCardProps) {
  const description = 'bookDesc' in book ? book.bookDesc?.trim() : undefined;
  const body = (
    <>
      <div className="book-row__body">
        <div className="book-row__heading">
          <h3 className="book-row__title">{book.bookName}</h3>
        </div>
        {showDescription && description ? <p className="book-row__description">{description}</p> : null}
        <div className="book-row__meta">
          <span className="book-row__tag">{book.categoryName || '未分类'}</span>
          <span>{formatCount(book.wordCount, '字')}</span>
          <span className="book-row__likes">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" aria-hidden="true">
              <path d="M12 21s-7.5-4.6-10-9.3C.4 8.3 2 5 5.2 5c1.9 0 3.2 1 3.8 2.1.6.9.7.9 1 .9.3 0 .4 0 1-.9C11.6 6 12.9 5 14.8 5 18 5 19.6 8.3 18 11.7 15.5 16.4 12 21 12 21Z" />
            </svg>
            {formatCount(book.likeCount || 0, '赞')}
          </span>
        </div>
        {book.subCategories?.length ? (
          <div className="book-row__sub-tags" aria-label="副分类">
            {book.subCategories.map((category) => (
              <span className="book-row__sub-tag" key={category.categoryCode}>
                {category.categoryName || category.categoryCode}
              </span>
            ))}
          </div>
        ) : null}
      </div>
      <span className="book-row__chevron" aria-hidden="true">
        ›
      </span>
    </>
  );

  if (to) {
    return (
      <Link className="book-row" to={to}>
        {body}
      </Link>
    );
  }

  return (
    <button className="book-row" type="button" onClick={onAction}>
      {body}
    </button>
  );
}
