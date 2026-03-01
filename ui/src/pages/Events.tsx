import { useState, useCallback } from 'react';
import { api, severityIcon, severityLabel, severityClass, formatTime, Event } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import { RefreshCw, Filter } from 'lucide-react';

export default function Events() {
  const [since, setSince] = useState('1h');
  const [sevFilter, setSevFilter] = useState<number | null>(null);

  const { data, loading, refetch } = useFetch(
    useCallback(() => api.getEvents(since), [since]),
    10000
  );

  const { data: dedupData } = useFetch(
    useCallback(() => api.getDedupGroups(), []),
    15000
  );

  const events = data?.events || [];
  const filtered = sevFilter !== null
    ? events.filter(e => e.severity === sevFilter)
    : events;

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Events</h1>
          <p className="page-subtitle">{filtered.length} events {sevFilter !== null ? `(filtered)` : ''}</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <select className="select" value={since} onChange={e => setSince(e.target.value)}>
            <option value="30m">Last 30 min</option>
            <option value="1h">Last 1 hour</option>
            <option value="6h">Last 6 hours</option>
            <option value="24h">Last 24 hours</option>
          </select>
          <select
            className="select"
            value={sevFilter ?? ''}
            onChange={e => setSevFilter(e.target.value ? Number(e.target.value) : null)}
          >
            <option value="">All severities</option>
            <option value="2">Critical</option>
            <option value="1">Warning</option>
            <option value="0">Info</option>
          </select>
          <button className="btn btn-sm" onClick={refetch}>
            <RefreshCw size={14} /> Refresh
          </button>
        </div>
      </div>

      {/* Dedup Groups */}
      {dedupData?.groups && dedupData.groups.length > 0 && (
        <div className="card" style={{ marginBottom: 16 }}>
          <h3 style={{ fontSize: 13, marginBottom: 12, color: 'var(--text-secondary)' }}>
            <Filter size={14} style={{ marginRight: 6, verticalAlign: -2 }} />
            Active Event Groups (Deduplicated)
          </h3>
          {dedupData.groups.slice(0, 5).map((g, i) => (
            <div key={i} style={{ padding: '8px 0', borderBottom: '1px solid var(--bg-tertiary)', fontSize: 13 }}>
              <span>{severityIcon(g.severity)}</span>{' '}
              <strong>{g.reason}</strong> in {g.namespace}:{' '}
              <span style={{ color: 'var(--accent)', fontFamily: 'var(--font-mono)' }}>{g.count}x</span>{' '}
              <span style={{ color: 'var(--text-muted)' }}>
                ({g.affectedPods?.length || 0} pods
                {g.affectedNodes?.length ? `, ${g.affectedNodes.length} nodes` : ''})
              </span>
            </div>
          ))}
        </div>
      )}

      {/* Event Timeline */}
      <div className="card">
        {loading ? (
          <div className="loading"><div className="spinner" />Loading events...</div>
        ) : filtered.length === 0 ? (
          <div className="empty-state">
            <p>No events found for the selected period</p>
          </div>
        ) : (
          filtered.map((event, i) => (
            <EventRow key={`${event.id}-${i}`} event={event} />
          ))
        )}
      </div>
    </div>
  );
}

function EventRow({ event }: { event: Event }) {
  return (
    <div className="event-item">
      <span className="event-time">{formatTime(event.lastSeen)}</span>
      <span className="event-icon">{severityIcon(event.severity)}</span>
      <div style={{ flex: 1 }}>
        <span className="event-reason">[{event.reason}]</span>
        <span className="event-msg">{event.message}</span>
        <div style={{ marginTop: 2 }}>
          <span className="event-ns">{event.namespace}/{event.resourceName}</span>
          {event.count > 1 && (
            <span style={{ marginLeft: 8, fontSize: 11, color: 'var(--text-muted)' }}>×{event.count}</span>
          )}
          {event.nodeName && (
            <span style={{ marginLeft: 8, fontSize: 11, color: 'var(--text-muted)' }}>node: {event.nodeName}</span>
          )}
        </div>
      </div>
      <span className={`sev-badge ${severityClass(event.severity)}`}>{severityLabel(event.severity)}</span>
    </div>
  );
}
