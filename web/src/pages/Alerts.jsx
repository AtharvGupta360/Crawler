import { useState } from 'react';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { alerts as alertsApi, notifications as notifApi } from '../api/client';
import TagsInput from '../components/TagsInput';

export default function Alerts() {
  const toast = useToast();
  const [tab, setTab] = useState('alerts');
  const [showForm, setShowForm] = useState(false);

  // Alerts
  const { data: alertsData, loading: alertsLoading, refetch: refetchAlerts } = useApi(
    () => alertsApi.list(),
    []
  );

  // Notifications
  const [notifOffset, setNotifOffset] = useState(0);
  const { data: notifData, loading: notifLoading, refetch: refetchNotifs } = useApi(
    () => notifApi.list({ limit: 20, offset: notifOffset }),
    [notifOffset]
  );

  const alertsList = alertsData?.alerts || [];
  const notifList = notifData?.notifications || [];
  const unreadCount = notifData?.unread || 0;

  // Create alert form state
  const [formName, setFormName] = useState('');
  const [formSkills, setFormSkills] = useState([]);
  const [formSeniority, setFormSeniority] = useState([]);
  const [formLocationType, setFormLocationType] = useState([]);
  const [formKeyword, setFormKeyword] = useState('');

  const handleCreateAlert = async () => {
    if (!formName.trim()) {
      toast.warning('Missing name', 'Please give your alert a name');
      return;
    }

    const filters = {};
    if (formSkills.length > 0) filters.skills = formSkills;
    if (formSeniority.length > 0) filters.seniority = formSeniority;
    if (formLocationType.length > 0) filters.location_type = formLocationType;
    if (formKeyword.trim()) filters.keyword = formKeyword.trim();

    try {
      await alertsApi.create({
        name: formName,
        filters,
        is_active: true,
        notify_via: 'websocket',
      });
      toast.success('Alert created', `"${formName}" will notify you when matching jobs are found`);
      setShowForm(false);
      setFormName('');
      setFormSkills([]);
      setFormSeniority([]);
      setFormLocationType([]);
      setFormKeyword('');
      refetchAlerts();
    } catch (err) {
      toast.error('Failed to create alert', err.message);
    }
  };

  const handleToggleAlert = async (alert) => {
    try {
      await alertsApi.update(alert.id, {
        ...alert,
        is_active: !alert.is_active,
      });
      refetchAlerts();
    } catch (err) {
      toast.error('Update failed', err.message);
    }
  };

  const handleDeleteAlert = async (id) => {
    try {
      await alertsApi.delete(id);
      toast.success('Alert deleted');
      refetchAlerts();
    } catch (err) {
      toast.error('Delete failed', err.message);
    }
  };

  const handleMarkAllRead = async () => {
    try {
      await notifApi.markAllRead();
      toast.success('All read', 'All notifications marked as read');
      refetchNotifs();
    } catch (err) {
      toast.error('Failed', err.message);
    }
  };

  const handleMarkRead = async (id) => {
    try {
      await notifApi.markRead(id);
      refetchNotifs();
    } catch (err) {
      // silent
    }
  };

  const timeAgo = (dateStr) => {
    if (!dateStr) return '';
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.floor(hrs / 24);
    return `${days}d ago`;
  };

  return (
    <div className="animate-fade-up">
      <div className="page-header">
        <div>
          <h1 className="page-title">Alerts & Notifications</h1>
          <p className="page-subtitle">Get notified when jobs match your criteria</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowForm(true)}>
          + New Alert
        </button>
      </div>

      {/* Tabs */}
      <div className="tabs">
        <button className={`tab ${tab === 'alerts' ? 'active' : ''}`} onClick={() => setTab('alerts')}>
          Alerts ({alertsList.length})
        </button>
        <button className={`tab ${tab === 'notifications' ? 'active' : ''}`} onClick={() => setTab('notifications')}>
          Notifications {unreadCount > 0 && `(${unreadCount} unread)`}
        </button>
      </div>

      {/* Alerts Tab */}
      {tab === 'alerts' && (
        <div>
          {alertsLoading ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {[1, 2, 3].map((i) => (
                <div key={i} className="skeleton skeleton-card" style={{ height: 80 }} />
              ))}
            </div>
          ) : alertsList.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {alertsList.map((alert) => (
                <div key={alert.id} className="alert-card">
                  <div className="alert-card-body">
                    <div className="alert-card-name">
                      {alert.name}
                      {!alert.is_active && (
                        <span style={{ fontSize: 11, color: 'var(--text-tertiary)', marginLeft: 8 }}>
                          (paused)
                        </span>
                      )}
                    </div>
                    <div className="alert-card-filters">
                      {alert.filters?.skills?.map((s, i) => (
                        <span key={i} className="badge badge-skill">{s}</span>
                      ))}
                      {alert.filters?.seniority?.map((s, i) => (
                        <span key={i} className="badge badge-seniority">{s}</span>
                      ))}
                      {alert.filters?.location_type?.map((s, i) => (
                        <span key={i} className="badge badge-location">{s}</span>
                      ))}
                      {alert.filters?.keyword && (
                        <span className="badge badge-type">"{alert.filters.keyword}"</span>
                      )}
                    </div>
                    {alert.last_triggered && (
                      <div style={{ fontSize: 11, color: 'var(--text-tertiary)', marginTop: 4 }}>
                        Last triggered {timeAgo(alert.last_triggered)}
                      </div>
                    )}
                  </div>

                  <div className="alert-card-actions">
                    <label className="toggle">
                      <input
                        type="checkbox"
                        checked={alert.is_active}
                        onChange={() => handleToggleAlert(alert)}
                      />
                      <span className="toggle-slider" />
                    </label>
                    <button
                      className="btn btn-danger btn-sm"
                      onClick={() => handleDeleteAlert(alert.id)}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">🔔</div>
              <div className="empty-state-title">No alerts yet</div>
              <div className="empty-state-text">
                Create an alert to get notified when matching jobs are found.
              </div>
            </div>
          )}
        </div>
      )}

      {/* Notifications Tab */}
      {tab === 'notifications' && (
        <div>
          {unreadCount > 0 && (
            <div style={{ marginBottom: 16 }}>
              <button className="btn btn-ghost btn-sm" onClick={handleMarkAllRead}>
                Mark all as read
              </button>
            </div>
          )}

          {notifLoading ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {[1, 2, 3].map((i) => (
                <div key={i} className="skeleton" style={{ height: 60, borderRadius: 'var(--radius)' }} />
              ))}
            </div>
          ) : notifList.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {notifList.map((n) => (
                <div
                  key={n.id}
                  className={`notification-item ${!n.is_read ? 'unread' : ''}`}
                  onClick={() => !n.is_read && handleMarkRead(n.id)}
                >
                  {!n.is_read && <div className="notification-dot" />}
                  <div className="notification-body">
                    <div className="notification-title">{n.title}</div>
                    <div className="notification-subtitle">
                      {n.company}
                      {n.apply_url && (
                        <>
                          {' · '}
                          <a href={n.apply_url} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                            Apply →
                          </a>
                        </>
                      )}
                    </div>
                  </div>
                  <span className="notification-time">{timeAgo(n.created_at)}</span>
                </div>
              ))}
            </div>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">📬</div>
              <div className="empty-state-title">No notifications</div>
              <div className="empty-state-text">
                You'll see notifications here when jobs match your alerts.
              </div>
            </div>
          )}
        </div>
      )}

      {/* Create Alert Modal */}
      {showForm && (
        <div className="modal-overlay" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2 className="modal-title">Create Alert</h2>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div className="input-group">
                <label>Alert Name</label>
                <input
                  className="input"
                  placeholder="e.g. Remote Go Backend Jobs"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  autoFocus
                />
              </div>

              <div className="input-group">
                <label>Skills</label>
                <TagsInput
                  value={formSkills}
                  onChange={setFormSkills}
                  placeholder="e.g. Go, Kubernetes"
                />
              </div>

              <div className="input-group">
                <label>Seniority</label>
                <TagsInput
                  value={formSeniority}
                  onChange={setFormSeniority}
                  placeholder="e.g. junior, mid"
                />
              </div>

              <div className="input-group">
                <label>Location Type</label>
                <TagsInput
                  value={formLocationType}
                  onChange={setFormLocationType}
                  placeholder="e.g. remote, hybrid"
                />
              </div>

              <div className="input-group">
                <label>Keyword</label>
                <input
                  className="input"
                  placeholder="e.g. machine learning"
                  value={formKeyword}
                  onChange={(e) => setFormKeyword(e.target.value)}
                />
              </div>
            </div>

            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={() => setShowForm(false)}>
                Cancel
              </button>
              <button className="btn btn-primary" onClick={handleCreateAlert}>
                Create Alert
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
