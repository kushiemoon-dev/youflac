import { ReactNode } from 'react';
import { Sidebar } from './Sidebar';
import type { Page } from '../../types';

interface LayoutProps {
  children: ReactNode;
  activePage: Page;
  onNavigate: (page: Page) => void;
}

export function Layout({ children, activePage, onNavigate }: LayoutProps) {
  return (
    <div className="flex min-h-screen w-full">
      <Sidebar activePage={activePage} onNavigate={onNavigate} />
      <main
        className="flex-1 overflow-y-auto relative"
        style={{ background: 'var(--color-bg-primary)' }}
      >
        {/* Subtle gradient overlay at top */}
        <div
          className="absolute top-0 left-0 right-0 h-32 pointer-events-none z-0"
          style={{
            background: 'linear-gradient(to bottom, rgba(244, 114, 182, 0.03), transparent)'
          }}
        />
        <div className="relative z-10">
          {children}
        </div>
      </main>
    </div>
  );
}
