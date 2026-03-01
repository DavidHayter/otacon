const BASE_URL = '/api/v1';

export interface Event {
  id: string;
  type: string;
  reason: string;
  message: string;
  namespace: string;
  resourceKind: string;
  resourceName: string;
  nodeName: string;
  severity: number; // 0=Info, 1=Warning, 2=Critical
  count: number;
  firstSeen: string;
  lastSeen: string;
  correlationId?: string;
}

export interface Finding {
  id: string;
  category: string;
  rule: string;
  severity: number;
  resource: string;
  namespace: string;
  kind: string;
  message: string;
  remediation: string;
  explain: string;
}

export interface CategoryScore {
  name: string;
  score: number;
  maxScore: number;
  weight: number;
  findings: Finding[];
  critical: number;
  warning: number;
  info: number;
}

export interface Scorecard {
  clusterName: string;
  scanTime: string;
  overallScore: number;
  grade: string;
  categories: CategoryScore[];
  totalFindings: number;
  totalCritical: number;
  totalWarning: number;
  totalInfo: number;
  namespaceCount: number;
  podCount: number;
  nodeCount: number;
}

export interface Incident {
  id: string;
  title: string;
  rootCause: string;
  severity: number;
  events: Event[];
  impact: string;
  suggestion: string;
  startTime: string;
  endTime: string;
}

export interface DedupGroup {
  key: string;
  reason: string;
  namespace: string;
  severity: number;
  count: number;
  affectedPods: string[];
  affectedNodes: string[];
  firstSeen: string;
  lastSeen: string;
  sampleMessage: string;
}

export interface Stats {
  database: Record<string, number>;
  cooldown: { TotalReceived: number; TotalPassed: number; TotalSuppressed: number };
  router: { TotalReceived: number; TotalRouted: number; TotalErrors: number; ByChannel: Record<string, number> };
}

export interface ScoreTrend {
  time: string;
  score: number;
  grade: string;
}

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(BASE_URL + url);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export const api = {
  getEvents: (since = '1h', namespace = '') =>
    fetchJSON<{ events: Event[]; total: number }>(
      `/events?since=${since}${namespace ? `&namespace=${namespace}` : ''}`
    ),

  getIncidents: (since = '24h') =>
    fetchJSON<{ incidents: Incident[]; total: number }>(`/incidents?since=${since}`),

  getLatestAudit: () =>
    fetchJSON<Scorecard>('/audit/reports/latest'),

  getAuditHistory: (since = '168h') =>
    fetchJSON<{ trend: ScoreTrend[]; total: number }>(`/audit/history?since=${since}`),

  getDedupGroups: () =>
    fetchJSON<{ groups: DedupGroup[]; total: number }>('/dedup/groups'),

  getStats: () =>
    fetchJSON<Stats>('/stats'),

  getStatus: () =>
    fetchJSON<{ status: string; mode: string; version: string }>('/status'),

  exportAuditHTML: () =>
    window.open(BASE_URL + '/audit/reports/latest?format=html', '_blank'),

  exportAuditPDF: () =>
    window.open(BASE_URL + '/audit/reports/latest?format=pdf', '_blank'),
};

// Severity helpers
export const severityLabel = (sev: number) => ['INFO', 'WARNING', 'CRITICAL'][sev] || 'UNKNOWN';
export const severityIcon = (sev: number) => ['🔵', '🟡', '🔴'][sev] || '⚪';
export const severityClass = (sev: number) => ['sev-info', 'sev-warning', 'sev-critical'][sev] || '';
export const gradeColor = (score: number) => score >= 85 ? 'var(--green)' : score >= 70 ? 'var(--yellow)' : 'var(--red)';
export const progressClass = (pct: number) => pct >= 80 ? 'progress-green' : pct >= 60 ? 'progress-yellow' : 'progress-red';
export const formatTime = (iso: string) => new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
export const formatDate = (iso: string) => new Date(iso).toLocaleDateString('en-GB', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
