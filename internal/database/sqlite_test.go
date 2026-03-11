package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

func newTestDatabaseManager(t *testing.T) *DatabaseManager {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() {
		if err := dm.Close(); err != nil {
			t.Fatalf("failed to close test database: %v", err)
		}
	})
	return dm
}

func testEvent(id, service string, ts time.Time, attachments ...config.EventAttachment) *config.Event {
	return &config.Event{
		ID:          id,
		Service:     service,
		Type:        "message",
		Title:       "title-" + id,
		Content:     "content-" + id,
		Timestamp:   ts,
		Metadata:    `{"source":"test"}`,
		UserID:      "user-1",
		Attachments: attachments,
	}
}

func TestAttachmentRowIDIsDeterministicAndNormalizesWhitespace(t *testing.T) {
	got := attachmentRowID("event 1", "file 1")
	if got != "event_1__file_1" {
		t.Fatalf("unexpected attachment row id: %q", got)
	}

	if gotAgain := attachmentRowID("event 1", "file 1"); gotAgain != got {
		t.Fatalf("expected deterministic attachment row id, got %q and %q", got, gotAgain)
	}
}

func TestInsertEventAndGetEventsHydratesAttachments(t *testing.T) {
	dm := newTestDatabaseManager(t)
	base := time.Date(2026, 3, 1, 9, 30, 0, 0, time.FixedZone("JST", 9*60*60))

	event := testEvent(
		"event-1",
		"google_calendar",
		base,
		config.EventAttachment{FileID: "file-1", Title: "doc-1", MimeType: "application/vnd.google-apps.document", TextFull: "hello"},
		config.EventAttachment{FileID: "", Title: "skipped"},
		config.EventAttachment{FileID: "file-2", Title: "doc-2", ExportAs: "text/plain", Truncated: true},
	)

	if err := dm.InsertEvent(event); err != nil {
		t.Fatalf("InsertEvent returned error: %v", err)
	}

	events, err := dm.GetEvents(base.Add(-time.Hour), base.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetEvents returned error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.ID != event.ID {
		t.Fatalf("expected event id %q, got %q", event.ID, got.ID)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("expected 2 hydrated attachments, got %d", len(got.Attachments))
	}
	if got.Attachments[0].FileID != "file-1" {
		t.Fatalf("expected first attachment file-1, got %q", got.Attachments[0].FileID)
	}
	if got.Attachments[1].FileID != "file-2" {
		t.Fatalf("expected second attachment file-2, got %q", got.Attachments[1].FileID)
	}
	if !got.Attachments[1].Truncated {
		t.Fatalf("expected second attachment truncated flag to be restored")
	}
}

func TestInsertEventsAndFilterByTimeRangeAndService(t *testing.T) {
	dm := newTestDatabaseManager(t)
	base := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	events := []*config.Event{
		testEvent("event-1", "slack", base.Add(1*time.Hour)),
		testEvent("event-2", "github", base.Add(2*time.Hour)),
		testEvent("event-3", "slack", base.Add(3*time.Hour)),
	}

	if err := dm.InsertEvents(events); err != nil {
		t.Fatalf("InsertEvents returned error: %v", err)
	}

	filtered, err := dm.GetEvents(base.Add(90*time.Minute), base.Add(4*time.Hour), []string{"slack"})
	if err != nil {
		t.Fatalf("GetEvents returned error: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered event, got %d", len(filtered))
	}
	if filtered[0].ID != "event-3" {
		t.Fatalf("expected filtered event event-3, got %q", filtered[0].ID)
	}

	byService, err := dm.GetEventsByService("github", base, base.Add(4*time.Hour))
	if err != nil {
		t.Fatalf("GetEventsByService returned error: %v", err)
	}
	if len(byService) != 1 || byService[0].ID != "event-2" {
		t.Fatalf("expected github event event-2, got %+v", byService)
	}
}

func TestGetEventsReturnsSortedByTimestamp(t *testing.T) {
	dm := newTestDatabaseManager(t)
	base := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)

	if err := dm.InsertEvents([]*config.Event{
		testEvent("event-2", "github", base.Add(2*time.Hour)),
		testEvent("event-1", "github", base.Add(1*time.Hour)),
		testEvent("event-3", "github", base.Add(3*time.Hour)),
	}); err != nil {
		t.Fatalf("InsertEvents returned error: %v", err)
	}

	events, err := dm.GetEvents(base, base.Add(4*time.Hour), nil)
	if err != nil {
		t.Fatalf("GetEvents returned error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	for i := 0; i < len(events)-1; i++ {
		if events[i].Timestamp.After(events[i+1].Timestamp) {
			t.Fatalf("events are not sorted at index %d", i)
		}
	}
}
