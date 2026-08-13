import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HomePage } from './HomePage';

const readerApi = vi.hoisted(() => ({
  listCategories: vi.fn(),
  listFeaturedBooks: vi.fn(),
  listSubCategories: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Input: ({ onChange, placeholder, suffix, value }: { onChange?: React.ChangeEventHandler<HTMLInputElement>; placeholder?: string; suffix?: React.ReactNode; value?: string }) => (
    <label>
      <input placeholder={placeholder} value={value} onChange={onChange} />
      {suffix}
    </label>
  ),
  Loading: () => <div>加载中</div>
}));

function CurrentLocation() {
  const location = useLocation();
  return <div>{location.pathname}{location.search}</div>;
}

afterEach(cleanup);

describe('HomePage sub-category navigation', () => {
  it('opens all sub-categories and navigates to the selected book filter', async () => {
    readerApi.listFeaturedBooks.mockResolvedValue([]);
    readerApi.listCategories.mockResolvedValue([{ categoryCode: 'fantasy', categoryName: '玄幻' }]);
    readerApi.listSubCategories.mockResolvedValue([{ categoryCode: 'system', categoryName: '系统' }]);

    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/books" element={<CurrentLocation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => expect(readerApi.listSubCategories).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: '标签' }));
    const panel = screen.getByRole('navigation', { name: '副分类' });
    fireEvent.click(within(panel).getByRole('button', { name: '系统' }));

    expect(await screen.findByText('/books?subCategory=system')).toBeInTheDocument();
  });
});
