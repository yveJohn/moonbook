import { AxiosError, type AxiosAdapter, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import { http, unwrap } from './http';

const originalAdapter = http.defaults.adapter;
const readerProfile = JSON.stringify({ id: 'reader-1', nickname: 'Reader' });

function storeSession(token: string) {
  window.localStorage.setItem(READER_TOKEN_KEY, token);
  window.localStorage.setItem(READER_PROFILE_KEY, readerProfile);
}

function unauthorizedError(config: InternalAxiosRequestConfig) {
  const response = {
    config,
    data: { code: 401, msg: '未登录' },
    headers: {},
    status: 401,
    statusText: 'Unauthorized'
  } as AxiosResponse;

  return new AxiosError(
    'Request failed with status code 401',
    AxiosError.ERR_BAD_REQUEST,
    config,
    undefined,
    response
  );
}

function responseError(
  config: InternalAxiosRequestConfig,
  data: { code: number; msg: string }
) {
  const response = {
    config,
    data,
    headers: {},
    status: 400,
    statusText: 'Bad Request'
  } as AxiosResponse;

  return new AxiosError(
    'Request failed with status code 400',
    AxiosError.ERR_BAD_REQUEST,
    config,
    undefined,
    response
  );
}

describe('reader http client', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    http.defaults.adapter = originalAdapter;
    window.localStorage.clear();
  });

  it('sends reader token with Bearer authorization scheme', async () => {
    window.localStorage.setItem(READER_TOKEN_KEY, 'reader.jwt.token');
    let sentAuthorization: unknown;
    const adapter: AxiosAdapter = async (config) => {
      sentAuthorization = config.headers?.Authorization;
      return {
        config,
        data: { code: 200, data: true },
        headers: {},
        status: 200,
        statusText: 'OK'
      } as AxiosResponse;
    };
    http.defaults.adapter = adapter;

    await http.get('/reader/me/profile');

    expect(sentAuthorization).toBe('Bearer reader.jwt.token');
  });

  it('preserves an explicit authorization header when local storage has another token', async () => {
    window.localStorage.setItem(READER_TOKEN_KEY, 'token-B');
    let sentAuthorization: unknown;
    const adapter: AxiosAdapter = async (config) => {
      sentAuthorization = config.headers?.Authorization;
      return {
        config,
        data: { code: 200, data: true },
        headers: {},
        status: 200,
        statusText: 'OK'
      } as AxiosResponse;
    };
    http.defaults.adapter = adapter;

    await http.get('/reader/me/profile', {
      headers: { Authorization: 'Bearer token-A' }
    });

    expect(sentAuthorization).toBe('Bearer token-A');
  });

  it('preserves a newer session when an HTTP 401 arrives for an older explicit token', async () => {
    storeSession('token-A');
    http.defaults.adapter = async (config) => {
      storeSession('token-B');
      throw unauthorizedError(config);
    };

    await expect(http.get('/reader/me/profile', {
      headers: { Authorization: 'Bearer token-A' }
    })).rejects.toThrow('Request failed with status code 401');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBe('token-B');
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBe(readerProfile);
  });

  it('preserves a newer session when an application 401 arrives for an older explicit token', async () => {
    storeSession('token-A');
    http.defaults.adapter = async (config) => {
      storeSession('token-B');
      return {
        config,
        data: { code: 401, msg: '未登录' },
        headers: {},
        status: 200,
        statusText: 'OK'
      } as AxiosResponse;
    };

    const response = await http.get('/reader/me/profile', {
      headers: { Authorization: 'Bearer token-A' }
    });

    expect(() => unwrap(response)).toThrow('未登录');
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBe('token-B');
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBe(readerProfile);
  });

  it('clears the current session when its request receives an HTTP 401', async () => {
    storeSession('token-B');
    http.defaults.adapter = async (config) => {
      throw unauthorizedError(config);
    };

    await expect(http.get('/reader/me/profile')).rejects.toThrow('Request failed with status code 401');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
  });

  it('clears the current session when an HTTP error body reports unauthorized code', async () => {
    storeSession('token-B');
    http.defaults.adapter = async (config) => {
      throw responseError(config, { code: 401, msg: '请求失败' });
    };

    await expect(http.get('/reader/me/profile')).rejects.toThrow('Request failed with status code 400');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
  });

  it('clears the current session when an HTTP error body reports an expired login message', async () => {
    storeSession('token-B');
    http.defaults.adapter = async (config) => {
      throw responseError(config, { code: 400, msg: '登录已过期，请重新登录' });
    };

    await expect(http.get('/reader/me/profile')).rejects.toThrow('Request failed with status code 400');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
  });

  it('clears the current session when its request receives an application 401', async () => {
    storeSession('token-B');
    http.defaults.adapter = async (config) => ({
      config,
      data: { code: 401, msg: '未登录' },
      headers: {},
      status: 200,
      statusText: 'OK'
    } as AxiosResponse);

    const response = await http.get('/reader/me/profile');

    expect(() => unwrap(response)).toThrow('未登录');
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
  });

  it('does not clear a session created after a request without authorization', async () => {
    http.defaults.adapter = async (config) => {
      expect(config.headers.get('Authorization')).toBeUndefined();
      storeSession('token-B');
      throw unauthorizedError(config);
    };

    await expect(http.get('/reader/auth/profile')).rejects.toThrow('Request failed with status code 401');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBe('token-B');
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBe(readerProfile);
  });

  it('does not clear an existing session when an application 401 has no request config', () => {
    storeSession('token-B');

    expect(() => unwrap({
      data: { code: 401, msg: '未登录' }
    })).toThrow('未登录');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBe('token-B');
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBe(readerProfile);
  });

  it('reads a case-insensitive authorization header from plain response config headers', () => {
    storeSession('token-B');

    expect(() => unwrap({
      config: { headers: { authorization: 'Bearer token-B' } },
      data: { code: 401, msg: '未登录' }
    })).toThrow('未登录');

    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
  });
});
