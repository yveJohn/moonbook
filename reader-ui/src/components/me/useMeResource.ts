import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';

export interface MeResourceState<T> {
  data: T | null;
  loading: boolean;
  error: string;
  reload: () => Promise<T | null>;
  setData: (data: T) => void;
  setError: (message: string) => void;
}

interface InternalMeResourceState<T> {
  data: T | null;
  loading: boolean;
  error: string;
  identityKey: string;
}

export function useMeResource<T>(
  loader: () => Promise<T>,
  enabled: boolean,
  fallbackError: string,
  identityKey: string
): MeResourceState<T> {
  const requestSequence = useRef(0);
  const identityRef = useRef(identityKey);
  const loaderRef = useRef(loader);
  const fallbackErrorRef = useRef(fallbackError);
  loaderRef.current = loader;
  fallbackErrorRef.current = fallbackError;
  const [state, setState] = useState<InternalMeResourceState<T>>({
    data: null,
    loading: enabled,
    error: '',
    identityKey
  });

  const reload = useCallback(async () => {
    const identity = identityRef.current;
    const sequence = ++requestSequence.current;
    setState((current) => ({ ...current, loading: true, error: '', identityKey: identity }));
    try {
      const data = await loaderRef.current();
      if (requestSequence.current !== sequence || identityRef.current !== identity) return null;
      setState({ data, loading: false, error: '', identityKey: identity });
      return data;
    } catch (error) {
      if (requestSequence.current === sequence && identityRef.current === identity) {
        const message = error instanceof Error && error.message ? error.message : fallbackErrorRef.current;
        setState((current) => ({ ...current, loading: false, error: message, identityKey: identity }));
      }
      return null;
    }
  }, []);

  const setData = useCallback((data: T) => {
    requestSequence.current += 1;
    setState({ data, loading: false, error: '', identityKey: identityRef.current });
  }, []);

  const setError = useCallback((message: string) => {
    requestSequence.current += 1;
    setState((current) => ({ ...current, loading: false, error: message, identityKey: identityRef.current }));
  }, []);

  useLayoutEffect(() => {
    if (identityRef.current === identityKey) return;
    identityRef.current = identityKey;
    requestSequence.current += 1;
  }, [identityKey]);

  useEffect(() => {
    requestSequence.current += 1;
    setState({ data: null, loading: enabled, error: '', identityKey });
    if (!enabled) {
      return;
    }
    void reload();
    return () => {
      requestSequence.current += 1;
    };
  }, [enabled, identityKey, reload]);

  const visibleState = state.identityKey === identityKey
    ? state
    : { data: null, loading: enabled, error: '', identityKey };
  return {
    data: visibleState.data,
    loading: visibleState.loading,
    error: visibleState.error,
    reload,
    setData,
    setError
  };
}
