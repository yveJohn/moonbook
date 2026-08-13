import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BookTextCard } from './BookTextCard';

const productStatus = {
  bookId: '9223372036854775807',
  chargeMode: 'login_free' as const,
  readable: true,
  accessReason: 'login_free' as const,
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

describe('BookTextCard', () => {
  it('renders name, category, word count and likes without author or featured note', () => {
    render(
      <BookTextCard
        book={{
          bookId: '9223372036854775807',
          bookName: '月下长书',
          authorName: '青山',
          bookDesc: '一段很长的简介',
          categoryCode: 'fantasy',
          categoryName: '玄幻',
          bookStatus: '1',
          wordCount: 120000,
          likeCount: 3200,
          subCategories: [
            { categoryCode: 'rebirth', categoryName: '重生', sort: 0 },
            { categoryCode: 'system', categoryName: '系统', sort: 1 }
          ],
          lastChapterId: '9223372036854775806',
          lastChapterName: '第一百章',
          lastChapterUpdateTime: '2026-06-27 10:00:00',
          featured: 1,
          featuredNote: '适合睡前读',
          productStatus
        }}
      />
    );

    expect(screen.getByText('月下长书')).toBeInTheDocument();
    expect(screen.getByText('玄幻')).toBeInTheDocument();
    expect(screen.getByText('重生')).toBeInTheDocument();
    expect(screen.getByText('系统')).toBeInTheDocument();
    expect(screen.getByText('12 万字')).toBeInTheDocument();
    expect(screen.getByText('3200 赞')).toBeInTheDocument();
    expect(screen.queryByText('青山')).toBeNull();
    expect(screen.queryByText('适合睡前读')).toBeNull();
    expect(screen.queryByText('第一百章')).toBeNull();
    expect(screen.queryByText('一段很长的简介')).toBeNull();
  });

  it('renders the book description when requested', () => {
    render(
      <BookTextCard
        showDescription
        book={{
          bookId: '9223372036854775807',
          bookName: '月下长书',
          authorName: '青山',
          bookDesc: '一段很长的简介',
          categoryCode: 'fantasy',
          categoryName: '玄幻',
          bookStatus: '1',
          wordCount: 120000,
          likeCount: 3200,
          lastChapterId: '9223372036854775806',
          lastChapterName: '第一百章',
          lastChapterUpdateTime: '2026-06-27 10:00:00',
          featured: 1,
          featuredNote: '适合睡前读',
          productStatus
        }}
      />
    );

    expect(screen.getByText('一段很长的简介')).toHaveClass('book-row__description');
  });
});
