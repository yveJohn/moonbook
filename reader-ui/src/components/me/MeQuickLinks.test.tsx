import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { MeQuickLinks } from './MeQuickLinks';

describe('MeQuickLinks', () => {
  afterEach(() => cleanup());

  it('renders three icon links inside a fixed four-column function grid', () => {
    const { container } = render(
      <MemoryRouter>
        <MeQuickLinks />
      </MemoryRouter>
    );

    const nav = screen.getByRole('navigation', { name: '我的功能' });
    expect(nav).toHaveClass('me-quick-links');
    expect(nav).toHaveAttribute('data-section', 'quick-links');
    expect(nav.querySelectorAll('a')).toHaveLength(3);
    expect(screen.getByRole('link', { name: '赞过' })).toHaveAttribute('href', '/me/likes');
    expect(screen.getByRole('link', { name: '意见反馈' })).toHaveAttribute('href', '/me/feedback');
    expect(screen.getByRole('link', { name: '设置' })).toHaveAttribute('href', '/me/settings');
    expect(container.querySelector('.lucide-heart')).not.toBeNull();
    expect(container.querySelector('.lucide-message-square')).not.toBeNull();
    expect(container.querySelector('.lucide-settings')).not.toBeNull();
  });
});
