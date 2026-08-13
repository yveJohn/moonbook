import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderChapterSummary } from '../types/reader';
import { ChapterDrawer } from './ChapterDrawer';

vi.mock('animal-island-ui', () => ({
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => <section className={className}>{children}</section>
}));

function chapter(chapterNo: number, access: Partial<ReaderChapterSummary['accessStatus']>): ReaderChapterSummary {
  const chapterId = `922337203685477580${chapterNo}`;
  const productStatus: ReaderChapterSummary['productStatus'] = {
    bookId: '9223372036854775800', chargeMode: 'word_charge', readable: false,
    accessReason: 'chapter_purchase_required', entitled: false, purchased: false,
    membershipEntitled: false, purchasable: false, productId: null, productName: null,
    priceCoin: null, saleStatus: null, product: null
  };
  return {
    chapterId, bookId: productStatus.bookId, chapterNo, chapterName: `章节${chapterNo}`,
    wordCount: 1000, updateTime: '', productStatus,
    accessStatus: {
      bookId: productStatus.bookId, chapterId, chargeMode: 'word_charge', readable: false,
      accessReason: 'chapter_purchase_required', membershipEntitled: false, bookPurchased: false,
      chapterPurchased: false, purchasable: false, chapterWordCount: 1000, pricingWordUnit: 1000,
      pricingCoinUnit: 1, chapterPrice: null, ...access
    }
  };
}

describe('ChapterDrawer access states', () => {
  afterEach(cleanup);

  it('shows the same free, price, purchased and membership labels as the detail directory', () => {
    const onSelect = vi.fn();
    render(
      <ChapterDrawer
        open
        currentChapterId="9223372036854775801"
        onClose={vi.fn()}
        onSelect={onSelect}
        chapters={[
          chapter(1, { readable: true, accessReason: 'free_chapter', chapterPrice: 0 }),
          chapter(2, { purchasable: true, chapterPrice: '9223372036854775807' }),
          chapter(3, { readable: true, accessReason: 'chapter_owned', chapterPurchased: true }),
          chapter(4, { readable: true, accessReason: 'membership', membershipEntitled: true, purchasable: true, chapterPrice: 3 })
        ]}
      />
    );

    expect(screen.getByText('免费')).toBeInTheDocument();
    expect(screen.getByText('9,223,372,036,854,775,807 币')).toBeInTheDocument();
    expect(screen.getByText('已购买')).toBeInTheDocument();
    expect(screen.getByText('会员可读')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '第1章：章节1，免费' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '第2章：章节2，9,223,372,036,854,775,807 币' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '第3章：章节3，已购买' })).toBeInTheDocument();
    const membershipChapter = screen.getByRole('button', { name: '第4章：章节4，会员可读' });
    fireEvent.click(membershipChapter);
    expect(onSelect).toHaveBeenCalledWith('9223372036854775804');
  });

  it('shows unsupported access as temporarily unreadable without exposing a price', () => {
    const onSelect = vi.fn();
    render(
      <ChapterDrawer
        open
        currentChapterId="9223372036854775801"
        onClose={vi.fn()}
        onSelect={onSelect}
        chapters={[chapter(1, {
          accessReason: 'unsupported_mode',
          membershipEntitled: true,
          purchasable: true,
          chapterPrice: '9223372036854775807'
        })]}
      />
    );

    expect(screen.getByText('暂不可读')).toBeInTheDocument();
    expect(screen.queryByText('9,223,372,036,854,775,807 币')).not.toBeInTheDocument();
    const item = screen.getByRole('button', { name: /暂不可读/ });
    expect(item).toBeDisabled();
    fireEvent.click(item);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('fails an unknown access reason closed instead of inferring free access', () => {
    const onSelect = vi.fn();
    render(
      <ChapterDrawer
        open
        currentChapterId="9223372036854775801"
        onClose={vi.fn()}
        onSelect={onSelect}
        chapters={[chapter(1, {
          accessReason: 'future_reason',
          chargeMode: 'login_free',
          readable: false
        })]}
      />
    );

    expect(screen.getByText('暂不可读')).toBeInTheDocument();
    expect(screen.queryByText('免费')).not.toBeInTheDocument();
    const item = screen.getByRole('button', { name: /暂不可读/ });
    expect(item).toBeDisabled();
    fireEvent.click(item);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it.each(['future_mode', null, ''] as const)('fails wire charge mode %s closed', (chargeMode) => {
    const onSelect = vi.fn();
    render(
      <ChapterDrawer
        open
        currentChapterId="9223372036854775801"
        onClose={vi.fn()}
        onSelect={onSelect}
        chapters={[chapter(1, {
          chargeMode,
          accessReason: 'membership',
          membershipEntitled: true,
          readable: true,
          purchasable: true,
          chapterPrice: 3
        })]}
      />
    );

    expect(screen.getByText('暂不可读')).toBeInTheDocument();
    expect(screen.queryByText('会员可读')).not.toBeInTheDocument();
    const item = screen.getByRole('button', { name: /暂不可读/ });
    expect(item).toBeDisabled();
    fireEvent.click(item);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it.each([
    ['book reason', { chargeMode: 'future_mode', accessReason: 'book_owned' }],
    ['chapter reason', { chargeMode: null, accessReason: 'chapter_owned' }],
    ['book flag', { chargeMode: '', accessReason: 'future_reason', bookPurchased: true }],
    ['chapter flag', { chargeMode: 'future_mode', accessReason: 'future_reason', chapterPurchased: true }]
  ])('keeps permanent ownership selectable for unknown wire state by %s', (_label, access) => {
    const onSelect = vi.fn();
    render(
      <ChapterDrawer
        open
        currentChapterId=""
        onClose={vi.fn()}
        onSelect={onSelect}
        chapters={[chapter(1, { ...access, readable: false })]}
      />
    );

    const item = screen.getByRole('button', { name: /已购买/ });
    expect(item).toBeEnabled();
    fireEvent.click(item);
    expect(onSelect).toHaveBeenCalledWith('9223372036854775801');
  });
});
