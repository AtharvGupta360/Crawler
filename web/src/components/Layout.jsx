import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { useApi } from '../hooks/useApi';
import { useWebSocket } from '../hooks/useWebSocket';
import { useToast } from '../hooks/useToast';
import { notifications as notifApi } from '../api/client';
import { useState, useCallback } from 'react';

const NAV_ITEMS = [
  { section: 'Overview', items: [
    { to: '/', icon: '◈', label: 'Dashboard' },
    { to: '/jobs', icon: '💼', label: 'Jobs' },
    { to: '/search', icon: '🔍', label: 'Search' },
  ]},
  { section: 'Intelligence', items: [
    { to: '/trends', icon: '📊', label: 'Trends' },
    { to: '/match', icon: '🎯', label: 'Resume Match' },
  ]},
  { section: 'Personal', items: [
    { to: '/alerts', icon: '🔔', label: 'Alerts' },
    { to: '/profile', icon: '👤', label: 'Profile' },
  ]},
];

const PAGE_TITLES = {
  '/': 'Dashboard',
  '/jobs': 'Job Listings',
  '/search': 'Search Jobs',
  '/trends': 'Trend Analytics',
  '/match': 'Resume Match',
  '/alerts': 'Alerts & Notifications',
  '/profile': 'My Profile',
};

export default function Layout({ children }) {
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const toast = useToast();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const { data: notifData, refetch: refetchNotifs } = useApi(
    () => notifApi.list({ limit: 5 }),
    []
  );

  const unreadCount = notifData?.unread || 0;

  const handleWsMessage = useCallback((msg) => {
    if (msg.type === 'job_alert') {
      toast.info('New Job Match', `${msg.title} at ${msg.company}`);
      refetchNotifs();
    }
  }, [toast, refetchNotifs]);

  useWebSocket(handleWsMessage, !!user);

  const pageTitle = PAGE_TITLES[location.pathname] ||
    (location.pathname.startsWith('/jobs/') ? 'Job Details' : 'JobCrawl');

  const initials = user?.name
    ? user.name.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)
    : user?.email?.[0]?.toUpperCase() || '?';

  return (
    <div className="app-layout">
      <aside className={`sidebar ${sidebarOpen ? 'open' : ''}`}>
        <div className="sidebar-logo">
          <div className="sidebar-logo-icon">JC</div>
          <span className="sidebar-logo-text">JobCrawl</span>
        </div>

        <nav className="sidebar-nav">
          {NAV_ITEMS.map(({ section, items }) => (
            <div key={section} className="sidebar-section">
              <div className="sidebar-section-label">{section}</div>
              {items.map(({ to, icon, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  end={to === '/'}
                  className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
                  onClick={() => setSidebarOpen(false)}
                >
                  <span className="nav-icon">{icon}</span>
                  {label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
      </aside>

      <main className="main-content">
        <header className="topbar">
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <button
              className="btn btn-ghost btn-icon"
              style={{ display: 'none' }}
              onClick={() => setSidebarOpen(!sidebarOpen)}
            >
              ☰
            </button>
            <h1 className="topbar-title">{pageTitle}</h1>
          </div>

          <div className="topbar-actions">
            <button
              className={`notification-bell ${unreadCount > 0 ? 'has-unread' : ''}`}
              onClick={() => navigate('/alerts')}
              title="Notifications"
            >
              🔔
              {unreadCount > 0 && (
                <span className="notification-badge">
                  {unreadCount > 99 ? '99+' : unreadCount}
                </span>
              )}
            </button>

            <div className="user-menu" onClick={() => navigate('/profile')}>
              <div className="user-avatar">{initials}</div>
              <span className="user-name">{user?.name || user?.email}</span>
            </div>

            <button className="btn btn-ghost btn-sm" onClick={logout}>
              Logout
            </button>
          </div>
        </header>

        <div className="page-content">
          {children}
        </div>
      </main>
    </div>
  );
}
