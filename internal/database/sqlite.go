package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	`

	if _, err := dm.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

// InsertEvent inserts a new event into the database
func (dm *DatabaseManager) InsertEvent(event *config.Event) error {
	query := `
	INSERT OR REPLACE INTO events (id, service, type, title, content, timestamp, metadata, user_id)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := dm.db.Exec(query,
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
		return fmt.Errorf("failed to insert event: %w", err)
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
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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

	return events, nil
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
