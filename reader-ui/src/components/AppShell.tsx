import type { ReactNode } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';

type TabKey = 'home' | 'books' | 'shelf' | 'me';

interface AppShellProps {
  /** Highlighted bottom tab. Omit on sub-pages that are not a tab destination. */
  active?: TabKey;
  /** Page title shown in the top bar. When omitted the brand mark is shown instead. */
  title?: string;
  /** Show a back chevron in the top bar instead of the brand mark. */
  back?: boolean;
  /** Custom back behavior for pages that need to skip synthetic history entries. */
  onBack?: () => void;
  /** Extra controls rendered on the right side of the top bar. */
  rightSlot?: ReactNode;
  /** When provided, replaces the bottom tab bar (e.g. a sub-page action bar). */
  footer?: ReactNode;
  /** Omit the main tab bar on focused sub-pages. */
  hideTabs?: boolean;
  children: ReactNode;
}

const HomeIcon = () => (
  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M3 10.5 12 3l9 7.5" />
    <path d="M5 9.5V20a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V9.5" />
    <path d="M9.5 21v-6h5v6" />
  </svg>
);

const BooksIcon = () => (
  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M4 5.5A1.5 1.5 0 0 1 5.5 4H11v16H5.5A1.5 1.5 0 0 1 4 18.5Z" />
    <path d="M13 4h5.5A1.5 1.5 0 0 1 20 5.5v13a1.5 1.5 0 0 1-1.5 1.5H13Z" />
    <path d="M7 8h1M7 11h1M16 8h1M16 11h1" />
  </svg>
);

const ShelfIcon = () => (
  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M5 4h11a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3Z" />
    <path d="M8 4v13a3 3 0 0 0 3 3" />
    <path d="M9 8h6" />
  </svg>
);

const MeIcon = () => (
  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <circle cx="12" cy="8" r="3.5" />
    <path d="M5 20c0-3.6 3.1-6 7-6s7 2.4 7 6" />
  </svg>
);

const tabs: { key: TabKey; to: string; label: string; icon: ReactNode }[] = [
  { key: 'home', to: '/', label: '首页', icon: <HomeIcon /> },
  { key: 'books', to: '/books', label: '书库', icon: <BooksIcon /> },
  { key: 'shelf', to: '/shelf', label: '书架', icon: <ShelfIcon /> },
  { key: 'me', to: '/me', label: '我的', icon: <MeIcon /> }
];

export function AppShell({ active, title, back, onBack, rightSlot, footer, hideTabs, children }: AppShellProps) {
  const navigate = useNavigate();

  return (
    <div className="app-shell">
      <header className="app-bar">
        <div className="app-bar__left">
          {back ? (
            <button type="button" className="app-bar__back" aria-label="返回" onClick={onBack || (() => navigate(-1))}>
              <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M15 5l-7 7 7 7" />
              </svg>
            </button>
          ) : null}
        </div>
        {title ? <h1 className="app-bar__title">{title}</h1> : null}
        <div className="app-bar__right">{rightSlot}</div>
      </header>

      <div className="app-shell__body">{children}</div>

      {footer ? (
        <div className="app-foot">{footer}</div>
      ) : hideTabs ? null : (
        <nav className="tab-bar" aria-label="主导航">
          {tabs.map((tab) => (
            <NavLink
              key={tab.key}
              to={tab.to}
              className={active === tab.key ? 'tab-bar__item is-active' : 'tab-bar__item'}
            >
              <span className="tab-bar__icon">{tab.icon}</span>
              <span className="tab-bar__label">{tab.label}</span>
            </NavLink>
          ))}
        </nav>
      )}
    </div>
  );
}
