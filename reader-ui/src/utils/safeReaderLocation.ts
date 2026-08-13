export interface SafeReaderLocation {
  pathname: string;
  search: string;
  hash: string;
  state?: unknown;
}

function isReaderPathname(pathname: string) {
  return pathname === '/'
    || pathname === '/books'
    || /^\/books\/\d+$/.test(pathname)
    || /^\/read\/\d+$/.test(pathname)
    || pathname === '/shelf'
    || pathname === '/me'
    || pathname === '/me/diamonds'
    || pathname === '/me/coins';
}

export function safeReaderLocation(value: unknown): SafeReaderLocation | null {
  if (typeof value === 'string') {
    if (!value.startsWith('/') || value.startsWith('//')) return null;
    const url = new URL(value, 'https://reader.moonbook.local');
    if (!isReaderPathname(url.pathname)) return null;
    return { pathname: url.pathname, search: url.search, hash: url.hash };
  }
  if (!value || typeof value !== 'object') return null;
  const candidate = value as { pathname?: unknown; search?: unknown; hash?: unknown; state?: unknown };
  if (typeof candidate.pathname !== 'string' || !isReaderPathname(candidate.pathname)) return null;
  return {
    pathname: candidate.pathname,
    search: typeof candidate.search === 'string' && (candidate.search === '' || candidate.search.startsWith('?')) ? candidate.search : '',
    hash: typeof candidate.hash === 'string' && (candidate.hash === '' || candidate.hash.startsWith('#')) ? candidate.hash : '',
    state: candidate.state
  };
}

export function resolveReaderReturnLocation(routeState: unknown, redirect: unknown): SafeReaderLocation {
  const from = routeState && typeof routeState === 'object'
    ? safeReaderLocation((routeState as { from?: unknown }).from)
    : null;
  return from ?? safeReaderLocation(redirect) ?? { pathname: '/me', search: '', hash: '' };
}

export function readerLocationHref(location: SafeReaderLocation) {
  return `${location.pathname}${location.search}${location.hash}`;
}
