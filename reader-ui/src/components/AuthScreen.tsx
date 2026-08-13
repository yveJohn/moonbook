import type { ReactNode } from 'react';
import { Card } from 'animal-island-ui';
import readerLogo from '../assets/reader-logo.png';

export type AuthFieldIconName = 'gift' | 'lock' | 'user';

export function AuthFieldIcon({ name }: { name: AuthFieldIconName }) {
  if (name === 'user') {
    return (
      <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M20 20a8 8 0 0 0-16 0" />
        <circle cx="12" cy="8" r="4" />
      </svg>
    );
  }

  if (name === 'gift') {
    return (
      <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M4 10h16v10H4z" />
        <path d="M12 10v10M3 7h18v3H3z" />
        <path d="M7.5 7a2.5 2.5 0 1 1 4.5-1.5V7M16.5 7A2.5 2.5 0 1 0 12 5.5V7" />
      </svg>
    );
  }

  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="5" y="10" width="14" height="10" rx="2" />
      <path d="M8 10V7a4 4 0 0 1 8 0v3" />
      <path d="M12 14.5v2" />
    </svg>
  );
}

interface AuthScreenProps {
  /** Short supporting line under the brand name. */
  subtitle: string;
  children: ReactNode;
}

/** Full-screen, mobile-first shell shared by the login and register pages. */
export function AuthScreen({ subtitle, children }: AuthScreenProps) {
  return (
    <main className="auth-page">
      <Card className="auth-card">
        <header className="auth-brand">
          <img className="auth-brand__logo" src={readerLogo} alt="月白书城" />
          <h1 className="auth-brand__name">月白书城</h1>
          <p className="auth-brand__tagline">{subtitle}</p>
        </header>
        {children}
      </Card>
    </main>
  );
}
