import {
  isRouteErrorResponse,
  Links,
  type LinksFunction,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration
} from 'react-router';
import 'animal-island-ui/style';
import '../src/styles/global.css';
import { ReaderAuthProvider } from '../src/auth/ReaderAuthContext';

export const links: LinksFunction = () => [
  { rel: 'icon', type: 'image/png', href: '/favicon.png?v=2' },
  { rel: 'apple-touch-icon', sizes: '180x180', href: '/apple-touch-icon.png?v=2' }
];

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function Root() {
  return (
    <ReaderAuthProvider deferClientSession>
      <Outlet />
    </ReaderAuthProvider>
  );
}

export function ErrorBoundary({ error }: { error: unknown }) {
  const status = isRouteErrorResponse(error) ? error.status : 500;
  const message = status === 404 ? '页面不存在' : '页面暂时无法访问';
  return (
    <main className="reader-page reader-page--narrow">
      <title>{message} - 月白书城</title>
      <meta name="robots" content="noindex,nofollow" />
      <h1 className="page-title">{message}</h1>
      <p className="notice notice--error">请稍后重试或返回首页。</p>
      <a href="/">返回首页</a>
    </main>
  );
}
