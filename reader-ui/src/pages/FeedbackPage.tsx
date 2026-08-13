import { type FormEvent, useCallback, useEffect, useRef, useState } from 'react';
import { Button } from 'animal-island-ui';
import { MessageSquare } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { createFeedback, queryFeedbacks } from '../api/reader';
import { ApiError } from '../api/http';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import type { ReaderFeedback } from '../types/reader';

const FEEDBACK_MIN_LENGTH = 5;
const FEEDBACK_MAX_LENGTH = 1000;
const PAGE_SIZE = 10;
const SAFE_SUBMIT_MESSAGES = new Set([
  '反馈内容应为5-1000个字符',
  '反馈内容不能为空',
  '反馈内容不能超过1000个字符',
  '提交反馈失败'
]);

function formatFeedbackTime(value: string | null) {
  if (!value) return '时间待确认';
  const date = new Date(value.replace(' ', 'T'));
  return Number.isNaN(date.getTime()) ? '时间待确认' : date.toLocaleString('zh-CN', { hour12: false });
}

function feedbackSubmitError(error: unknown) {
  if (error instanceof ApiError && SAFE_SUBMIT_MESSAGES.has(error.message)) return error.message;
  return '反馈提交失败，请稍后重试';
}

function mergeFeedbacks(current: ReaderFeedback[], next: ReaderFeedback[]) {
  const seen = new Set(current.map((item) => item.id));
  return [...current, ...next.filter((item) => !seen.has(item.id))];
}

function FeedbackItem({ feedback }: { feedback: ReaderFeedback }) {
  return (
    <article className="feedback-history-item" data-testid="feedback-history-item">
      <div className="feedback-history-item__heading">
        <span className="feedback-history-item__status" data-status={feedback.status}>
          {feedback.status === 'replied' ? '已回复' : '待处理'}
        </span>
        <time>{formatFeedbackTime(feedback.createTime)}</time>
      </div>
      <p className="feedback-history-item__content">{feedback.content}</p>
      {feedback.status === 'replied' && feedback.replyContent ? (
        <div className="feedback-history-item__reply">
          <strong>管理员回复</strong>
          <p>{feedback.replyContent}</p>
          <time>{formatFeedbackTime(feedback.replyTime)}</time>
        </div>
      ) : null}
    </article>
  );
}

export function FeedbackPage() {
  const navigate = useNavigate();
  const { sessionReady, token } = useReaderAuth();
  const [content, setContent] = useState('');
  const [feedbacks, setFeedbacks] = useState<ReaderFeedback[]>([]);
  const [total, setTotal] = useState(0);
  const [pageNum, setPageNum] = useState(1);
  const [initialLoading, setInitialLoading] = useState(Boolean(token));
  const [moreLoading, setMoreLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [listError, setListError] = useState('');
  const [submitError, setSubmitError] = useState('');
  const mountedRef = useRef(true);
  const identityRef = useRef(token);
  const listSequence = useRef(0);
  const submitSequence = useRef(0);
  const submittingRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      listSequence.current += 1;
      submitSequence.current += 1;
    };
  }, []);

  const loadFeedbacks = useCallback(async (identity: string, nextPage: number, append: boolean) => {
    const sequence = ++listSequence.current;
    const canPublish = () => mountedRef.current
      && listSequence.current === sequence
      && identityRef.current === identity;
    append ? setMoreLoading(true) : setInitialLoading(true);
    setListError('');
    try {
      const result = await queryFeedbacks(nextPage, PAGE_SIZE);
      if (!canPublish()) return;
      setFeedbacks((current) => append ? mergeFeedbacks(current, result.rows) : result.rows);
      setTotal(result.total);
      setPageNum(nextPage);
    } catch {
      if (canPublish()) setListError('历史反馈加载失败');
    } finally {
      if (canPublish()) {
        setInitialLoading(false);
        setMoreLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (!sessionReady) return;
    identityRef.current = token;
    listSequence.current += 1;
    submitSequence.current += 1;
    submittingRef.current = false;
    setContent('');
    setFeedbacks([]);
    setTotal(0);
    setPageNum(1);
    setListError('');
    setSubmitError('');
    setSubmitting(false);
    setMoreLoading(false);
    if (!token) {
      setInitialLoading(false);
      navigate('/auth/login?redirect=/me/feedback', { replace: true });
      return;
    }
    void loadFeedbacks(token, 1, false);
  }, [loadFeedbacks, navigate, sessionReady, token]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submittingRef.current || !token) return;
    const trimmed = content.trim();
    if (trimmed.length < FEEDBACK_MIN_LENGTH || trimmed.length > FEEDBACK_MAX_LENGTH) {
      setSubmitError('反馈内容应为5-1000个字符');
      return;
    }
    submittingRef.current = true;
    setSubmitting(true);
    setSubmitError('');
    const identity = token;
    const sequence = ++submitSequence.current;
    try {
      const created = await createFeedback(trimmed);
      if (!mountedRef.current || submitSequence.current !== sequence || identityRef.current !== identity) return;
      setFeedbacks((current) => current.some((item) => item.id === created.id) ? current : [created, ...current]);
      setTotal((current) => current + 1);
      setContent('');
    } catch (error) {
      if (mountedRef.current && submitSequence.current === sequence && identityRef.current === identity) {
        setSubmitError(feedbackSubmitError(error));
      }
    } finally {
      if (submitSequence.current === sequence) {
        submittingRef.current = false;
        if (mountedRef.current) setSubmitting(false);
      }
    }
  };

  if (!token) return null;

  return (
    <AppShell active="me" title="意见反馈" back onBack={() => navigate('/me')}>
      <main className="reader-page reader-page--narrow feedback-page">
        <section className="feedback-compose" aria-labelledby="feedback-compose-title">
          <div className="feedback-section-heading">
            <span className="feedback-section-heading__icon" aria-hidden="true"><MessageSquare size={20} /></span>
            <div>
              <h2 id="feedback-compose-title">告诉我们你的想法</h2>
              <p>你的建议会帮助我们改善阅读体验。</p>
            </div>
          </div>
          <form onSubmit={(event) => { void submit(event); }}>
            <label className="feedback-textarea-label" htmlFor="feedback-content">反馈内容</label>
            <textarea
              id="feedback-content"
              maxLength={FEEDBACK_MAX_LENGTH}
              placeholder="请输入你的意见或遇到的问题..."
              value={content}
              disabled={submitting}
              onChange={(event) => setContent(event.target.value)}
            />
            <div className="feedback-compose__meta">
              <span>{content.length} / {FEEDBACK_MAX_LENGTH}</span>
            </div>
            {submitError ? <p className="feedback-form-error" role="alert">{submitError}</p> : null}
            <Button block disabled={submitting} htmlType="submit" loading={submitting} size="large" type="primary">
              立即反馈
            </Button>
          </form>
        </section>

        <section className="feedback-history" aria-labelledby="feedback-history-title">
          <div className="feedback-history__heading">
            <h2 id="feedback-history-title">历史反馈</h2>
            <span>共 {total} 条</span>
          </div>
          {initialLoading ? <p className="feedback-history__state" role="status">历史反馈加载中</p> : null}
          {!initialLoading && listError ? (
            <div className="feedback-history__state feedback-history__state--error" role="alert">
              <p>{listError}</p>
              <button type="button" onClick={() => { void loadFeedbacks(token, 1, false); }}>重新加载</button>
            </div>
          ) : null}
          {!initialLoading && !listError && feedbacks.length === 0 ? (
            <p className="feedback-history__state">暂无历史反馈</p>
          ) : null}
          {feedbacks.length > 0 ? (
            <div className="feedback-history__list">
              {feedbacks.map((feedback) => <FeedbackItem key={feedback.id} feedback={feedback} />)}
            </div>
          ) : null}
          {feedbacks.length < total ? (
            <button
              type="button"
              className="feedback-load-more"
              disabled={moreLoading}
              onClick={() => { void loadFeedbacks(token, pageNum + 1, true); }}
            >
              {moreLoading ? '加载中...' : '加载更多'}
            </button>
          ) : null}
        </section>
      </main>
    </AppShell>
  );
}
