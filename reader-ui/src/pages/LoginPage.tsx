import { FormEvent, useEffect, useState } from 'react';
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Input } from 'animal-island-ui';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AuthFieldIcon, AuthScreen } from '../components/AuthScreen';
import { PasswordInput } from '../components/PasswordInput';
import { readerLocationHref, resolveReaderReturnLocation, safeReaderLocation } from '../utils/safeReaderLocation';

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const requestedRedirect = searchParams.get('redirect');
  const safeRedirect = safeReaderLocation(requestedRedirect);
  const { isAuthenticated, login, sessionReady } = useReaderAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!sessionReady || !isAuthenticated) return;
    const target = resolveReaderReturnLocation(location.state, requestedRedirect);
    navigate({ pathname: target.pathname, search: target.search, hash: target.hash }, { replace: true, state: target.state });
  }, [isAuthenticated, location.state, navigate, requestedRedirect, sessionReady]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username.trim(), password);
      const target = resolveReaderReturnLocation(location.state, requestedRedirect);
      navigate({ pathname: target.pathname, search: target.search, hash: target.hash }, { replace: true, state: target.state });
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : '登录失败');
    } finally {
      setLoading(false);
    }
  };

  const registerHref = `/auth/register${safeRedirect ? `?redirect=${encodeURIComponent(readerLocationHref(safeRedirect))}` : ''}`;

  if (!sessionReady || isAuthenticated) return null;

  return (
    <AuthScreen subtitle="登录后继续阅读">
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
            name="password"
            autoComplete="current-password"
            enterKeyHint="go"
            placeholder="输入登录密码"
          />
        </label>
        {error ? (
          <p className="notice notice--error" role="alert">
            {error}
          </p>
        ) : null}
        <Button block className="auth-submit" disabled={loading} htmlType="submit" loading={loading} size="large" type="primary">
          登录
        </Button>
      </form>
      <div className="auth-links">
        <Link to={registerHref} state={location.state}>注册账号</Link>
        <Link to="/">返回首页</Link>
      </div>
    </AuthScreen>
  );
}
