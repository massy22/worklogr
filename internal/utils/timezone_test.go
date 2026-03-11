package utils

import (
	"testing"
	"time"
)

func TestParseTimeInTimezoneSupportsAbsoluteAndRelativeFormats(t *testing.T) {
	tm, err := NewTimezoneManager("Asia/Tokyo")
	if err != nil {
		t.Fatalf("NewTimezoneManager returned error: %v", err)
	}

	absolute, err := tm.ParseTimeInTimezone("2026-03-06 12:34:56")
	if err != nil {
		t.Fatalf("ParseTimeInTimezone returned error for absolute time: %v", err)
	}
	expectedAbsolute := time.Date(2026, 3, 6, 12, 34, 56, 0, tm.GetLocation())
	if !absolute.Equal(expectedAbsolute) {
		t.Fatalf("expected %v, got %v", expectedAbsolute, absolute)
	}

	today, err := tm.ParseTimeInTimezone("today")
	if err != nil {
		t.Fatalf("ParseTimeInTimezone returned error for today: %v", err)
	}
	nowInTZ := time.Now().In(tm.GetLocation())
	expectedToday := time.Date(nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(), 0, 0, 0, 0, tm.GetLocation())
	if !today.Equal(expectedToday) {
		t.Fatalf("expected today %v, got %v", expectedToday, today)
	}

	yesterday, err := tm.ParseTimeInTimezone("yesterday")
	if err != nil {
		t.Fatalf("ParseTimeInTimezone returned error for yesterday: %v", err)
	}
	expectedYesterday := expectedToday.AddDate(0, 0, -1)
	if !yesterday.Equal(expectedYesterday) {
		t.Fatalf("expected yesterday %v, got %v", expectedYesterday, yesterday)
	}

	twoHoursAgo, err := tm.ParseTimeInTimezone("2h")
	if err != nil {
		t.Fatalf("ParseTimeInTimezone returned error for relative hours: %v", err)
	}
	if delta := time.Since(twoHoursAgo); delta < 2*time.Hour || delta > 2*time.Hour+5*time.Second {
		t.Fatalf("expected about 2 hours ago, got delta %v", delta)
	}

	thirtyMinutesAgo, err := tm.ParseTimeInTimezone("30m")
	if err != nil {
		t.Fatalf("ParseTimeInTimezone returned error for relative minutes: %v", err)
	}
	if delta := time.Since(thirtyMinutesAgo); delta < 30*time.Minute || delta > 30*time.Minute+5*time.Second {
		t.Fatalf("expected about 30 minutes ago, got delta %v", delta)
	}
}

func TestValidateTimeRangeRejectsFutureAndOverOneYear(t *testing.T) {
	tm, err := NewTimezoneManager("UTC")
	if err != nil {
		t.Fatalf("NewTimezoneManager returned error: %v", err)
	}

	now := time.Now().UTC()
	if err := tm.ValidateTimeRange(now.Add(-24*time.Hour), now); err != nil {
		t.Fatalf("expected valid time range, got %v", err)
	}
	if err := tm.ValidateTimeRange(now.Add(-time.Hour), now.Add(time.Hour)); err == nil {
		t.Fatalf("expected future end time to fail")
	}
	if err := tm.ValidateTimeRange(now.Add(-366*24*time.Hour), now); err == nil {
		t.Fatalf("expected range over one year to fail")
	}
}

func TestValidateTimezoneAndSupportedList(t *testing.T) {
	valid := ValidateTimezone("Asia/Tokyo")
	if !valid.IsValid || valid.Error != nil || valid.Location == nil {
		t.Fatalf("expected Asia/Tokyo to be valid, got %+v", valid)
	}

	invalid := ValidateTimezone("Invalid/Timezone")
	if invalid.IsValid || invalid.Error == nil {
		t.Fatalf("expected invalid timezone validation to fail, got %+v", invalid)
	}

	if !IsTimezoneSupported("Asia/Tokyo") {
		t.Fatalf("expected Asia/Tokyo to be in supported list")
	}
	if IsTimezoneSupported("Mars/Olympus") {
		t.Fatalf("expected unsupported timezone to be rejected")
	}
}
