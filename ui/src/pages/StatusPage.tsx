import { useCallback } from 'react';
import { api } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import { Server, Database, Bell, Shield } from 'lucide-react';

export default function StatusPage() {
  const { data: stats, loading } = useFetch(
    useCallback(() => api.getStats(), []),
    10000
  );
  const { data: status } = useFetch(
    useCallback(() => api.getStatus(), []),
    10000
  );

  if (loading) {
    return <div className="loading"><div className="spinner" />Loading status...</div>;
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">System Status</h1>
          <p className="page-subtitle">Otacon guardian mode health and statistics</p>
        </div>
      </div>

      {/* System Info */}
      <div className="card-grid">
        <div className="stat-card">
          <div className="stat-label"><Server size={12} style={{ marginRight: 4, verticalAlign: -1 }} /> Status</div>
          <div className="stat-value" style={{ color: 'var(--green)', fontSize: 20 }}>
            {status?.status || 'Unknown'}
          </div>
          <div className="stat-change">Mode: {status?.mode || '—'} • v{status?.version || '—'}</div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
        {/* Database Stats */}
        <div className="card">
          <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>
            <Database size={14} style={{ marginRight: 6, verticalAlign: -2 }} />
            Storage
          </h3>
          {stats?.database && Object.entries(stats.database).map(([table, count]) => (
            <div key={table} style={{
              display: 'flex', justifyContent: 'space-between', padding: '8px 0',
              borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13
            }}>
              <span style={{ fontFamily: 'var(--font-mono)' }}>{table}</span>
              <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--accent)' }}>{count.toLocaleString()}</span>
            </div>
          ))}
        </div>

        {/* Notification Stats */}
        <div className="card">
          <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>
            <Bell size={14} style={{ marginRight: 6, verticalAlign: -2 }} />
            Notifications
          </h3>
          {stats?.router && (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13 }}>
                <span>Total Received</span>
                <span style={{ fontFamily: 'var(--font-mono)' }}>{stats.router.TotalReceived}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13 }}>
                <span>Routed</span>
                <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--green)' }}>{stats.router.TotalRouted}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13 }}>
                <span>Errors</span>
                <span style={{ fontFamily: 'var(--font-mono)', color: stats.router.TotalErrors > 0 ? 'var(--red)' : 'var(--text-muted)' }}>{stats.router.TotalErrors}</span>
              </div>
              {stats.router.ByChannel && Object.entries(stats.router.ByChannel).map(([ch, count]) => (
                <div key={ch} style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13 }}>
                  <span style={{ fontFamily: 'var(--font-mono)' }}>→ {ch}</span>
                  <span style={{ fontFamily: 'var(--font-mono)' }}>{count}</span>
                </div>
              ))}
            </>
          )}

          <h3 style={{ fontSize: 14, marginTop: 20, marginBottom: 12, color: 'var(--text-secondary)' }}>
            <Shield size={14} style={{ marginRight: 6, verticalAlign: -2 }} />
            Cooldown
          </h3>
          {stats?.cooldown && (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13 }}>
                <span>Passed Through</span>
                <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--green)' }}>{stats.cooldown.TotalPassed}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', fontSize: 13 }}>
                <span>Suppressed</span>
                <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--yellow)' }}>{stats.cooldown.TotalSuppressed}</span>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
