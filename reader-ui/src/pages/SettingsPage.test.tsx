import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../api/http';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import { SettingsPage } from './SettingsPage';

const readerApi = vi.hoisted(() => ({
  changeReaderPassword: vi.fn(),
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  registerReader: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Button: ({
    block: _block,
    children,
    htmlType,
    loading: _loading,
    size: _size,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement> & {
    block?: boolean;
    htmlType?: 'button' | 'submit';
    loading?: boolean;
    size?: string;
  }) => <button {...props} type={htmlType}>{children}</button>,
  Input: ({
    prefix: _prefix,
    size: _size,
    suffix,
    ...props
  }: React.InputHTMLAttributes<HTMLInputElement> & {
    prefix?: React.ReactNode;
    size?: string;
    suffix?: React.ReactNode;
  }) => <span><input {...props} />{suffix}</span>
}));

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{`${location.pathname}${location.search}`}</output>;
}

function HistoryBackProbe() {
  const navigate = useNavigate();
  return <button type="button" onClick={() => navigate(-1)}>测试返回</button>;
}

function renderSettingsPage(authenticated = true) {
  if (authenticated) {
    window.localStorage.setItem(READER_TOKEN_KEY, 'reader.jwt.token');
    window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify({
      readerId: '9223372036854775801',
      username: 'reader',
      nickname: '读者',
      status: 'enabled'
    }));
  }

  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={['/history-start', '/me/settings']} initialIndex={1}>
        <Routes>
          <Route path="/me/settings" element={<SettingsPage />} />
        </Routes>
        <LocationProbe />
        <HistoryBackProbe />
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

function fillPasswords(currentPassword = 'secret123', newPassword = 'newSecret456', confirmPassword = newPassword) {
  fireEvent.change(screen.getByLabelText('当前密码'), { target: { value: currentPassword } });
  fireEvent.change(screen.getByLabelText('新密码'), { target: { value: newPassword } });
  fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: confirmPassword } });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, reject, resolve };
}

function switchSession(token: string, username = 'new-reader') {
  const previousToken = window.localStorage.getItem(READER_TOKEN_KEY);
  window.localStorage.setItem(READER_TOKEN_KEY, token);
  window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify({
    readerId: '9223372036854775802', username, nickname: '新读者', status: 'enabled'
  }));
  window.dispatchEvent(new StorageEvent('storage', {
    key: READER_TOKEN_KEY,
    oldValue: previousToken,
    newValue: token,
    storageArea: window.localStorage
  }));
}

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.changeReaderPassword.mockResolvedValue(undefined);
    readerApi.logoutReader.mockResolvedValue(undefined);
  });

  afterEach(cleanup);

  it('replaces anonymous readers with login and does not render or submit settings actions', async () => {
    renderSettingsPage(false);

    expect(await screen.findByTestId('location')).toHaveTextContent('/auth/login?redirect=/me/settings');
    expect(screen.queryByRole('button', { name: '修改密码' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '退出登录' })).not.toBeInTheDocument();
    expect(readerApi.changeReaderPassword).not.toHaveBeenCalled();
    expect(readerApi.logoutReader).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '测试返回' }));
    expect(await screen.findByTestId('location')).toHaveTextContent('/history-start');
    expect(screen.getByTestId('location')).not.toHaveTextContent('/me/settings');
  });

  it('uses password-manager field metadata and one page heading', () => {
    renderSettingsPage();

    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 1, name: '设置' })).toBeInTheDocument();
    expect(screen.getByLabelText('当前密码')).toHaveAttribute('name', 'currentPassword');
    expect(screen.getByLabelText('当前密码')).toHaveAttribute('autocomplete', 'current-password');
    expect(screen.getByLabelText('当前密码')).toHaveAttribute('enterkeyhint', 'next');
    expect(screen.getByLabelText('新密码')).toHaveAttribute('name', 'newPassword');
    expect(screen.getByLabelText('新密码')).toHaveAttribute('autocomplete', 'new-password');
    expect(screen.getByLabelText('新密码')).toHaveAttribute('enterkeyhint', 'next');
    expect(screen.getByLabelText('确认新密码')).toHaveAttribute('name', 'confirmPassword');
    expect(screen.getByLabelText('确认新密码')).toHaveAttribute('autocomplete', 'new-password');
    expect(screen.getByLabelText('确认新密码')).toHaveAttribute('enterkeyhint', 'done');
    expect(screen.getByRole('button', { name: '退出登录' })).toBeInTheDocument();
  });

  it.each([
    ['', 'newSecret456', 'newSecret456', '请输入当前密码'],
    ['secret123', '', '', '请输入新密码'],
    ['secret123', 'newSecret456', '', '请确认新密码']
  ])('validates required password fields', (currentPassword, newPassword, confirmPassword, message) => {
    renderSettingsPage();
    fillPasswords(currentPassword, newPassword, confirmPassword);

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(screen.getByRole('alert')).toHaveTextContent(message);
    expect(readerApi.changeReaderPassword).not.toHaveBeenCalled();
  });

  it.each([
    ['current password', '      ', 'newSecret456', 'newSecret456', '请输入当前密码'],
    ['new password', 'secret123', '      ', '      ', '请输入新密码'],
    ['confirmation', 'secret123', 'newSecret456', '      ', '请确认新密码']
  ])('rejects a whitespace-only %s without trimming submitted password values', (_name, currentPassword, newPassword, confirmPassword, message) => {
    renderSettingsPage();
    fillPasswords(currentPassword, newPassword, confirmPassword);

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(screen.getByRole('alert')).toHaveTextContent(message);
    expect(readerApi.changeReaderPassword).not.toHaveBeenCalled();
  });

  it.each([
    ['short current password', '12345', 'newSecret456', 'newSecret456', '当前密码长度应为6-64个字符'],
    ['long current password', 'a'.repeat(65), 'newSecret456', 'newSecret456', '当前密码长度应为6-64个字符'],
    ['short new password', 'secret123', '12345', '12345', '新密码长度应为6-64个字符'],
    ['long new password', 'secret123', 'b'.repeat(65), 'b'.repeat(65), '新密码长度应为6-64个字符'],
    ['short confirmation', 'secret123', 'newSecret456', '12345', '确认新密码长度应为6-64个字符'],
    ['long confirmation', 'secret123', 'newSecret456', 'c'.repeat(65), '确认新密码长度应为6-64个字符']
  ])('validates the 6-64 character boundary for %s', (_name, currentPassword, newPassword, confirmPassword, message) => {
    renderSettingsPage();
    fillPasswords(currentPassword, newPassword, confirmPassword);

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(screen.getByRole('alert')).toHaveTextContent(message);
    expect(readerApi.changeReaderPassword).not.toHaveBeenCalled();
  });

  it('accepts passwords at the 6 and 64 character boundaries', async () => {
    renderSettingsPage();
    fillPasswords('a'.repeat(6), 'b'.repeat(64), 'b'.repeat(64));

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenCalledTimes(1));
  });

  it('rejects mismatched new-password confirmation', () => {
    renderSettingsPage();
    fillPasswords('secret123', 'newSecret456', 'different789');

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(screen.getByRole('alert')).toHaveTextContent('两次输入的新密码不一致');
    expect(readerApi.changeReaderPassword).not.toHaveBeenCalled();
  });

  it('rejects a new password equal to the current password', () => {
    renderSettingsPage();
    fillPasswords('secret123', 'secret123', 'secret123');

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(screen.getByRole('alert')).toHaveTextContent('新密码不能与当前密码相同');
    expect(readerApi.changeReaderPassword).not.toHaveBeenCalled();
  });

  it('submits the exact password payload without trimming values', async () => {
    renderSettingsPage();
    fillPasswords(' old password ', ' new password ', ' new password ');

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenCalledWith({
      currentPassword: ' old password ',
      newPassword: ' new password ',
      confirmPassword: ' new password '
    }, 'reader.jwt.token'));
  });

  it('prevents duplicate password requests while submission is pending', async () => {
    const request = deferred<void>();
    readerApi.changeReaderPassword.mockReturnValue(request.promise);
    renderSettingsPage();
    fillPasswords();

    const form = screen.getByRole('button', { name: '修改密码' }).closest('form');
    expect(form).not.toBeNull();
    fireEvent.submit(form!);
    fireEvent.submit(form!);

    expect(readerApi.changeReaderPassword).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: '修改密码' })).toBeDisabled();

    await act(async () => request.reject(new Error('offline')));
  });

  it('keeps authentication and field values after a backend business failure', async () => {
    readerApi.changeReaderPassword.mockRejectedValue(new ApiError(400, '当前密码错误'));
    renderSettingsPage();
    fillPasswords();

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('当前密码错误');
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBe('reader.jwt.token');
    expect(screen.getByLabelText('当前密码')).toHaveValue('secret123');
    expect(screen.getByLabelText('新密码')).toHaveValue('newSecret456');
    expect(screen.getByLabelText('确认新密码')).toHaveValue('newSecret456');
    expect(screen.getByTestId('location')).toHaveTextContent('/me/settings');
  });

  it.each([
    '当前密码错误',
    '两次输入的新密码不一致',
    '新密码不能与当前密码相同',
    '当前密码不能为空',
    '当前密码长度必须在6到64个字符之间',
    '新密码不能为空',
    '新密码长度必须在6到64个字符之间',
    '确认密码不能为空',
    '确认密码长度必须在6到64个字符之间',
    '账号已禁用',
    '读者不存在',
    '修改密码失败'
  ])('shows the known safe backend message: %s', async (message) => {
    readerApi.changeReaderPassword.mockRejectedValue(new ApiError(400, message));
    renderSettingsPage();
    fillPasswords();

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message);
  });

  it('hides unknown API error details behind the safe fallback', async () => {
    readerApi.changeReaderPassword.mockRejectedValue(new ApiError(500, 'sql update internal detail'));
    renderSettingsPage();
    fillPasswords();

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('修改密码失败');
    expect(screen.getByRole('alert')).not.toHaveTextContent('sql update internal detail');
  });

  it('uses a safe message for ordinary unknown password failures', async () => {
    readerApi.changeReaderPassword.mockRejectedValue(new Error('socket host internal detail'));
    renderSettingsPage();
    fillPasswords();

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('修改密码失败');
    expect(screen.getByRole('alert')).not.toHaveTextContent('socket host internal detail');
  });

  it('clears the already invalidated local session and replaces with login after password change', async () => {
    renderSettingsPage();
    fillPasswords();

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/auth/login'));
    expect(screen.getByTestId('location')).not.toHaveTextContent('redirect=');
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
    expect(readerApi.logoutReader).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '测试返回' }));
    expect(await screen.findByTestId('location')).toHaveTextContent('/history-start');
    expect(screen.getByTestId('location')).not.toHaveTextContent('/me/settings');
  });

  it.each([
    ['remote success', false],
    ['remote failure', true]
  ])('clears local session and opens home on manual logout after %s', async (_name, rejects) => {
    if (rejects) readerApi.logoutReader.mockRejectedValue(new Error('remote logout failed'));
    renderSettingsPage();

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));

    await waitFor(() => expect(readerApi.logoutReader).toHaveBeenCalledTimes(1));
    expect(await screen.findByTestId('location')).toHaveTextContent(/^\/$/);
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '测试返回' }));
    expect(await screen.findByTestId('location')).toHaveTextContent('/history-start');
    expect(screen.getByTestId('location')).not.toHaveTextContent('/me/settings');
  });

  it('clears the original session without navigating when password change resolves after the settings page unmounts', async () => {
    const request = deferred<void>();
    readerApi.changeReaderPassword.mockReturnValue(request.promise);
    renderSettingsPage();
    fillPasswords();
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: '测试返回' }));
    expect(await screen.findByTestId('location')).toHaveTextContent('/history-start');

    await act(async () => request.resolve());

    expect(screen.getByTestId('location')).toHaveTextContent('/history-start');
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
    expect(readerApi.logoutReader).not.toHaveBeenCalled();
  });

  it('does not clear or navigate away from a newer session after password change resolves', async () => {
    const request = deferred<void>();
    readerApi.changeReaderPassword.mockReturnValue(request.promise);
    renderSettingsPage();
    fillPasswords();
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenCalledTimes(1));

    switchSession('new-reader-token');

    await act(async () => request.resolve());

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBe('new-reader-token');
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toContain('new-reader');
    expect(screen.getByTestId('location')).toHaveTextContent('/me/settings');
  });

  it('clears sensitive fields and validation errors when the authenticated token changes', async () => {
    renderSettingsPage();
    fillPasswords('secret123', 'newSecret456', 'different789');
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    expect(screen.getByRole('alert')).toHaveTextContent('两次输入的新密码不一致');

    switchSession('token-B');

    await waitFor(() => expect(screen.getByLabelText('当前密码')).toHaveValue(''));
    expect(screen.getByLabelText('新密码')).toHaveValue('');
    expect(screen.getByLabelText('确认新密码')).toHaveValue('');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled();
  });

  it('lets a new session submit while ignoring a late failure from the old session', async () => {
    const oldRequest = deferred<void>();
    const newRequest = deferred<void>();
    readerApi.changeReaderPassword
      .mockReturnValueOnce(oldRequest.promise)
      .mockReturnValueOnce(newRequest.promise);
    renderSettingsPage();
    fillPasswords('oldSecret123', 'oldNewSecret456', 'oldNewSecret456');
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenNthCalledWith(1, {
      currentPassword: 'oldSecret123',
      newPassword: 'oldNewSecret456',
      confirmPassword: 'oldNewSecret456'
    }, 'reader.jwt.token'));

    switchSession('token-B');
    await waitFor(() => expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled());
    expect(screen.getByLabelText('当前密码')).toHaveValue('');
    expect(screen.getByLabelText('新密码')).toHaveValue('');
    expect(screen.getByLabelText('确认新密码')).toHaveValue('');

    fillPasswords('newSecret123', 'newerSecret456', 'newerSecret456');
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    await waitFor(() => expect(readerApi.changeReaderPassword).toHaveBeenNthCalledWith(2, {
      currentPassword: 'newSecret123',
      newPassword: 'newerSecret456',
      confirmPassword: 'newerSecret456'
    }, 'token-B'));
    expect(screen.getByRole('button', { name: '修改密码' })).toBeDisabled();

    await act(async () => oldRequest.reject(new ApiError(400, '当前密码错误')));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '修改密码' })).toBeDisabled();

    await act(async () => newRequest.reject(new ApiError(400, '当前密码错误')));
    expect(screen.getByRole('alert')).toHaveTextContent('当前密码错误');
    expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled();
  });

  it('returns to the me page from the app bar', async () => {
    renderSettingsPage();

    fireEvent.click(screen.getByRole('button', { name: '返回' }));

    expect(await screen.findByTestId('location')).toHaveTextContent('/me');
  });
});
