import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import type { ReaderFeedback } from '../types/reader';
import { FeedbackPage } from './FeedbackPage';

vi.mock('animal-island-ui', () => ({
  Button: ({ children, htmlType, loading: _loading, block: _block, size: _size, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { htmlType?: 'button' | 'submit'; loading?: boolean; block?: boolean; size?: string }) => (
    <button {...props} type={htmlType}>{children}</button>
  )
}));

const readerApi = vi.hoisted(() => ({
  createFeedback: vi.fn(),
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  queryFeedbacks: vi.fn(),
  registerReader: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}{location.search}</div>;
}

function loginReader() {
  window.localStorage.setItem(READER_TOKEN_KEY, 'reader.token');
  window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify({
    readerId: '9223372036854775801', username: 'reader', nickname: '读者', status: 'enabled'
  }));
}

function feedback(overrides: Partial<ReaderFeedback> = {}): ReaderFeedback {
  return {
    id: '9223372036854775802',
    content: '希望增加夜间阅读定时关闭',
    status: 'pending',
    replyContent: null,
    replyTime: null,
    createTime: '2026-07-31 10:26:00',
    ...overrides
  };
}

function renderPage() {
  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={['/me/feedback']}>
        <Routes>
          <Route path="/me/feedback" element={<FeedbackPage />} />
          <Route path="/me" element={<div>我的页面</div>} />
          <Route path="/auth/login" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

describe('FeedbackPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.queryFeedbacks.mockResolvedValue({ rows: [], total: 0 });
  });

  afterEach(() => cleanup());

  it('redirects anonymous readers with a return URL', async () => {
    renderPage();
    expect(await screen.findByTestId('location')).toHaveTextContent('/auth/login?redirect=/me/feedback');
    expect(readerApi.queryFeedbacks).not.toHaveBeenCalled();
  });

  it('shows empty history without disabling the feedback form', async () => {
    loginReader();
    renderPage();
    expect(await screen.findByText('暂无历史反馈')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '反馈内容' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '立即反馈' })).toBeEnabled();
  });

  it('validates trimmed content before submitting', async () => {
    loginReader();
    renderPage();
    await screen.findByText('暂无历史反馈');
    fireEvent.change(screen.getByRole('textbox', { name: '反馈内容' }), { target: { value: ' 四字 ' } });
    fireEvent.click(screen.getByRole('button', { name: '立即反馈' }));
    expect(await screen.findByText('反馈内容应为5-1000个字符')).toBeInTheDocument();
    expect(readerApi.createFeedback).not.toHaveBeenCalled();
  });

  it('prepends a successful submission and clears the editor', async () => {
    loginReader();
    const created = feedback();
    readerApi.createFeedback.mockResolvedValue(created);
    renderPage();
    await screen.findByText('暂无历史反馈');
    const editor = screen.getByRole('textbox', { name: '反馈内容' });
    fireEvent.change(editor, { target: { value: `  ${created.content}  ` } });
    fireEvent.click(screen.getByRole('button', { name: '立即反馈' }));

    expect(await screen.findByText(created.content)).toBeInTheDocument();
    expect(readerApi.createFeedback).toHaveBeenCalledWith(created.content);
    expect(editor).toHaveValue('');
    expect(screen.getByText('共 1 条')).toBeInTheDocument();
  });

  it('keeps content when submission fails', async () => {
    loginReader();
    readerApi.createFeedback.mockRejectedValue(new Error('server error'));
    renderPage();
    await screen.findByText('暂无历史反馈');
    const editor = screen.getByRole('textbox', { name: '反馈内容' });
    fireEvent.change(editor, { target: { value: '这个问题需要处理' } });
    fireEvent.click(screen.getByRole('button', { name: '立即反馈' }));
    expect(await screen.findByText('反馈提交失败，请稍后重试')).toBeInTheDocument();
    expect(editor).toHaveValue('这个问题需要处理');
  });

  it('renders replies and deduplicates shifted pagination', async () => {
    loginReader();
    const first = feedback({ status: 'replied', replyContent: '已修复，请刷新重试。', replyTime: '2026-07-31 11:00:00' });
    const second = feedback({ id: '9223372036854775803', content: '第二条反馈' });
    readerApi.queryFeedbacks
      .mockResolvedValueOnce({ rows: [first], total: 2 })
      .mockResolvedValueOnce({ rows: [first, second], total: 2 });
    renderPage();
    expect(await screen.findByText('已修复，请刷新重试。')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '加载更多' }));
    await waitFor(() => expect(screen.getAllByTestId('feedback-history-item')).toHaveLength(2));
    expect(screen.getAllByText(first.content)).toHaveLength(1);
    expect(screen.getByText('第二条反馈')).toBeInTheDocument();
  });

  it('keeps form usable when history loading fails and retries', async () => {
    loginReader();
    readerApi.queryFeedbacks
      .mockRejectedValueOnce(new Error('load failed'))
      .mockResolvedValueOnce({ rows: [feedback()], total: 1 });
    renderPage();
    expect(await screen.findByText('历史反馈加载失败')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '反馈内容' })).toBeEnabled();
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '重新加载' })));
    expect(await screen.findByText('希望增加夜间阅读定时关闭')).toBeInTheDocument();
  });
});
