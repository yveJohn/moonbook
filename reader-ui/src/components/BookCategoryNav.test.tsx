import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BookCategoryNav } from './BookCategoryNav';

const categories = [
  { categoryCode: 'fantasy', categoryName: '玄幻' },
  { categoryCode: 'urban', categoryName: '都市' }
];
const subCategories = [
  { categoryCode: 'system', categoryName: '系统' },
  { categoryCode: 'rebirth', categoryName: '重生' }
];

afterEach(cleanup);

describe('BookCategoryNav', () => {
  it('opens and closes the sub-category panel from the dedicated tag button', () => {
    render(
      <BookCategoryNav
        categories={categories}
        subCategories={subCategories}
        onSelectCategory={vi.fn()}
        onSelectSubCategory={vi.fn()}
      />
    );

    const tagButton = screen.getByRole('button', { name: '标签', expanded: false });
    expect(screen.getByRole('button', { name: '全部' })).toHaveClass('is-active');

    fireEvent.click(tagButton);

    const panel = screen.getByRole('navigation', { name: '副分类' });
    const options = panel.querySelector('.sub-category-panel__options');
    const collapseButton = within(panel).getByRole('button', { name: '收起分类' });

    expect(options).toBeInTheDocument();
    expect(options).not.toContainElement(collapseButton);
    expect(collapseButton.parentElement).toBe(panel);
    expect(screen.getByRole('button', { name: '标签', expanded: true })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '标签', expanded: true }));

    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
  });

  it('selects a sub-category and closes the panel', () => {
    const onSelectSubCategory = vi.fn();
    render(
      <BookCategoryNav
        categories={categories}
        subCategories={subCategories}
        onSelectCategory={vi.fn()}
        onSelectSubCategory={onSelectSubCategory}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: '标签' }));
    fireEvent.click(within(screen.getByRole('navigation', { name: '副分类' })).getByRole('button', { name: '系统' }));

    expect(onSelectSubCategory).toHaveBeenCalledWith('system');
    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
  });

  it('shows the selected sub-category in the tag button and clears it from the panel', () => {
    const onSelectSubCategory = vi.fn();
    render(
      <BookCategoryNav
        categories={categories}
        subCategories={subCategories}
        selectedSubCategoryCode="system"
        onSelectCategory={vi.fn()}
        onSelectSubCategory={onSelectSubCategory}
      />
    );

    expect(screen.getByRole('button', { name: '全部' })).not.toHaveClass('is-active');
    expect(screen.getByRole('button', { name: '系统', expanded: false })).toHaveClass('is-active');

    fireEvent.click(screen.getByRole('button', { name: '系统' }));
    const panel = screen.getByRole('navigation', { name: '副分类' });
    expect(within(panel).getByRole('button', { name: '系统' })).toHaveClass('is-active');
    fireEvent.click(within(panel).getByRole('button', { name: '不限标签' }));

    expect(onSelectSubCategory).toHaveBeenCalledWith('');
    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
  });

  it.each([
    { selectedCategoryCode: 'fantasy', selectedSubCategoryCode: '' },
    { selectedCategoryCode: '', selectedSubCategoryCode: 'system' }
  ])('clears the current filter from the top-level all button without opening the panel', ({ selectedCategoryCode, selectedSubCategoryCode }) => {
    const onSelectCategory = vi.fn();
    render(
      <BookCategoryNav
        categories={categories}
        subCategories={subCategories}
        selectedCategoryCode={selectedCategoryCode}
        selectedSubCategoryCode={selectedSubCategoryCode}
        onSelectCategory={onSelectCategory}
        onSelectSubCategory={vi.fn()}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: '全部' }));

    expect(onSelectCategory).toHaveBeenCalledWith('');
    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
  });

  it('closes the tag panel when a main category is selected', () => {
    const onSelectCategory = vi.fn();
    render(
      <BookCategoryNav
        categories={categories}
        subCategories={subCategories}
        onSelectCategory={onSelectCategory}
        onSelectSubCategory={vi.fn()}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: '标签' }));
    fireEvent.click(screen.getByRole('button', { name: '玄幻' }));

    expect(onSelectCategory).toHaveBeenCalledWith('fantasy');
    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
  });

  it('collapses without changing the current filter', () => {
    const onSelectCategory = vi.fn();
    const onSelectSubCategory = vi.fn();
    render(
      <BookCategoryNav
        categories={categories}
        subCategories={subCategories}
        onSelectCategory={onSelectCategory}
        onSelectSubCategory={onSelectSubCategory}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: '标签' }));
    fireEvent.click(screen.getByRole('button', { name: '收起分类' }));

    expect(onSelectCategory).not.toHaveBeenCalled();
    expect(onSelectSubCategory).not.toHaveBeenCalled();
    expect(screen.queryByRole('navigation', { name: '副分类' })).not.toBeInTheDocument();
  });
});
