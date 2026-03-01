import { useState, useCallback } from 'react';
import { api, severityLabel, severityClass, gradeColor, progressClass, formatDate } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import { Download, ChevronDown, ChevronRight, Shield } from 'lucide-react';

export default function Audit() {
  const { data: audit, loading } = useFetch(
    useCallback(() => api.getLatestAudit(), []),
    30000
  );
  const [expandedCat, setExpandedCat] = useState<string | null>(null);

  if (loading) {
    return <div className="loading"><div className="spinner" />Loading audit report...</div>;
  }

  if (!audit) {
    return (
      <div className="empty-state">
        <Shield size={48} />
        <p style={{ marginTop: 16 }}>No audit reports yet</p>
        <p style={{ fontSize: 13, color: 'var(--text-muted)' }}>Run <code>otacon scan</code> to generate one</p>
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Audit Report</h1>
          <p className="page-subtitle">
            Cluster: {audit.clusterName} • Scanned: {formatDate(audit.scanTime)}
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn" onClick={() => api.exportAuditHTML()}>
            <Download size={14} /> Export HTML
          </button>
          <button className="btn" onClick={() => api.exportAuditPDF()}>
            <Download size={14} /> Export PDF
          </button>
        </div>
      </div>

      {/* Grade Card */}
      <div style={{ display: 'grid', gridTemplateColumns: '200px 1fr', gap: 16, marginBottom: 24 }}>
        <div className="card" style={{ textAlign: 'center', padding: 32 }}>
          <div className="grade-display" style={{ color: gradeColor(audit.overallScore) }}>
            {audit.grade}
          </div>
          <div className="grade-score">{audit.overallScore.toFixed(0)} / 100</div>
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'center', gap: 16, fontSize: 13 }}>
            <div>
              <div style={{ color: 'var(--red)', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{audit.totalCritical}</div>
              <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>Critical</div>
            </div>
            <div>
              <div style={{ color: 'var(--yellow)', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{audit.totalWarning}</div>
              <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>Warning</div>
            </div>
            <div>
              <div style={{ color: 'var(--accent)', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{audit.totalInfo}</div>
              <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>Info</div>
            </div>
          </div>
        </div>

        {/* Cluster Info */}
        <div className="card">
          <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>Cluster Overview</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16 }}>
            <div>
              <div style={{ fontSize: 24, fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{audit.nodeCount}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Nodes</div>
            </div>
            <div>
              <div style={{ fontSize: 24, fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{audit.podCount}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Pods</div>
            </div>
            <div>
              <div style={{ fontSize: 24, fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{audit.namespaceCount}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Namespaces</div>
            </div>
          </div>

          <div style={{ marginTop: 20 }}>
            {audit.categories.map((cat) => {
              const pct = cat.maxScore > 0 ? (cat.score / cat.maxScore) * 100 : 0;
              return (
                <div className="category-row" key={cat.name}>
                  <span className="category-name">{cat.name}</span>
                  <div className="category-bar">
                    <div className="progress-bar">
                      <div className={`progress-fill ${progressClass(pct)}`} style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                  <span className="category-pct">{pct.toFixed(0)}%</span>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* Findings by Category */}
      <div className="card">
        <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>
          Findings ({audit.totalFindings} total)
        </h3>

        {audit.categories.map((cat) => {
          const isExpanded = expandedCat === cat.name;
          const findingCount = cat.critical + cat.warning + cat.info;

          if (findingCount === 0) return null;

          return (
            <div key={cat.name} style={{ marginBottom: 4 }}>
              <div
                onClick={() => setExpandedCat(isExpanded ? null : cat.name)}
                style={{
                  display: 'flex', alignItems: 'center', gap: 8, padding: '10px 0',
                  cursor: 'pointer', borderBottom: '1px solid var(--bg-tertiary)'
                }}
              >
                {isExpanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                <span style={{ fontWeight: 600, fontSize: 14 }}>{cat.name}</span>
                <span style={{ color: 'var(--text-muted)', fontSize: 12, marginLeft: 'auto' }}>
                  {cat.critical > 0 && <span style={{ color: 'var(--red)', marginRight: 8 }}>{cat.critical} critical</span>}
                  {cat.warning > 0 && <span style={{ color: 'var(--yellow)', marginRight: 8 }}>{cat.warning} warning</span>}
                  {cat.info > 0 && <span style={{ color: 'var(--accent)' }}>{cat.info} info</span>}
                </span>
              </div>

              {isExpanded && cat.findings?.map((f, i) => (
                <div className="finding-row" key={i}>
                  <span className={`sev-badge ${severityClass(f.severity)}`}>
                    {severityLabel(f.severity)}
                  </span>
                  <div style={{ flex: 1 }}>
                    <div className="finding-message">{f.message}</div>
                    <div className="finding-resource">{f.resource}</div>
                    {f.remediation && (
                      <div style={{ marginTop: 4, fontSize: 12, color: 'var(--green)', fontFamily: 'var(--font-mono)' }}>
                        Fix: {f.remediation}
                      </div>
                    )}
                    {f.explain && (
                      <div style={{ marginTop: 4, fontSize: 12, color: 'var(--text-muted)' }}>
                        {f.explain}
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}
