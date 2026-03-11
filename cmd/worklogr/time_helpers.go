package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/utils"
)

var nowFunc = time.Now

func parseAdjustedTimeRange(startStr, endStr, configPath string) (time.Time, time.Time, error) {
	startTime, endTime, err := parseTimeRange(startStr, endStr, configPath)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return startTime, adjustInclusiveEndTime(endTime, nowFunc()), nil
}

func adjustInclusiveEndTime(endTime, now time.Time) time.Time {
	if endTime.Hour() != 0 || endTime.Minute() != 0 || endTime.Second() != 0 {
		return endTime
	}

	adjustedEndTime := endTime.Add(24*time.Hour - time.Second)
	if !adjustedEndTime.After(now) {
		return adjustedEndTime
	}

	return now
}

func parseTimeRange(startStr, endStr, configPath string) (time.Time, time.Time, error) {
	var startTime, endTime time.Time
	var err error

	if startTime, err = parseTimeString(startStr, configPath); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("開始時刻が無効です: %w", err)
	}

	if endTime, err = parseTimeString(endStr, configPath); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("終了時刻が無効です: %w", err)
	}

	if startTime.After(endTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("開始時刻は終了時刻より後にできません")
	}

	return startTime, endTime, nil
}

func parseTimeString(timeStr, configPath string) (time.Time, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		cfg = &config.Config{Timezone: "Asia/Tokyo"}
	}

	timezoneManager, err := utils.NewTimezoneManager(cfg.Timezone)
	if err != nil {
		timezoneManager, _ = utils.NewTimezoneManager("Asia/Tokyo")
	}

	if t, err := timezoneManager.ParseTimeInTimezone(timeStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("時刻の解析に失敗しました: %s", timeStr)
}

func parseRelativeTime(timeStr, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
	}

	now := nowFunc().In(loc)

	switch strings.ToLower(timeStr) {
	case "now":
		return now, nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, loc), nil
	}

	if len(timeStr) > 1 {
		unit := timeStr[len(timeStr)-1:]
		valueStr := timeStr[:len(timeStr)-1]

		value, err := strconv.Atoi(valueStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("相対時刻の値が無効です: %s", valueStr)
		}

		switch unit {
		case "d":
			return now.AddDate(0, 0, -value), nil
		case "h":
			return now.Add(time.Duration(-value) * time.Hour), nil
		case "m":
			return now.Add(time.Duration(-value) * time.Minute), nil
		}
	}

	return time.Time{}, fmt.Errorf("サポートされていない相対時刻形式です: %s", timeStr)
}
