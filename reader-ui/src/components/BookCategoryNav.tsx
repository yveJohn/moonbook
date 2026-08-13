import { useEffect, useId, useRef, useState } from 'react';
import { ChevronDown, Tag } from 'lucide-react';
import type { ReaderCategory } from '../types/reader';

interface BookCategoryNavProps {
  categories: ReaderCategory[];
  subCategories: ReaderCategory[];
  selectedCategoryCode?: string;
  selectedSubCategoryCode?: string;
  onSelectCategory: (categoryCode: string) => void;
  onSelectSubCategory: (categoryCode: string) => void;
}

export function BookCategoryNav({
  categories,
  subCategories,
  selectedCategoryCode = '',
  selectedSubCategoryCode = '',
  onSelectCategory,
  onSelectSubCategory
}: BookCategoryNavProps) {
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const categoryButtonRefs = useRef(new Map<string, HTMLButtonElement>());
  const selectedSubCategory = subCategories.find((category) => category.categoryCode === selectedSubCategoryCode);
  const tagLabel = selectedSubCategoryCode
    ? selectedSubCategory?.categoryName || selectedSubCategoryCode
    : '标签';
  const allSelected = !selectedCategoryCode && !selectedSubCategoryCode;

  useEffect(() => {
    if (!selectedCategoryCode) {
      return;
    }
    categoryButtonRefs.current.get(selectedCategoryCode)?.scrollIntoView?.({
      behavior: 'smooth',
      block: 'nearest',
      inline: 'center'
    });
  }, [categories, selectedCategoryCode]);

  const setCategoryButtonRef = (code: string) => (node: HTMLButtonElement | null) => {
    if (node) {
      categoryButtonRefs.current.set(code, node);
    } else {
      categoryButtonRefs.current.delete(code);
    }
  };

  const chooseAll = () => {
    setOpen(false);
    onSelectCategory('');
  };

  const chooseCategory = (categoryCode: string) => {
    setOpen(false);
    onSelectCategory(categoryCode);
  };

  const chooseSubCategory = (categoryCode: string) => {
    setOpen(false);
    onSelectSubCategory(categoryCode);
  };

  return (
    <div className="book-category-nav">
      <nav className="cat-strip" aria-label="分类">
        <button
          className={allSelected ? 'cat-chip is-active' : 'cat-chip'}
          type="button"
          onClick={chooseAll}
        >
          全部
        </button>
        <button
          className={`cat-chip category-tag-toggle${selectedSubCategoryCode ? ' is-active' : ''}${open ? ' is-open' : ''}`}
          type="button"
          aria-expanded={open}
          aria-controls={panelId}
          title={tagLabel}
          onClick={() => setOpen((current) => !current)}
        >
          <Tag className="category-tag-toggle__icon" size={14} aria-hidden="true" />
          <span>{tagLabel}</span>
          <ChevronDown className="category-tag-toggle__arrow" size={14} aria-hidden="true" />
        </button>
        {categories.map((category) => (
          <button
            ref={setCategoryButtonRef(category.categoryCode)}
            className={category.categoryCode === selectedCategoryCode ? 'cat-chip is-active' : 'cat-chip'}
            key={category.categoryCode}
            type="button"
            onClick={() => chooseCategory(category.categoryCode)}
          >
            {category.categoryName || category.categoryCode}
          </button>
        ))}
      </nav>
      {open ? (
        <nav className="sub-category-panel" id={panelId} aria-label="副分类">
          <div className="sub-category-panel__options">
            <button
              className={`sub-category-chip${selectedSubCategoryCode ? '' : ' is-active'}`}
              type="button"
              onClick={() => chooseSubCategory('')}
            >
              不限标签
            </button>
            {subCategories.map((category) => (
              <button
                className={`sub-category-chip${category.categoryCode === selectedSubCategoryCode ? ' is-active' : ''}`}
                key={category.categoryCode}
                type="button"
                onClick={() => chooseSubCategory(category.categoryCode)}
              >
                {category.categoryName || category.categoryCode}
              </button>
            ))}
          </div>
          <button className="sub-category-panel__collapse" type="button" onClick={() => setOpen(false)}>
            收起分类
          </button>
        </nav>
      ) : null}
    </div>
  );
}
