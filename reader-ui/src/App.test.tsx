import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { App } from './App';

vi.mock('./pages/BookshelfPage', () => ({
  BookshelfPage: () => <div>书架路由页面</div>
}));

vi.mock('./pages/LikedBooksPage', () => ({
  LikedBooksPage: () => <div>赞过图书路由页面</div>
}));

vi.mock('./pages/SettingsPage', () => ({
  SettingsPage: () => <div>设置路由页面</div>
}));

vi.mock('./pages/FeedbackPage', () => ({
  FeedbackPage: () => <div>意见反馈路由页面</div>
}));

vi.mock('./pages/WalletDetailPage', () => ({
  WalletDetailPage: ({ coinType }: { coinType: string }) => <div>钱包明细路由页面 {coinType}</div>
}));

describe('App routes', () => {
  it('registers the bookshelf route', async () => {
    render(
      <MemoryRouter initialEntries={['/shelf']}>
        <App />
      </MemoryRouter>
    );

    expect(await screen.findByText('书架路由页面')).toBeInTheDocument();
  });

  it('registers the liked books route', async () => {
    render(
      <MemoryRouter initialEntries={['/me/likes']}>
        <App />
      </MemoryRouter>
    );

    expect(await screen.findByText('赞过图书路由页面')).toBeInTheDocument();
  });

  it('registers the settings route', async () => {
    render(
      <MemoryRouter initialEntries={['/me/settings']}>
        <App />
      </MemoryRouter>
    );

    expect(await screen.findByText('设置路由页面')).toBeInTheDocument();
  });

  it('registers the feedback route', async () => {
    render(
      <MemoryRouter initialEntries={['/me/feedback']}>
        <App />
      </MemoryRouter>
    );

    expect(await screen.findByText('意见反馈路由页面')).toBeInTheDocument();
  });

  it.each([
    ['/me/diamonds', 'recharge'],
    ['/me/coins', 'bonus']
  ])('registers wallet detail route %s with %s coin type', async (path, coinType) => {
    render(
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    );

    expect(await screen.findByText(`钱包明细路由页面 ${coinType}`)).toBeInTheDocument();
  });
});
