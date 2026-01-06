import { useState, useEffect } from 'react';
import './style.css';

import { Layout } from './components/layout/Layout';
import { Home } from './components/home/Home';
import { History } from './components/history/History';
import { Settings } from './components/settings/Settings';
import { FileManager } from './components/files/FileManager';
import { Terminal } from './components/Terminal';
import { About } from './components/About';
import { applyAccentColor } from './hooks/useAccentColor';
import { setSoundEnabled } from './hooks/useSoundEffects';
import type { Page, AccentColor } from './types';
import * as Api from './lib/api';

function App() {
  const [activePage, setActivePage] = useState<Page>('home');

  // Apply saved settings on startup
  useEffect(() => {
    Api.GetConfig()
      .then((config) => {
        // Apply accent color
        if (config.accentColor) {
          applyAccentColor(config.accentColor as AccentColor);
        }
        // Set sound effects state
        setSoundEnabled(config.soundEffectsEnabled ?? true);
      })
      .catch(console.error);
  }, []);

  const renderPage = () => {
    switch (activePage) {
      case 'home':
        return <Home />;
      case 'history':
        return <History />;
      case 'settings':
        return <Settings />;
      case 'files':
        return <FileManager />;
      case 'terminal':
        return <Terminal />;
      case 'about':
        return <About />;
      default:
        return <Home />;
    }
  };

  return (
    <Layout activePage={activePage} onNavigate={setActivePage}>
      {renderPage()}
    </Layout>
  );
}

export default App;
