import { useCallback } from 'react';
import { api } from '../api/client';
import { useFetch } from '../hooks/useFetch';
import { Cpu } from 'lucide-react';

export default function Resources() {
  const { data: audit, loading } = useFetch(
    useCallback(() => api.getLatestAudit(), []),
    30000
  );

  if (loading) {
    return <div className="loading"><div className="spinner" />Loading resource data...</div>;
  }

  // Extract resource-related findings
  const resourceFindings = audit?.categories
    ?.find(c => c.name === 'Resource Management')?.findings || [];

  const noLimits = resourceFindings.filter(f =>
    f.rule === 'memory-limits-defined' || f.rule === 'cpu-limits-defined'
  );
  const noRequests = resourceFindings.filter(f =>
    f.rule === 'memory-requests-defined' || f.rule === 'cpu-requests-defined'
  );
  const noQuotas = resourceFindings.filter(f => f.rule === 'namespace-resource-quota');

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Resources</h1>
          <p className="page-subtitle">Resource configuration analysis and right-sizing</p>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="card-grid">
        <div className="stat-card">
          <div className="stat-label">Missing Limits</div>
          <div className="stat-value" style={{ color: noLimits.length > 0 ? 'var(--red)' : 'var(--green)' }}>
            {noLimits.length}
          </div>
          <div className="stat-change">containers without CPU/Memory limits</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Missing Requests</div>
          <div className="stat-value" style={{ color: noRequests.length > 0 ? 'var(--yellow)' : 'var(--green)' }}>
            {noRequests.length}
          </div>
          <div className="stat-change">containers without CPU/Memory requests</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Missing Quotas</div>
          <div className="stat-value" style={{ color: noQuotas.length > 0 ? 'var(--yellow)' : 'var(--green)' }}>
            {noQuotas.length}
          </div>
          <div className="stat-change">namespaces without ResourceQuota</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Total Issues</div>
          <div className="stat-value" style={{ color: 'var(--accent)' }}>
            {resourceFindings.length}
          </div>
          <div className="stat-change">resource management findings</div>
        </div>
      </div>

      {/* Findings Table */}
      <div className="card">
        <h3 style={{ fontSize: 14, marginBottom: 16, color: 'var(--text-secondary)' }}>
          <Cpu size={14} style={{ marginRight: 6, verticalAlign: -2 }} />
          Resource Findings
        </h3>

        {resourceFindings.length === 0 ? (
          <div className="empty-state" style={{ padding: 40 }}>
            <p style={{ color: 'var(--green)' }}>All resources properly configured ✓</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Severity</th>
                  <th>Rule</th>
                  <th>Resource</th>
                  <th>Message</th>
                  <th>Fix</th>
                </tr>
              </thead>
              <tbody>
                {resourceFindings.map((f, i) => {
                  const sevClass = f.severity === 2 ? 'sev-critical' : f.severity === 1 ? 'sev-warning' : 'sev-info';
                  const sevLabel = ['INFO', 'WARN', 'CRIT'][f.severity];
                  return (
                    <tr key={i}>
                      <td><span className={`sev-badge ${sevClass}`}>{sevLabel}</span></td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-muted)' }}>{f.rule}</td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{f.resource}</td>
                      <td style={{ fontSize: 13 }}>{f.message}</td>
                      <td style={{ fontSize: 12, color: 'var(--green)', fontFamily: 'var(--font-mono)' }}>{f.remediation}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
