package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/intelligence"
	_ "modernc.org/sqlite"
)

// Config configures the storage layer
type Config struct {
	DSN              string        // SQLite path (default: otacon.db)
	EventRetention   time.Duration // Default: 7 days
	AuditRetention   time.Duration // Default: 7 days
	DigestRetention  time.Duration // Default: 30 days
	CleanupInterval  time.Duration // Default: 1 hour
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		DSN:             "otacon.db",
		EventRetention:  7 * 24 * time.Hour,
		AuditRetention:  7 * 24 * time.Hour,
		DigestRetention: 30 * 24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
}

// Store manages persistent storage
type Store struct {
	db     *sql.DB
	config Config
	mu     sync.RWMutex
}

// New creates a new store
func New(config Config) (*Store, error) {
	db, err := sql.Open("sqlite", config.DSN+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db, config: config}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return store, nil
}

// Close closes the database
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			reason TEXT NOT NULL,
			message TEXT,
			namespace TEXT,
			resource_kind TEXT,
			resource_name TEXT,
			node_name TEXT,
			severity INTEGER,
			count INTEGER DEFAULT 1,
			first_seen DATETIME,
			last_seen DATETIME,
			correlation_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_namespace ON events(namespace)`,
		`CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_events_last_seen ON events(last_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_events_reason ON events(reason)`,

		`CREATE TABLE IF NOT EXISTS incidents (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			root_cause TEXT,
			severity INTEGER,
			impact TEXT,
			suggestion TEXT,
			events_json TEXT,
			start_time DATETIME,
			end_time DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_incidents_start_time ON incidents(start_time)`,

		`CREATE TABLE IF NOT EXISTS audit_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_name TEXT,
			overall_score REAL,
			grade TEXT,
			scorecard_json TEXT,
			scan_time DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_scan_time ON audit_reports(scan_time)`,

		`CREATE TABLE IF NOT EXISTS digests (
			id TEXT PRIMARY KEY,
			type TEXT,
			digest_json TEXT,
			period_start DATETIME,
			period_end DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digests_period ON digests(period_start, period_end)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	log.Println("[store] Database migrations complete")
	return nil
}

// ============================================================
// EVENTS
// ============================================================

// SaveEvent persists an event
func (s *Store) SaveEvent(event engine.Event) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO events (id, type, reason, message, namespace, resource_kind, resource_name, node_name, severity, count, first_seen, last_seen, correlation_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.Type, event.Reason, event.Message,
		event.Namespace, event.ResourceKind, event.ResourceName, event.NodeName,
		int(event.Severity), event.Count,
		event.FirstSeen, event.LastSeen, event.CorrelationID,
	)
	return err
}

// GetEvents retrieves events with optional filters
func (s *Store) GetEvents(since time.Duration, namespace string, severity *engine.Severity) ([]engine.Event, error) {
	query := `SELECT id, type, reason, message, namespace, resource_kind, resource_name, node_name, severity, count, first_seen, last_seen, correlation_id
	          FROM events WHERE last_seen > ?`
	args := []interface{}{time.Now().Add(-since)}

	if namespace != "" {
		query += " AND namespace = ?"
		args = append(args, namespace)
	}
	if severity != nil {
		query += " AND severity = ?"
		args = append(args, int(*severity))
	}
	query += " ORDER BY last_seen DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []engine.Event
	for rows.Next() {
		var e engine.Event
		var sev int
		err := rows.Scan(&e.ID, &e.Type, &e.Reason, &e.Message, &e.Namespace,
			&e.ResourceKind, &e.ResourceName, &e.NodeName, &sev, &e.Count,
			&e.FirstSeen, &e.LastSeen, &e.CorrelationID)
		if err != nil {
			continue
		}
		e.Severity = engine.Severity(sev)
		events = append(events, e)
	}
	return events, nil
}

// ============================================================
// INCIDENTS
// ============================================================

// SaveIncident persists a correlated incident
func (s *Store) SaveIncident(incident engine.CorrelatedIncident) error {
	eventsJSON, _ := json.Marshal(incident.Events)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO incidents (id, title, root_cause, severity, impact, suggestion, events_json, start_time, end_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		incident.ID, incident.Title, incident.RootCause, int(incident.Severity),
		incident.Impact, incident.Suggestion, string(eventsJSON),
		incident.StartTime, incident.EndTime,
	)
	return err
}

// GetIncidents retrieves recent incidents
func (s *Store) GetIncidents(since time.Duration) ([]engine.CorrelatedIncident, error) {
	rows, err := s.db.Query(
		`SELECT id, title, root_cause, severity, impact, suggestion, events_json, start_time, end_time
		 FROM incidents WHERE start_time > ? ORDER BY start_time DESC`,
		time.Now().Add(-since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []engine.CorrelatedIncident
	for rows.Next() {
		var inc engine.CorrelatedIncident
		var sev int
		var eventsJSON string
		err := rows.Scan(&inc.ID, &inc.Title, &inc.RootCause, &sev,
			&inc.Impact, &inc.Suggestion, &eventsJSON,
			&inc.StartTime, &inc.EndTime)
		if err != nil {
			continue
		}
		inc.Severity = engine.Severity(sev)
		json.Unmarshal([]byte(eventsJSON), &inc.Events)
		incidents = append(incidents, inc)
	}
	return incidents, nil
}

// ============================================================
// AUDIT REPORTS
// ============================================================

// SaveAuditReport persists an audit scorecard
func (s *Store) SaveAuditReport(scorecard *engine.Scorecard) error {
	scorecardJSON, _ := json.Marshal(scorecard)
	_, err := s.db.Exec(
		`INSERT INTO audit_reports (cluster_name, overall_score, grade, scorecard_json, scan_time)
		 VALUES (?, ?, ?, ?, ?)`,
		scorecard.ClusterName, scorecard.OverallScore, scorecard.Grade,
		string(scorecardJSON), scorecard.ScanTime,
	)
	return err
}

// GetLatestAuditReport retrieves the most recent audit report
func (s *Store) GetLatestAuditReport() (*engine.Scorecard, error) {
	row := s.db.QueryRow(
		`SELECT scorecard_json FROM audit_reports ORDER BY scan_time DESC LIMIT 1`,
	)

	var scorecardJSON string
	if err := row.Scan(&scorecardJSON); err != nil {
		return nil, err
	}

	var scorecard engine.Scorecard
	json.Unmarshal([]byte(scorecardJSON), &scorecard)
	return &scorecard, nil
}

// GetAuditHistory retrieves audit score history
func (s *Store) GetAuditHistory(since time.Duration) ([]engine.Scorecard, error) {
	rows, err := s.db.Query(
		`SELECT scorecard_json FROM audit_reports WHERE scan_time > ? ORDER BY scan_time ASC`,
		time.Now().Add(-since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []engine.Scorecard
	for rows.Next() {
		var scorecardJSON string
		if err := rows.Scan(&scorecardJSON); err != nil {
			continue
		}
		var sc engine.Scorecard
		json.Unmarshal([]byte(scorecardJSON), &sc)
		reports = append(reports, sc)
	}
	return reports, nil
}

// ============================================================
// DIGESTS
// ============================================================

// SaveDigest persists a digest report
func (s *Store) SaveDigest(digest *intelligence.Digest) error {
	digestJSON, _ := json.Marshal(digest)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO digests (id, type, digest_json, period_start, period_end)
		 VALUES (?, ?, ?, ?, ?)`,
		digest.ID, string(digest.Type), string(digestJSON),
		digest.PeriodStart, digest.PeriodEnd,
	)
	return err
}

// ============================================================
// RETENTION & CLEANUP
// ============================================================

// StartRetentionWorker begins periodic cleanup of old data
func (s *Store) StartRetentionWorker(ctx context.Context) {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	log.Printf("[store] Retention worker started (events: %s, audits: %s, digests: %s)",
		s.config.EventRetention, s.config.AuditRetention, s.config.DigestRetention)

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-ctx.Done():
			log.Println("[store] Retention worker stopped")
			return
		}
	}
}

func (s *Store) cleanup() {
	now := time.Now()

	// Clean events
	result, _ := s.db.Exec("DELETE FROM events WHERE created_at < ?",
		now.Add(-s.config.EventRetention))
	if result != nil {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("[store] Cleaned %d expired events", rows)
		}
	}

	// Clean incidents
	result, _ = s.db.Exec("DELETE FROM incidents WHERE created_at < ?",
		now.Add(-s.config.EventRetention))
	if result != nil {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("[store] Cleaned %d expired incidents", rows)
		}
	}

	// Clean audit reports
	result, _ = s.db.Exec("DELETE FROM audit_reports WHERE created_at < ?",
		now.Add(-s.config.AuditRetention))
	if result != nil {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("[store] Cleaned %d expired audit reports", rows)
		}
	}

	// Clean digests
	result, _ = s.db.Exec("DELETE FROM digests WHERE created_at < ?",
		now.Add(-s.config.DigestRetention))
	if result != nil {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("[store] Cleaned %d expired digests", rows)
		}
	}
}

// Stats returns storage statistics
func (s *Store) Stats() map[string]int64 {
	stats := make(map[string]int64)
	tables := []string{"events", "incidents", "audit_reports", "digests"}

	for _, table := range tables {
		var count int64
		row := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
		row.Scan(&count)
		stats[table] = count
	}

	return stats
}
