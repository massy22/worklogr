package collector

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iriam/worklogr/internal/config"
)

type mockServiceClient struct {
	events [](*config.Event)
	err    error
	delay  time.Duration
	called int32
}

func (m *mockServiceClient) CollectEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	atomic.AddInt32(&m.called, 1)

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.err != nil {
		return nil, m.err
	}

	return m.events, nil
}

func makeEvent(id string, ts time.Time) *config.Event {
	return &config.Event{
		ID:        id,
		Service:   "test",
		Type:      "test",
		Title:     "title-" + id,
		Content:   "content-" + id,
		Timestamp: ts,
	}
}

func TestCollectEventsParallelSortedByTimestamp(t *testing.T) {
	base := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)

	slowService := &mockServiceClient{
		delay: 80 * time.Millisecond,
		events: []*config.Event{
			makeEvent("slow-2", base.Add(3*time.Minute)),
			makeEvent("slow-1", base.Add(1*time.Minute)),
		},
	}
	fastService := &mockServiceClient{
		delay: 10 * time.Millisecond,
		events: []*config.Event{
			makeEvent("fast-2", base.Add(4*time.Minute)),
			makeEvent("fast-1", base.Add(2*time.Minute)),
		},
	}

	ec := &EventCollector{
		services: map[string]ServiceClient{
			"slow": slowService,
			"fast": fastService,
		},
	}

	events, err := ec.CollectEvents(base, base.Add(24*time.Hour), nil)
	if err != nil {
		t.Fatalf("CollectEvents returned error: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	for i := 0; i < len(events)-1; i++ {
		if events[i].Timestamp.After(events[i+1].Timestamp) {
			t.Fatalf("events are not sorted at index %d", i)
		}
	}
}

func TestCollectEventsKeepsPartialSuccessOnServiceError(t *testing.T) {
	base := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)

	okService := &mockServiceClient{
		events: []*config.Event{
			makeEvent("ok-1", base.Add(1*time.Minute)),
			makeEvent("ok-2", base.Add(2*time.Minute)),
		},
	}
	failedService := &mockServiceClient{
		err: fmt.Errorf("upstream failed"),
	}

	ec := &EventCollector{
		services: map[string]ServiceClient{
			"ok":     okService,
			"failed": failedService,
		},
	}

	events, err := ec.CollectEvents(base, base.Add(24*time.Hour), nil)
	if err != nil {
		t.Fatalf("CollectEvents should keep partial success, got error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events from successful service, got %d", len(events))
	}
}

func TestCollectEventsRespectsServiceFilter(t *testing.T) {
	base := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)

	slackService := &mockServiceClient{
		events: []*config.Event{
			makeEvent("slack-1", base.Add(1*time.Minute)),
		},
	}
	githubService := &mockServiceClient{
		events: []*config.Event{
			makeEvent("github-1", base.Add(2*time.Minute)),
		},
	}

	ec := &EventCollector{
		services: map[string]ServiceClient{
			"slack":  slackService,
			"github": githubService,
		},
	}

	events, err := ec.CollectEvents(base, base.Add(24*time.Hour), []string{"slack"})
	if err != nil {
		t.Fatalf("CollectEvents returned error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event from filtered service, got %d", len(events))
	}
	if got := atomic.LoadInt32(&slackService.called); got != 1 {
		t.Fatalf("expected slack service called once, got %d", got)
	}
	if got := atomic.LoadInt32(&githubService.called); got != 0 {
		t.Fatalf("expected github service not called, got %d", got)
	}
}
