import { useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { AlertTriangle, CheckCircle, Info, TrendingUp, TrendingDown } from 'lucide-react';
import { api, gradeColor, severityIcon, formatDate } from '../api/client';
import { useFetch } from '../hooks/useFetch';

export default function Dashboard() {
  const { data: audit, loading: auditLoading } = useFetch(
    useCallback(() => api.getLatestAudit(), []),
    30000
  );
  const { data: incidents } = useFetch(
    useCallback(() => api.getIncidents('24h'), []),
    15000
  );
  const { data: events } = useFetch(
    useCallback(() => api.getEvents('1h'), []),
    10000
  );
  const { data: trend } = useFetch(
    useCallback(() => api.getAuditHistory('168h'), []),
    60000
  );

  if (auditLoading) {
    return <div className="loading"><div className="spinner" />Loading cluster data...</div>;
  }

  const healthyPct = audit
    ? ((audit.podCount - (audit.totalCritical || 0)) / Math.max(audit.podCount, 1) * 100).toFixed(1)
    : '—';

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-subtitle">
            Cluster: {audit?.clusterName || '—'} • Last scan: {audit ? formatDate(audit.scanTime) : '—'}
          </p>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="card-grid">
        <div className="stat-card">
          <div className="stat-label">Cluster Score</div>
          <div className="stat-value" style={{ color: audit ? gradeColor(audit.overallScore) : undefined }}>
            {audit?.grade || '—'}
          </div>
          <div className="stat-change">{audit ? `${audit.overallScore.toFixed(0)}/100` : ''}</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Critical Events</div>
          <div className="stat-value" style={{ color: 'var(--red)' }}>
            {events?.events?.filter(e => e.severity === 2).length || 0}
          </div>
          <div className="stat-change">Last hour</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Incidents</div>
          <div className="stat-value" style={{ color: 'var(--orange)' }}>
            {incidents?.total || 0}
          </div>
          <div className="stat-change">Last 24h</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Healthy Pods</div>
          <div className="stat-value" style={{ color: 'var(--green)' }}>
            {healthyPct}%
          </div>
          <div className="stat-change">{audit?.podCount || 0} total</div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
        {/* Score Trend Chart */}
        <div className="card">
          <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>Score Trend (7 days)</h3>
          {trend && trend.trend && trend.trend.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={trend.trend.map(t => ({ ...t, time: new Date(t.time).toLocaleDateString('en-GB', { day: '2-digit', month: 'short' }) }))}>
                <XAxis dataKey="time" stroke="var(--text-muted)" fontSize={11} />
                <YAxis domain={[0, 100]} stroke="var(--text-muted)" fontSize={11} />
                <Tooltip
                  contentStyle={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 6, fontSize: 12 }}
                  labelStyle={{ color: 'var(--text-primary)' }}
                />
                <Line type="monotone" dataKey="score" stroke="var(--accent)" strokeWidth={2} dot={{ r: 3 }} />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <div className="empty-state" style={{ padding: 40 }}>
              <TrendingUp size={32} color="var(--text-muted)" />
              <p>No trend data yet</p>
            </div>
          )}
        </div>

        {/* Category Breakdown */}
        <div className="card">
          <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>Category Scores</h3>
          {audit?.categories?.map((cat) => {
            const pct = cat.maxScore > 0 ? (cat.score / cat.maxScore) * 100 : 0;
            const barColor = pct >= 80 ? 'progress-green' : pct >= 60 ? 'progress-yellow' : 'progress-red';
            const findingCount = cat.critical + cat.warning + cat.info;
            return (
              <div className="category-row" key={cat.name}>
                <span className="category-name">{cat.name}</span>
                <div className="category-bar">
                  <div className="progress-bar">
                    <div className={`progress-fill ${barColor}`} style={{ width: `${pct}%` }} />
                  </div>
                </div>
                <span className="category-pct">{pct.toFixed(0)}%</span>
                <span className="category-count">
                  {findingCount > 0 ? `${findingCount} findings` : '✓ Clean'}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Recent Incidents */}
      <div className="card" style={{ marginBottom: 24 }}>
        <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>
          Recent Incidents
          <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}> — Last 24h</span>
        </h3>
        {incidents?.incidents && incidents.incidents.length > 0 ? (
          incidents.incidents.slice(0, 5).map((inc) => (
            <div
              key={inc.id}
              className={`incident-card ${inc.severity === 1 ? 'warning' : ''}`}
            >
              <div className="incident-title">
                {severityIcon(inc.severity)} {inc.title}
              </div>
              <div className="incident-meta">
                {inc.id} • {inc.events?.length || 0} events • {formatDate(inc.startTime)}
              </div>
              <div className="incident-impact">{inc.impact}</div>
            </div>
          ))
        ) : (
          <div className="empty-state" style={{ padding: 30 }}>
            <CheckCircle size={32} color="var(--green)" />
            <p style={{ marginTop: 8 }}>No incidents in the last 24 hours</p>
          </div>
        )}
      </div>
    </div>
  );
}
