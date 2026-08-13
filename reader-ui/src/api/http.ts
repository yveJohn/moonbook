import axios from 'axios';
import {
  READER_TOKEN_KEY,
  clearReaderSession,
  getStoredReaderToken
} from '../auth/session';

interface ApiBody<T> {
  code?: number;
  msg?: string;
  data?: T;
}

interface PageBody<T> {
  code?: number;
  msg?: string;
  rows?: T[];
  total?: number;
}

export const QUOTE_CHANGED_CODE = 46106;

export class ApiError extends Error {
  constructor(public readonly code: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export function isApiErrorCode(error: unknown, code: number): error is ApiError {
  return error instanceof ApiError && error.code === code;
}

export const http = axios.create({
  baseURL: import.meta.env.VITE_READER_API_BASE || '/dev-api',
  timeout: 15000
});

function isUnauthorized(code?: number, msg?: string) {
  return code === 401 || Boolean(msg && (msg.includes('未登录') || msg.includes('登录已过期')));
}

function getAuthorizationHeader(headers: unknown): unknown {
  if (!headers || typeof headers !== 'object') {
    return undefined;
  }

  const getHeader = (headers as { get?: unknown }).get;
  if (typeof getHeader === 'function') {
    const value = getHeader.call(headers, 'Authorization');
    if (value != null) {
      return value;
    }
  }

  const entry = Object.entries(headers).find(([name]) => name.toLowerCase() === 'authorization');
  return entry?.[1];
}

function getRequestBearerToken(config: unknown): string {
  if (!config || typeof config !== 'object') {
    return '';
  }

  const header = getAuthorizationHeader((config as { headers?: unknown }).headers);
  if (typeof header !== 'string') {
    return '';
  }

  return /^Bearer\s+(\S+)$/i.exec(header)?.[1] || '';
}

function clearRequestSessionIfCurrent(config: unknown) {
  const requestToken = getRequestBearerToken(config);
  if (requestToken && requestToken === getStoredReaderToken()) {
    clearReaderSession();
  }
}

http.interceptors.request.use((config) => {
  if (typeof window === 'undefined') return config;
  const token = window.localStorage.getItem(READER_TOKEN_KEY);
  if (token && !config.headers.has('Authorization')) {
    config.headers.set('Authorization', `Bearer ${token}`);
  }
  return config;
});

http.interceptors.response.use(
  (response) => response,
  (error) => {
    const body = error?.response?.data as ApiBody<unknown> | undefined;
    if (error?.response?.status === 401 || isUnauthorized(body?.code, body?.msg)) {
      clearRequestSessionIfCurrent(error?.config ?? error?.response?.config);
    }
    return Promise.reject(error);
  }
);

interface ResponseWithConfig<T> {
  data: T;
  config?: { headers?: unknown };
}

export function unwrap<T>(response: ResponseWithConfig<ApiBody<T>>): T {
  const body = response.data;
  if (body.code && body.code !== 200) {
    if (isUnauthorized(body.code, body.msg)) {
      clearRequestSessionIfCurrent(response.config);
    }
    throw new ApiError(body.code, body.msg || '请求失败');
  }
  return body.data as T;
}

export function unwrapPage<T>(response: ResponseWithConfig<PageBody<T>>) {
  const body = response.data;
  if (body.code && body.code !== 200) {
    if (isUnauthorized(body.code, body.msg)) {
      clearRequestSessionIfCurrent(response.config);
    }
    throw new ApiError(body.code, body.msg || '请求失败');
  }
  return {
    rows: body.rows || [],
    total: body.total || 0
  };
}
