import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { AppShell } from './AppShell';

function renderShell(active: 'home' | 'books' | 'shelf' | 'me' = 'shelf') {
  return render(
    <MemoryRouter>
      <AppShell active={active} title="书架">
        <main>页面内容</main>
      </AppShell>
    </MemoryRouter>
  );
}

describe('AppShell navigation', () => {
  it('renders the four main reader tabs with bookshelf after books', () => {
    renderShell();

    const links = screen.getAllByRole('link');

    expect(links.map((link) => link.textContent)).toEqual(['首页', '书库', '书架', '我的']);
    expect(screen.getByRole('link', { name: '书架' })).toHaveAttribute('href', '/shelf');
    expect(screen.getByRole('link', { name: '书架' })).toHaveClass('is-active');
  });

  it('can omit the main tabs on a focused sub-page', () => {
    const { container } = render(
      <MemoryRouter>
        <AppShell title="钻石明细" hideTabs>
          <main>钱包明细</main>
        </AppShell>
      </MemoryRouter>
    );

    expect(container.querySelector('[aria-label="主导航"]')).not.toBeInTheDocument();
    expect(container).not.toHaveTextContent('首页');
  });
});
