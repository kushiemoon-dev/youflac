import { useState } from 'react';
import './style.css';

import { Layout } from './components/layout/Layout';
import { Home } from './components/home/Home';
import { Settings } from './components/settings/Settings';
import { FileManager } from './components/files/FileManager';
import { Terminal } from './components/Terminal';
import { About } from './components/About';
import type { Page } from './types';

function App() {
  const [activePage, setActivePage] = useState<Page>('home');

  const renderPage = () => {
    switch (activePage) {
      case 'home':
        return <Home />;
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
