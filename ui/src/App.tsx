import { useState } from 'react';
import { Activity, FileSearch, Cpu, BarChart3, Settings, Terminal } from 'lucide-react';
import Dashboard from './pages/Dashboard';
import Events from './pages/Events';
import Audit from './pages/Audit';
import Resources from './pages/Resources';
import StatusPage from './pages/StatusPage';

type Page = 'dashboard' | 'events' | 'audit' | 'resources' | 'status';

const navItems: { id: Page; label: string; icon: React.ReactNode }[] = [
  { id: 'dashboard', label: 'Dashboard', icon: <BarChart3 size={18} /> },
  { id: 'events', label: 'Events', icon: <Activity size={18} /> },
  { id: 'audit', label: 'Audit', icon: <FileSearch size={18} /> },
  { id: 'resources', label: 'Resources', icon: <Cpu size={18} /> },
  { id: 'status', label: 'Status', icon: <Settings size={18} /> },
];

export default function App() {
  const [page, setPage] = useState<Page>('dashboard');

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-logo">
          <img src="/logo.png" alt="Otacon" />
          <div className="sidebar-logo-text">
            <h1>OTACON</h1>
          </div>
        </div>

        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <div
              key={item.id}
              className={`nav-item ${page === item.id ? 'active' : ''}`}
              onClick={() => setPage(item.id)}
            >
              {item.icon}
              <span>{item.label}</span>
            </div>
          ))}
        </nav>

        <div className="sidebar-footer">
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Terminal size={12} />
            <span>v0.1.0</span>
          </div>
        </div>
      </aside>

      <main className="main-content">
        {page === 'dashboard' && <Dashboard />}
        {page === 'events' && <Events />}
        {page === 'audit' && <Audit />}
        {page === 'resources' && <Resources />}
        {page === 'status' && <StatusPage />}
      </main>
    </div>
  );
}
