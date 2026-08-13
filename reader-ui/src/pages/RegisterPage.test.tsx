import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { LoginPage } from './LoginPage';
import { RegisterPage } from './RegisterPage';

const readerApi = vi.hoisted(() => ({
  getReaderProfile: vi.fn(), getWallet: vi.fn(), loginReader: vi.fn(), logoutReader: vi.fn(), registerReader: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);
vi.mock('animal-island-ui', () => ({
  Button: ({ children, htmlType, block: _block, loading: _loading, size: _size, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { htmlType?: 'button' | 'submit'; block?: boolean; loading?: boolean; size?: string }) => <button {...props} type={htmlType}>{children}</button>,
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => <section className={className}>{children}</section>,
  Input: ({ prefix: _prefix, suffix, size: _size, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { prefix?: React.ReactNode; suffix?: React.ReactNode; size?: string }) => <span><input {...props} />{suffix}</span>
}));

function Destination() {
  const location = useLocation();
  return <pre data-testid="destination">{JSON.stringify(location)}</pre>;
}

function renderAuth(entry: string | { pathname: string; search?: string; state?: Record<string, unknown> }) {
  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/auth/login" element={<LoginPage />} />
          <Route path="/auth/register" element={<RegisterPage />} />
          <Route path="*" element={<Destination />} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

function fillRegistration() {
  fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
  fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
  fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'secret' } });
  fireEvent.change(screen.getByRole('textbox', { name: '邀请码' }), { target: { value: 'INVITE' } });
  fireEvent.click(screen.getByRole('button', { name: '注册' }));
}

function fillLogin() {
  fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
  fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
  fireEvent.click(screen.getByRole('button', { name: '登录' }));
}

describe('RegisterPage safe return location', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    const result = { accessToken: 'token', expireIn: 3600, reader: { readerId: '9223372036854775801', username: 'reader', nickname: '读者', status: 'enabled' } };
    readerApi.registerReader.mockResolvedValue(result);
    readerApi.loginReader.mockResolvedValue(result);
  });
  afterEach(cleanup);

  it('redirects an authenticated reader away from registration', async () => {
    window.localStorage.setItem('readerToken', 'reader-token');
    window.localStorage.setItem('readerProfile', JSON.stringify({
      readerId: '9223372036854775801', username: 'reader', nickname: '读者', status: 'enabled'
    }));

    renderAuth('/auth/register?redirect=%2Fbooks');

    expect(await screen.findByTestId('destination')).toHaveTextContent('"pathname":"/books"');
    expect(readerApi.registerReader).not.toHaveBeenCalled();
  });

  it('keeps the full target chapter from login through registration', async () => {
    renderAuth({ pathname: '/auth/login', state: { from: { pathname: '/read/9223372036854775806', search: '?mode=page', hash: '#target', state: { chapterId: '9223372036854775807' } } } });
    fireEvent.click(screen.getByRole('link', { name: '注册账号' }));
    fillRegistration();
    const destination = await screen.findByTestId('destination');
    expect(destination).toHaveTextContent('"pathname":"/read/9223372036854775806"');
    expect(destination).toHaveTextContent('"search":"?mode=page"');
    expect(destination).toHaveTextContent('"hash":"#target"');
    expect(destination).toHaveTextContent('"chapterId":"9223372036854775807"');
  });

  it('keeps the full target chapter from registration back through login', async () => {
    renderAuth({ pathname: '/auth/register', state: { from: { pathname: '/read/9223372036854775806', state: { chapterId: '9223372036854775807' } } } });
    fireEvent.click(screen.getByRole('link', { name: '已有账号' }));
    fillLogin();
    expect(await screen.findByTestId('destination')).toHaveTextContent('"chapterId":"9223372036854775807"');
  });

  it.each([
    ['/admin/system', '/me'], ['https://evil.example/steal', '/me'], ['//evil.example/steal', '/me'],
    ['/books', '/books'], ['/read/9223372036854775806', '/read/9223372036854775806'], ['/me', '/me']
  ])('validates direct registration redirect %s', async (redirect, expectedPath) => {
    renderAuth(`/auth/register?redirect=${encodeURIComponent(redirect)}`);
    fillRegistration();
    expect(await screen.findByTestId('destination')).toHaveTextContent(`"pathname":"${expectedPath}"`);
  });

  it('fills and locks an invite code supplied by the invite link', () => {
    renderAuth('/auth/register?inviteCode=MB%20CODE%2F%2B');

    const inviteInput = screen.getByRole('textbox', { name: '邀请码' });
    expect(inviteInput).toHaveValue('MB CODE/+');
    expect(inviteInput).toHaveAttribute('readonly');
    fireEvent.change(inviteInput, { target: { value: 'CHANGED' } });
    expect(inviteInput).toHaveValue('MB CODE/+');
  });

  it('submits the locked invite code while preserving a safe redirect', async () => {
    renderAuth('/auth/register?inviteCode=MBLOCKED&redirect=%2Fbooks');
    fireEvent.change(screen.getByRole('textbox', { name: '用户名' }), { target: { value: 'reader' } });
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'secret' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: '注册' }));

    expect(await screen.findByTestId('destination')).toHaveTextContent('"pathname":"/books"');
    expect(readerApi.registerReader).toHaveBeenCalledWith(expect.objectContaining({ inviteCode: 'MBLOCKED' }));
  });

  it('keeps the invite code editable when the link parameter is absent or blank', () => {
    const first = renderAuth('/auth/register?inviteCode=%20%20');
    const inviteInput = screen.getByRole('textbox', { name: '邀请码' });
    expect(inviteInput).not.toHaveAttribute('readonly');
    fireEvent.change(inviteInput, { target: { value: 'MANUAL' } });
    expect(inviteInput).toHaveValue('MANUAL');
    first.unmount();

    renderAuth('/auth/register');
    expect(screen.getByRole('textbox', { name: '邀请码' })).not.toHaveAttribute('readonly');
  });
});
