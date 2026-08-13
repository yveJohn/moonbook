import { type FormEvent, useEffect, useRef, useState } from 'react';
import { Button } from 'animal-island-ui';
import { LogOut } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { changeReaderPassword } from '../api/reader';
import { ApiError } from '../api/http';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import { PasswordInput } from '../components/PasswordInput';

const PASSWORD_MIN_LENGTH = 6;
const PASSWORD_MAX_LENGTH = 64;
const SAFE_PASSWORD_CHANGE_MESSAGES = new Set([
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
]);

function passwordValidationError(currentPassword: string, newPassword: string, confirmPassword: string) {
  if (!currentPassword.trim()) return '请输入当前密码';
  if (!newPassword.trim()) return '请输入新密码';
  if (!confirmPassword.trim()) return '请确认新密码';
  if (currentPassword.length < PASSWORD_MIN_LENGTH || currentPassword.length > PASSWORD_MAX_LENGTH) {
    return '当前密码长度应为6-64个字符';
  }
  if (newPassword.length < PASSWORD_MIN_LENGTH || newPassword.length > PASSWORD_MAX_LENGTH) {
    return '新密码长度应为6-64个字符';
  }
  if (confirmPassword.length < PASSWORD_MIN_LENGTH || confirmPassword.length > PASSWORD_MAX_LENGTH) {
    return '确认新密码长度应为6-64个字符';
  }
  if (newPassword !== confirmPassword) return '两次输入的新密码不一致';
  if (newPassword === currentPassword) return '新密码不能与当前密码相同';
  return '';
}

function passwordChangeError(error: unknown) {
  if (error instanceof ApiError && SAFE_PASSWORD_CHANGE_MESSAGES.has(error.message)) return error.message;
  return '修改密码失败';
}

export function SettingsPage() {
  const navigate = useNavigate();
  const { clearSessionIfCurrent, isAuthenticated, logout, sessionReady, token } = useReaderAuth();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [logoutSaving, setLogoutSaving] = useState(false);
  const [error, setError] = useState('');
  const mountedRef = useRef(true);
  const passwordSavingRef = useRef(false);
  const passwordRequestSequence = useRef(0);
  const logoutSavingRef = useRef(false);
  const leavingRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (sessionReady && !isAuthenticated && !leavingRef.current) {
      navigate('/auth/login?redirect=/me/settings', { replace: true });
    }
  }, [isAuthenticated, navigate, sessionReady]);

  useEffect(() => {
    passwordRequestSequence.current += 1;
    passwordSavingRef.current = false;
    setCurrentPassword('');
    setNewPassword('');
    setConfirmPassword('');
    setError('');
    setPasswordSaving(false);
  }, [token]);

  const submitPassword = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (passwordSavingRef.current) return;

    const validationError = passwordValidationError(currentPassword, newPassword, confirmPassword);
    setError(validationError);
    if (validationError) return;

    passwordSavingRef.current = true;
    setPasswordSaving(true);
    const requestSequence = ++passwordRequestSequence.current;
    const submittedToken = token;
    try {
      await changeReaderPassword({ currentPassword, newPassword, confirmPassword }, submittedToken);
      if (mountedRef.current) leavingRef.current = true;
      const cleared = clearSessionIfCurrent(submittedToken);
      if (cleared && mountedRef.current) {
        navigate('/auth/login', { replace: true });
      } else if (mountedRef.current) {
        leavingRef.current = false;
      }
    } catch (nextError) {
      if (mountedRef.current && passwordRequestSequence.current === requestSequence) {
        setError(passwordChangeError(nextError));
      }
    } finally {
      if (passwordRequestSequence.current === requestSequence) {
        passwordSavingRef.current = false;
        if (mountedRef.current) setPasswordSaving(false);
      }
    }
  };

  const submitLogout = async () => {
    if (logoutSavingRef.current) return;
    logoutSavingRef.current = true;
    leavingRef.current = true;
    setLogoutSaving(true);
    try {
      await logout();
    } catch {
      // ReaderAuthProvider clears the local session even when remote logout fails.
    } finally {
      logoutSavingRef.current = false;
      if (mountedRef.current) {
        setLogoutSaving(false);
        navigate('/', { replace: true });
      }
    }
  };

  if (!sessionReady || !isAuthenticated) return null;

  return (
    <AppShell active="me" title="设置" back onBack={() => navigate('/me')}>
      <main className="reader-page reader-page--narrow settings-page">
        <section className="settings-password-surface" aria-labelledby="settings-password-title">
          <div className="settings-section-heading">
            <h2 id="settings-password-title">修改密码</h2>
            <p>修改后需要使用新密码重新登录。</p>
          </div>
          <form className="settings-password-form" onSubmit={(event) => { void submitPassword(event); }}>
            <label>
              <span>当前密码</span>
              <PasswordInput
                size="large"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                name="currentPassword"
                autoComplete="current-password"
                enterKeyHint="next"
              />
            </label>
            <label>
              <span>新密码</span>
              <PasswordInput
                size="large"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                name="newPassword"
                autoComplete="new-password"
                enterKeyHint="next"
              />
            </label>
            <label>
              <span>确认新密码</span>
              <PasswordInput
                size="large"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                name="confirmPassword"
                autoComplete="new-password"
                enterKeyHint="done"
              />
            </label>
            {error ? <p className="settings-form-error" role="alert">{error}</p> : null}
            <Button
              block
              className="settings-submit"
              disabled={passwordSaving}
              htmlType="submit"
              loading={passwordSaving}
              size="large"
              type="primary"
            >
              修改密码
            </Button>
          </form>
        </section>

        <section className="settings-logout" aria-labelledby="settings-logout-title">
          <div className="settings-section-heading">
            <h2 id="settings-logout-title">退出登录</h2>
            <p>退出当前账号，返回首页。</p>
          </div>
          <button
            type="button"
            className="settings-logout-button"
            disabled={logoutSaving}
            onClick={() => { void submitLogout(); }}
          >
            <LogOut size={19} strokeWidth={2} aria-hidden="true" />
            <span>退出登录</span>
          </button>
        </section>
      </main>
    </AppShell>
  );
}
