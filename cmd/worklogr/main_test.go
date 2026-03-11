package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCommandTestConfig(t *testing.T, timezone string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	body := []byte("timezone: \"" + timezone + "\"\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatalf("failed to write command test config: %v", err)
	}
	return path
}

func TestParseTimeRangeRejectsStartAfterEnd(t *testing.T) {
	configPath := writeCommandTestConfig(t, "UTC")

	_, _, err := parseTimeRange("2026-03-06 12:00:00", "2026-03-06 11:00:00", configPath)
	if err == nil {
		t.Fatalf("expected parseTimeRange to reject start after end")
	}
}

func TestParseTimeStringUsesConfiguredTimezone(t *testing.T) {
	configPath := writeCommandTestConfig(t, "UTC")

	got, err := parseTimeString("2026-03-06 12:34:56", configPath)
	if err != nil {
		t.Fatalf("parseTimeString returned error: %v", err)
	}

	expected := time.Date(2026, 3, 6, 12, 34, 56, 0, time.UTC)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestParseTimeStringFallsBackToDefaultTimezoneWhenConfigLoadFails(t *testing.T) {
	configPath := t.TempDir()

	got, err := parseTimeString("2026-03-06 12:34:56", configPath)
	if err != nil {
		t.Fatalf("parseTimeString returned error: %v", err)
	}

	jst := time.FixedZone("JST", 9*60*60)
	expected := time.Date(2026, 3, 6, 12, 34, 56, 0, jst)
	if got.UTC() != expected.UTC() {
		t.Fatalf("expected JST fallback time %v, got %v", expected, got)
	}
}

func TestParseAdjustedTimeRangeExpandsDateOnlyEnd(t *testing.T) {
	configPath := writeCommandTestConfig(t, "UTC")
	nowFunc = func() time.Time {
		return time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() {
		nowFunc = time.Now
	})

	startTime, endTime, err := parseAdjustedTimeRange("2026-03-06", "2026-03-07", configPath)
	if err != nil {
		t.Fatalf("parseAdjustedTimeRange returned error: %v", err)
	}

	expectedStart := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	expectedEnd := time.Date(2026, 3, 7, 23, 59, 59, 0, time.UTC)
	if !startTime.Equal(expectedStart) {
		t.Fatalf("expected start %v, got %v", expectedStart, startTime)
	}
	if !endTime.Equal(expectedEnd) {
		t.Fatalf("expected end %v, got %v", expectedEnd, endTime)
	}
}

func TestAdjustInclusiveEndTimeCapsFutureDateOnlyAtNow(t *testing.T) {
	now := time.Date(2026, 3, 7, 10, 30, 0, 0, time.UTC)
	endTime := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)

	got := adjustInclusiveEndTime(endTime, now)
	if !got.Equal(now) {
		t.Fatalf("expected adjusted end time to cap at now %v, got %v", now, got)
	}
}

func TestCommandBuildersKeepFlagStateIndependent(t *testing.T) {
	rootOptions := &rootOptions{}
	collectCmd := newCollectCmd(rootOptions)
	exportCmd := newExportCmd(rootOptions)

	if err := collectCmd.ParseFlags([]string{"--start", "2026-03-01", "--end", "2026-03-02", "--services", "slack"}); err != nil {
		t.Fatalf("collect ParseFlags returned error: %v", err)
	}

	if got := exportCmd.Flags().Lookup("start").Value.String(); got != "" {
		t.Fatalf("expected export start flag to remain empty, got %q", got)
	}
	if got := exportCmd.Flags().Lookup("services").Value.String(); got != "[]" {
		t.Fatalf("expected export services flag to remain default, got %q", got)
	}

	if err := exportCmd.ParseFlags([]string{"--start", "2026-03-03", "--end", "2026-03-04", "--format", "csv"}); err != nil {
		t.Fatalf("export ParseFlags returned error: %v", err)
	}

	if got := collectCmd.Flags().Lookup("format"); got != nil {
		t.Fatalf("collect command should not have export-only format flag")
	}
	if got := exportCmd.Flags().Lookup("format").Value.String(); got != "csv" {
		t.Fatalf("expected export format flag to be csv, got %q", got)
	}
}

func TestNewRootCmdWiresExpectedSubcommands(t *testing.T) {
	cmd := newRootCmd()

	for _, subcommand := range []string{"gcloud", "collect", "export", "status", "config"} {
		if _, _, err := cmd.Find([]string{subcommand}); err != nil {
			t.Fatalf("expected root command to include %q: %v", subcommand, err)
		}
	}

	if got := cmd.PersistentFlags().Lookup("config"); got == nil {
		t.Fatalf("expected root command to have persistent config flag")
	}
}
