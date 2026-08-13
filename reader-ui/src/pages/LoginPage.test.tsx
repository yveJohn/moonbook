import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { WalletDetailPage } from './WalletDetailPage';
import { LoginPage } from './LoginPage';

const readerApi = vi.hoisted(() => ({
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  queryWalletLedgers: vi.fn(),
  registerReader: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Button: ({ children, htmlType, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { htmlType?: 'button' | 'submit' }) => (
    <button {...props} type={htmlType}>{children}</button>
  ),
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => <section className={className}>{children}</section>,
  Input: ({ prefix: _prefix, suffix, size: _size, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { prefix?: React.ReactNode; suffix?: React.ReactNode }) => (
    <span><input {...props} />{suffix}</span>
  )
}));

function LocationProbe() {
  const location = useLocation();
  return <pre data-testid="destination">{JSON.stringify({ pathname: location.pathname, state: location.state })}</pre>;
}

describe('LoginPage return location', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.loginReader.mockResolvedValue({
      accessToken: 'reader-token',
      expireIn: 3600,
      reader: { readerId: '9223372036854775801', username: 'reader', nickname: '读者', status: 'enabled' }
    });
    readerApi.getWallet.mockResolvedValue({
      readerId: '9223372036854775801',
      rechargeCoinBalance: '120',
      bonusCoinBalance: '30',
      totalRechargeCoinIncome: '120',
      totalBonusCoinIncome: '30',
      totalRechargeCoinExpense: '0',
      totalBonusCoinExpense: '0',
      expiringBonusCoin: '0'
    });
    readerApi.queryWalletLedgers.mockResolvedValue({ rows: [], total: 0 });
  });

  afterEach(cleanup);

  it.each([
    ['/me/diamonds', '/me/diamonds'],
    ['https://evil.example/steal', '/me']
  ])('redirects an SSR-restored session from login target %s to %s', async (redirect, expectedPathname) => {
    window.localStorage.setItem('readerToken', 'reader-token');
    window.localStorage.setItem('readerProfile', JSON.stringify({
      readerId: '9223372036854775801', username: 'reader', nickname: '读者', status: 'enabled'
    }));

    render(
      <ReaderAuthProvider deferClientSession>
        <MemoryRouter initialEntries={[`/auth/login?redirect=${encodeURIComponent(redirect)}`]}>
          <Routes>
            <Route path="/auth/login" element={<LoginPage />} />
            <Route path="*" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </ReaderAuthProvider>
    );

    expect(await screen.findByTestId('destination')).toHaveTextContent(`"pathname":"${expectedPathname}"`);
    expect(readerApi.loginReader).not.toHaveBeenCalled();
  });

  it('returns to the target reader chapter from router state after login', async () => {
    render(
      <ReaderAuthProvider>
        <MemoryRouter initialEntries={[{
          pathname: '/auth/login',
          search: '?redirect=%2Fbooks%2Ffallback',
          state: {
            from: {
              pathname: '/read/9223372036854775806',
              state: { chapterId: '9223372036854775807' }
            }
          }
        }]}>
          <Routes>
            <Route path="/auth/login" element={<LoginPage />} />
            <Route path="*" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </ReaderAuthProvider>
    );

    fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: '登录' }));

    await waitFor(() => expect(readerApi.loginReader).toHaveBeenCalledWith('reader', 'secret'));
    const destination = await screen.findByTestId('destination');
    expect(destination).toHaveTextContent('"pathname":"/read/9223372036854775806"');
    expect(destination).toHaveTextContent('"chapterId":"9223372036854775807"');
  });

  it('ignores a from location outside known reader routes', async () => {
    render(
      <ReaderAuthProvider>
        <MemoryRouter initialEntries={[{
          pathname: '/auth/login',
          search: '?redirect=%2Fbooks',
          state: { from: { pathname: '/admin/system', state: { chapterId: '9223372036854775807' } } }
        }]}>
          <Routes>
            <Route path="/auth/login" element={<LoginPage />} />
            <Route path="*" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </ReaderAuthProvider>
    );

    fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByTestId('destination')).toHaveTextContent('"pathname":"/books"');
  });

  it.each([
    ['/admin/system', '/me'],
    ['https://evil.example/steal', '/me'],
    ['//evil.example/steal', '/me'],
    ['/books', '/books'],
    ['/read/9223372036854775806', '/read/9223372036854775806'],
    ['/me', '/me']
  ])('validates redirect %s and navigates to %s', async (redirect, expectedPathname) => {
    render(
      <ReaderAuthProvider>
        <MemoryRouter initialEntries={[`/auth/login?redirect=${encodeURIComponent(redirect)}`]}>
          <Routes>
            <Route path="/auth/login" element={<LoginPage />} />
            <Route path="*" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </ReaderAuthProvider>
    );

    fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByTestId('destination')).toHaveTextContent(`"pathname":"${expectedPathname}"`);
  });

  it('returns an anonymous diamond-detail visit to the same page after a successful login', async () => {
    render(
      <ReaderAuthProvider>
        <MemoryRouter initialEntries={['/me/diamonds']}>
          <Routes>
            <Route path="/auth/login" element={<LoginPage />} />
            <Route path="/me/diamonds" element={<WalletDetailPage coinType="recharge" />} />
          </Routes>
          <LocationProbe />
        </MemoryRouter>
      </ReaderAuthProvider>
    );

    expect(await screen.findByRole('button', { name: '登录' })).toBeInTheDocument();
    expect(screen.getByTestId('destination')).toHaveTextContent('"pathname":"/auth/login"');
    expect(readerApi.getWallet).not.toHaveBeenCalled();
    expect(readerApi.queryWalletLedgers).not.toHaveBeenCalled();

    fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByRole('heading', { level: 1, name: '钻石明细' })).toBeInTheDocument();
    expect(screen.getByTestId('destination')).toHaveTextContent('"pathname":"/me/diamonds"');
    expect(readerApi.loginReader).toHaveBeenCalledWith('reader', 'secret');
    expect(readerApi.queryWalletLedgers).toHaveBeenCalledWith({ coinType: 'recharge', pageNum: 1, pageSize: 20 });
  });
});
