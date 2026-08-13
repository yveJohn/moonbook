import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  getWallet,
  getReaderProfile,
  loginReader,
  logoutReader,
  registerReader
} from '../api/reader';
import type { ReaderLoginResult, ReaderProfile, ReaderRegisterPayload, ReaderWallet } from '../types/reader';
import {
  clearReaderSession,
  getStoredReaderProfile,
  getStoredReaderToken,
  READER_PROFILE_KEY,
  READER_SESSION_CHANGED_EVENT,
  READER_TOKEN_KEY,
  storeReaderSession
} from './session';

interface ReaderAuthContextValue {
  token: string;
  profile: ReaderProfile | null;
  isAuthenticated: boolean;
  sessionReady: boolean;
  wallet: ReaderWallet | null;
  walletLoading: boolean;
  walletError: string;
  login: (username: string, password: string) => Promise<void>;
  register: (payload: ReaderRegisterPayload) => Promise<void>;
  clearSession: () => void;
  clearSessionIfCurrent: (expectedToken: string) => boolean;
  logout: () => Promise<void>;
  refreshProfile: () => Promise<void>;
  refreshWallet: () => Promise<ReaderWallet | null>;
}

const ReaderAuthContext = createContext<ReaderAuthContextValue | null>(null);

function persistLogin(result: ReaderLoginResult) {
  storeReaderSession(result);
}

export function ReaderAuthProvider({
  children,
  deferClientSession = false
}: {
  children: React.ReactNode;
  deferClientSession?: boolean;
}) {
  const initialToken = deferClientSession ? '' : getStoredReaderToken();
  const [token, setToken] = useState(initialToken);
  const [profile, setProfile] = useState<ReaderProfile | null>(() => (
    deferClientSession ? null : getStoredReaderProfile()
  ));
  const [sessionReady, setSessionReady] = useState(!deferClientSession);
  const [wallet, setWallet] = useState<ReaderWallet | null>(null);
  const [walletLoading, setWalletLoading] = useState(false);
  const [walletError, setWalletError] = useState('');
  const authActionSequence = useRef(0);
  const walletRequestSequence = useRef(0);
  const mountedRef = useRef(true);
  const sessionTokenRef = useRef(initialToken);

  const useClientSessionEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

  const commitLogin = useCallback((result: ReaderLoginResult) => {
    walletRequestSequence.current += 1;
    sessionTokenRef.current = result.accessToken;
    persistLogin(result);
    setToken(result.accessToken);
    setProfile(result.reader);
    setWallet(null);
    setWalletLoading(false);
    setWalletError('');
  }, []);

  const login = useCallback(
    async (username: string, password: string) => {
      const actionSequence = ++authActionSequence.current;
      const result = await loginReader(username, password);
      if (mountedRef.current && authActionSequence.current === actionSequence) {
        commitLogin(result);
      }
    },
    [commitLogin]
  );

  const register = useCallback(
    async (payload: ReaderRegisterPayload) => {
      const actionSequence = ++authActionSequence.current;
      const result = await registerReader(payload);
      if (mountedRef.current && authActionSequence.current === actionSequence) {
        commitLogin(result);
      }
    },
    [commitLogin]
  );

  const clearPersistedSession = useCallback(() => {
    walletRequestSequence.current += 1;
    sessionTokenRef.current = '';
    clearReaderSession();
  }, []);

  const resetSessionState = useCallback(() => {
    setToken('');
    setProfile(null);
    setWallet(null);
    setWalletLoading(false);
    setWalletError('');
  }, []);

  const clearSession = useCallback(() => {
    authActionSequence.current += 1;
    clearPersistedSession();
    if (mountedRef.current) resetSessionState();
  }, [clearPersistedSession, resetSessionState]);

  const clearSessionIfCurrent = useCallback((expectedToken: string) => {
    if (!expectedToken || getStoredReaderToken() !== expectedToken) return false;
    authActionSequence.current += 1;
    clearPersistedSession();
    if (mountedRef.current) resetSessionState();
    return true;
  }, [clearPersistedSession, resetSessionState]);

  const logout = useCallback(async () => {
    authActionSequence.current += 1;
    const logoutToken = getStoredReaderToken();
    walletRequestSequence.current += 1;
    setWalletLoading(false);
    try {
      if (logoutToken) {
        await logoutReader();
      }
    } finally {
      if (getStoredReaderToken() === logoutToken) {
        clearPersistedSession();
        if (mountedRef.current) resetSessionState();
      }
    }
  }, [clearPersistedSession, resetSessionState]);

  const refreshProfile = useCallback(async () => {
    const sessionToken = getStoredReaderToken();
    if (!sessionToken) return;
    const actionSequence = authActionSequence.current;
    const nextProfile = await getReaderProfile();
    if (!mountedRef.current
      || authActionSequence.current !== actionSequence
      || getStoredReaderToken() !== sessionToken) return;
    window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify(nextProfile));
    setProfile(nextProfile);
  }, []);

  const refreshWallet = useCallback(async () => {
    const sessionToken = getStoredReaderToken();
    if (!sessionToken) {
      setWallet(null);
      setWalletLoading(false);
      setWalletError('');
      return null;
    }
    const requestSequence = ++walletRequestSequence.current;
    const canPublish = () => mountedRef.current
      && walletRequestSequence.current === requestSequence
      && getStoredReaderToken() === sessionToken;
    setWalletLoading(true);
    setWalletError('');
    try {
      const nextWallet = await getWallet();
      if (!canPublish()) return null;
      setWallet(nextWallet);
      return nextWallet;
    } catch (error) {
      if (canPublish()) {
        setWalletError(error instanceof Error && error.message ? error.message : '钱包加载失败');
      }
      return null;
    } finally {
      if (canPublish()) setWalletLoading(false);
    }
  }, []);

  useClientSessionEffect(() => {
    mountedRef.current = true;
    const syncSessionFromStorage = () => {
      const nextToken = getStoredReaderToken();
      const accountChanged = sessionTokenRef.current !== nextToken;
      if (accountChanged) {
        authActionSequence.current += 1;
        walletRequestSequence.current += 1;
      }
      sessionTokenRef.current = nextToken;
      setToken(nextToken);
      setProfile(getStoredReaderProfile());
      setSessionReady(true);
      setWalletLoading(false);
      if (accountChanged || !nextToken) {
        setWallet(null);
        setWalletError('');
      }
    };
    const syncFromNativeStorage = (event: StorageEvent) => {
      if (event.key === null || event.key === READER_TOKEN_KEY) syncSessionFromStorage();
      else if (event.key === READER_PROFILE_KEY) setProfile(getStoredReaderProfile());
    };
    if (deferClientSession) syncSessionFromStorage();
    window.addEventListener(READER_SESSION_CHANGED_EVENT, syncSessionFromStorage);
    window.addEventListener('storage', syncFromNativeStorage);
    return () => {
      mountedRef.current = false;
      authActionSequence.current += 1;
      walletRequestSequence.current += 1;
      window.removeEventListener(READER_SESSION_CHANGED_EVENT, syncSessionFromStorage);
      window.removeEventListener('storage', syncFromNativeStorage);
    };
  }, [deferClientSession]);

  const value = useMemo(
    () => ({
      token,
      profile,
      isAuthenticated: Boolean(token),
      sessionReady,
      wallet,
      walletLoading,
      walletError,
      login,
      register,
      clearSession,
      clearSessionIfCurrent,
      logout,
      refreshProfile,
      refreshWallet
    }),
    [clearSession, clearSessionIfCurrent, login, logout, profile, refreshProfile, refreshWallet, register, sessionReady, token, wallet, walletError, walletLoading]
  );

  return <ReaderAuthContext.Provider value={value}>{children}</ReaderAuthContext.Provider>;
}

export function useReaderAuth() {
  const context = useContext(ReaderAuthContext);
  if (!context) {
    throw new Error('useReaderAuth must be used within ReaderAuthProvider');
  }
  return context;
}
