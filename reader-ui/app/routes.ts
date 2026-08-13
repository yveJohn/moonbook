import { index, route, type RouteConfig } from '@react-router/dev/routes';

export default [
  index('./routes/home.tsx'),
  route('health', './routes/health.ts'),
  route('robots.txt', './routes/robots.ts'),
  route('sitemap.xml', './routes/sitemap.ts'),
  route('sitemap-books-:page.xml', './routes/sitemap-books.ts'),
  route('*', './routes/site.tsx')
] satisfies RouteConfig;
