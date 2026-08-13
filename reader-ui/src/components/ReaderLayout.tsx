import type { ReactNode } from 'react';

interface ReaderLayoutProps {
  children: ReactNode;
  activeTool?: ReaderDockTool | null;
  controlsVisible?: boolean;
  mode?: 'scroll' | 'page';
  title?: string;
  onBackToDetail?: () => void;
  onContentClick?: () => void;
  onOpenChapters: () => void;
  onOpenPalette: () => void;
  onOpenSettings: () => void;
}

type ReaderDockTool = 'chapters' | 'palette' | 'settings';

function ReaderDockButton({
  activeTool,
  icon,
  label,
  tool,
  onClick
}: {
  activeTool?: ReaderDockTool | null;
  icon: ReactNode;
  label: string;
  tool: ReaderDockTool;
  onClick: () => void;
}) {
  const isActive = activeTool === tool;
  return (
    <button
      type="button"
      className={isActive ? 'reader-dock-button is-active' : 'reader-dock-button'}
      aria-label={label}
      aria-pressed={isActive}
      onClick={onClick}
    >
      {icon}
    </button>
  );
}

export function ReaderLayout({
  children,
  activeTool = null,
  controlsVisible = true,
  mode = 'scroll',
  title,
  onBackToDetail,
  onContentClick,
  onOpenChapters,
  onOpenPalette,
  onOpenSettings
}: ReaderLayoutProps) {
  const showChrome = controlsVisible;

  return (
    <main className={`reader-view reader-view--${mode}`}>
      {showChrome ? (
        <header className="reader-view__reader-bar" role="banner" aria-label="阅读顶部栏">
          <button type="button" className="reader-view__back" aria-label="返回详情" onClick={onBackToDetail}>
            <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2.1" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M15 5l-7 7 7 7" />
            </svg>
          </button>
          <strong>{title}</strong>
          <span aria-hidden="true" />
        </header>
      ) : null}
      <article className="reader-view__content" onClick={onContentClick}>{children}</article>
      {showChrome ? (
        <nav className="reader-view__toolbar" aria-label="阅读操作">
          <ReaderDockButton
            activeTool={activeTool}
            label="目录"
            tool="chapters"
            icon={(
              <span className="reader-dock-icon reader-dock-icon--menu" aria-hidden="true">
                <span />
                <span />
                <span />
              </span>
            )}
            onClick={onOpenChapters}
          />
          <ReaderDockButton
            activeTool={activeTool}
            label="配色"
            tool="palette"
            icon={<span className="reader-dock-icon reader-dock-icon--palette" aria-hidden="true" />}
            onClick={onOpenPalette}
          />
          <ReaderDockButton
            activeTool={activeTool}
            label="设置"
            tool="settings"
            icon={<span className="reader-dock-icon reader-dock-icon--type" aria-hidden="true">A</span>}
            onClick={onOpenSettings}
          />
        </nav>
      ) : null}
    </main>
  );
}
