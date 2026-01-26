// Package database provides SQLite persistence for collected events.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iriam/worklogr/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

// DatabaseManager handles SQLite database operations
type DatabaseManager struct {
	db *sql.DB
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	manager := &DatabaseManager{db: db}
	if err := manager.CreateTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return manager, nil
}

// CreateTables creates the necessary database tables
func (dm *DatabaseManager) CreateTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		service TEXT NOT NULL,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT,
		timestamp DATETIME NOT NULL,
		metadata TEXT,
		user_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_events_service ON events(service);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
	CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);

	CREATE TABLE IF NOT EXISTS event_attachments (
		id TEXT PRIMARY KEY,
		event_id TEXT NOT NULL,
		file_id TEXT NOT NULL,
		title TEXT,
		mime_type TEXT,
		export_as TEXT,
		text_full TEXT,
		truncated INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_event_attachments_event_id ON event_attachments(event_id);
	`

	if _, err := dm.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

func attachmentRowID(eventID, fileID string) string {
	// Use a deterministic, sqlite-safe identifier.
	// Avoid extremely long hashes; event IDs here are already stable.
	// Replace whitespace just in case.
	return strings.ReplaceAll(eventID+"__"+fileID, " ", "_")
}

// InsertEvent inserts a new event into the database
func (dm *DatabaseManager) InsertEvent(event *config.Event) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO events (id, service, type, title, content, timestamp, metadata, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.Exec(
		event.ID,
		event.Service,
		event.Type,
		event.Title,
		event.Content,
		event.Timestamp,
		event.Metadata,
		event.UserID,
	); err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	if err := dm.insertAttachmentsTx(tx, event); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// InsertEvents inserts multiple events in a transaction
func (dm *DatabaseManager) InsertEvents(events []*config.Event) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO events (id, service, type, title, content, timestamp, metadata, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		_, err := stmt.Exec(
			event.ID,
			event.Service,
			event.Type,
			event.Title,
			event.Content,
			event.Timestamp,
			event.Metadata,
			event.UserID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert event %s: %w", event.ID, err)
		}

		if err := dm.insertAttachmentsTx(tx, event); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (dm *DatabaseManager) insertAttachmentsTx(tx *sql.Tx, event *config.Event) error {
	if event == nil || len(event.Attachments) == 0 {
		return nil
	}

	attachStmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO event_attachments
		(id, event_id, file_id, title, mime_type, export_as, text_full, truncated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare attachment statement: %w", err)
	}
	defer attachStmt.Close()

	for _, a := range event.Attachments {
		if a.FileID == "" {
			continue
		}
		truncated := 0
		if a.Truncated {
			truncated = 1
		}
		if _, err := attachStmt.Exec(
			attachmentRowID(event.ID, a.FileID),
			event.ID,
			a.FileID,
			a.Title,
			a.MimeType,
			a.ExportAs,
			a.TextFull,
			truncated,
		); err != nil {
			return fmt.Errorf("failed to insert attachment for event %s: %w", event.ID, err)
		}
	}

	return nil
}

// GetEvents retrieves events within a time range
func (dm *DatabaseManager) GetEvents(startTime, endTime time.Time, services []string) ([]*config.Event, error) {
	// Convert times to UTC for consistent database comparison
	startTimeUTC := startTime.UTC()
	endTimeUTC := endTime.UTC()
	
	query := `
	SELECT id, service, type, title, content, timestamp, metadata, user_id
	FROM events
	WHERE datetime(timestamp) >= datetime(?) AND datetime(timestamp) <= datetime(?)
	`
	args := []interface{}{startTimeUTC.Format("2006-01-02 15:04:05"), endTimeUTC.Format("2006-01-02 15:04:05")}

	if len(services) > 0 {
		placeholders := ""
		for i, service := range services {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, service)
		}
		query += " AND service IN (" + placeholders + ")"
	}

	query += " ORDER BY timestamp ASC"

	rows, err := dm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*config.Event
	for rows.Next() {
		event := &config.Event{}
		err := rows.Scan(
			&event.ID,
			&event.Service,
			&event.Type,
			&event.Title,
			&event.Content,
			&event.Timestamp,
			&event.Metadata,
			&event.UserID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Hydrate attachments from separate table
	if err := dm.populateAttachments(events); err != nil {
		return nil, err
	}

	return events, nil
}

func (dm *DatabaseManager) populateAttachments(events []*config.Event) error {
	if len(events) == 0 {
		return nil
	}

	ids := make([]string, 0, len(events))
	eventByID := make(map[string]*config.Event, len(events))
	for _, e := range events {
		if e == nil || e.ID == "" {
			continue
		}
		ids = append(ids, e.ID)
		eventByID[e.ID] = e
	}
	if len(ids) == 0 {
		return nil
	}

	// SQLite has a default max variable number (commonly 999). Chunk to be safe.
	const chunkSize = 900
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		placeholders := make([]string, 0, len(chunk))
		args := make([]interface{}, 0, len(chunk))
		for _, id := range chunk {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}

		q := `
			SELECT event_id, file_id, title, mime_type, export_as, text_full, truncated
			FROM event_attachments
			WHERE event_id IN (` + strings.Join(placeholders, ",") + `)
			ORDER BY created_at ASC
		`

		rows, err := dm.db.Query(q, args...)
		if err != nil {
			return fmt.Errorf("failed to query event attachments: %w", err)
		}
		for rows.Next() {
			var eventID, fileID, title, mimeType, exportAs, textFull string
			var truncatedInt int
			if err := rows.Scan(&eventID, &fileID, &title, &mimeType, &exportAs, &textFull, &truncatedInt); err != nil {
				rows.Close()
				return fmt.Errorf("failed to scan attachment row: %w", err)
			}

			e := eventByID[eventID]
			if e == nil {
				continue
			}
			e.Attachments = append(e.Attachments, config.EventAttachment{
				FileID:    fileID,
				Title:     title,
				MimeType:  mimeType,
				ExportAs:  exportAs,
				TextFull:  textFull,
				Truncated: truncatedInt != 0,
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("error iterating attachment rows: %w", err)
		}
		rows.Close()
	}

	return nil
}

// GetEventsByService retrieves events for a specific service within a time range
func (dm *DatabaseManager) GetEventsByService(service string, startTime, endTime time.Time) ([]*config.Event, error) {
	return dm.GetEvents(startTime, endTime, []string{service})
}

// DeleteOldEvents deletes events older than the specified duration
func (dm *DatabaseManager) DeleteOldEvents(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	query := "DELETE FROM events WHERE timestamp < ?"

	result, err := dm.db.Exec(query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete old events: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	fmt.Printf("Deleted %d old events\n", rowsAffected)
	return nil
}

// Close closes the database connection
func (dm *DatabaseManager) Close() error {
	if dm.db != nil {
		return dm.db.Close()
	}
	return nil
}

// GetStats returns basic statistics about stored events
func (dm *DatabaseManager) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	// Total events
	var total int
	err := dm.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get total events: %w", err)
	}
	stats["total"] = total

	// Events by service
	rows, err := dm.db.Query("SELECT service, COUNT(*) FROM events GROUP BY service")
	if err != nil {
		return nil, fmt.Errorf("failed to get events by service: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var service string
		var count int
		if err := rows.Scan(&service, &count); err != nil {
			return nil, fmt.Errorf("failed to scan service stats: %w", err)
		}
		stats[service] = count
	}

	return stats, nil
}
