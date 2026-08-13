import { FormEvent, useEffect, useState } from 'react';
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Input } from 'animal-island-ui';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AuthFieldIcon, AuthScreen } from '../components/AuthScreen';
import { PasswordInput } from '../components/PasswordInput';
import { readerLocationHref, resolveReaderReturnLocation, safeReaderLocation } from '../utils/safeReaderLocation';

export function RegisterPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const requestedRedirect = searchParams.get('redirect');
  const safeRedirect = safeReaderLocation(requestedRedirect);
  const linkedInviteCode = searchParams.get('inviteCode')?.trim() || '';
  const inviteCodeLocked = linkedInviteCode.length > 0;
  const { isAuthenticated, register, sessionReady } = useReaderAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [inviteCode, setInviteCode] = useState(linkedInviteCode);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!sessionReady || !isAuthenticated) return;
    const target = resolveReaderReturnLocation(location.state, requestedRedirect);
    navigate({ pathname: target.pathname, search: target.search, hash: target.hash }, { replace: true, state: target.state });
  }, [isAuthenticated, location.state, navigate, requestedRedirect, sessionReady]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    if (!inviteCode.trim()) {
      setError('请输入邀请码');
      return;
    }
    if (password !== confirmPassword) {
      setError('两次密码不一致');
      return;
    }
    setLoading(true);
    try {
      const trimmedUsername = username.trim();
      await register({
        username: trimmedUsername,
        nickname: trimmedUsername,
        password,
        inviteCode: inviteCode.trim()
      });
      const target = resolveReaderReturnLocation(location.state, requestedRedirect);
      navigate({ pathname: target.pathname, search: target.search, hash: target.hash }, { replace: true, state: target.state });
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : '注册失败');
    } finally {
      setLoading(false);
    }
  };

  const loginHref = `/auth/login${safeRedirect ? `?redirect=${encodeURIComponent(readerLocationHref(safeRedirect))}` : ''}`;

  if (!sessionReady || isAuthenticated) return null;

  return (
    <AuthScreen subtitle="输入邀请码，加入月白书城">
      <form className="form-stack" onSubmit={submit}>
        <label>
          <span>用户名</span>
          <Input
            size="large"
            prefix={<AuthFieldIcon name="user" />}
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            name="username"
            autoComplete="username"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            placeholder="用于登录的账号"
          />
        </label>
        <label>
          <span>密码</span>
          <PasswordInput
            size="large"
            prefix={<AuthFieldIcon name="lock" />}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            name="new-password"
            autoComplete="new-password"
            enterKeyHint="next"
            placeholder="设置登录密码"
          />
        </label>
        <label>
          <span>确认密码</span>
          <PasswordInput
            size="large"
            prefix={<AuthFieldIcon name="lock" />}
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            name="confirm-password"
            autoComplete="new-password"
            enterKeyHint="next"
            placeholder="再次输入密码"
          />
        </label>
        <label>
          <span>邀请码</span>
          <Input
            size="large"
            prefix={<AuthFieldIcon name="gift" />}
            value={inviteCode}
            onChange={inviteCodeLocked ? undefined : (event) => setInviteCode(event.target.value)}
            readOnly={inviteCodeLocked}
            autoCapitalize="characters"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="done"
            placeholder="请输入邀请码"
          />
        </label>
        {error ? (
          <p className="notice notice--error" role="alert">
            {error}
          </p>
        ) : null}
        <Button block className="auth-submit" disabled={loading} htmlType="submit" loading={loading} size="large" type="primary">
          注册
        </Button>
      </form>
      <div className="auth-links">
        <Link to={loginHref} state={location.state}>已有账号</Link>
        <Link to="/">返回首页</Link>
      </div>
    </AuthScreen>
  );
}
