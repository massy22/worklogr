// Package services provides clients for external services.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/utils"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// CalendarClient はGoogle Calendar API操作を処理します
type CalendarClient struct {
	service         *calendar.Service
	driveService    *drive.Service
	ctx             context.Context
	userID          string
	timezoneManager *utils.TimezoneManager
	options         config.GoogleCalendarOptions
}

// NewCalendarClient はgcloud認証を使用して新しいGoogle Calendarクライアントを作成します
func NewCalendarClient(cfg *config.Config) (*CalendarClient, error) {
	return NewCalendarClientWithGCloud(cfg)
}

// NewCalendarClientWithGCloud はgcloud認証を使用して新しいGoogle Calendarクライアントを作成します
func NewCalendarClientWithGCloud(cfg *config.Config) (*CalendarClient, error) {
	ctx := context.Background()
	
	// まずgcloud認証の使用を試行
	creds, err := google.FindDefaultCredentials(ctx, 
		calendar.CalendarReadonlyScope,
		calendar.CalendarEventsReadonlyScope,
		drive.DriveReadonlyScope,
	)
	
	var service *calendar.Service
	if err == nil {
		// gcloud認証情報を使用
		service, err = calendar.NewService(ctx, option.WithCredentials(creds))
		if err != nil {
			return nil, fmt.Errorf("gcloud認証でカレンダーサービスの作成に失敗しました: %w", err)
		}
	} else {
		return nil, fmt.Errorf("gcloud認証が利用できません。'gcloud auth application-default login'を実行してください: %w", err)
	}

	driveService, err := drive.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("gcloud認証でDriveサービスの作成に失敗しました: %w", err)
	}

	// 認証を確認するためユーザーのプライマリカレンダーを取得
	calendarList, err := service.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("google calendar認証に失敗しました: %w", err)
	}

	var userID string
	for _, cal := range calendarList.Items {
		if cal.Primary {
			userID = cal.Id
			break
		}
	}

	// 設定からタイムゾーンマネージャーを作成
	var timezoneManager *utils.TimezoneManager
	var err2 error
	if cfg != nil {
		timezoneManager, err2 = cfg.GetTimezoneManager()
	} else {
		timezoneManager, err2 = utils.NewTimezoneManager("Asia/Tokyo")
	}
	if err2 != nil {
		return nil, fmt.Errorf("タイムゾーンマネージャーの作成に失敗しました: %w", err2)
	}

	var options config.GoogleCalendarOptions
	if cfg != nil {
		options = cfg.GoogleCalendarOptions
	}

	return &CalendarClient{
		service:         service,
		driveService:    driveService,
		ctx:             ctx,
		userID:          userID,
		timezoneManager: timezoneManager,
		options:         options,
	}, nil
}


// CollectCalendarEvents は指定された時間範囲内でGoogle Calendarからイベントを収集します
func (cc *CalendarClient) CollectCalendarEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	fmt.Printf("📅 %s から %s まで Google Calendar イベントを収集中\n",
		startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"))

	// カレンダーのリストを取得
	calendarList, err := cc.service.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("カレンダーリストの取得に失敗しました: %w", err)
	}

	// マイカレンダー（Primary）のみを対象にする
	var primaryCal *calendar.CalendarListEntry
	for _, cal := range calendarList.Items {
		// アクセスできないカレンダーをスキップ
		if cal.AccessRole == "freeBusyReader" {
			continue
		}
		if cal.Primary || (cc.userID != "" && cal.Id == cc.userID) {
			primaryCal = cal
			break
		}
	}
	if primaryCal == nil {
		fmt.Println("Google Calendar: マイカレンダー（Primary）が見つかりませんでした（権限/設定をご確認ください）")
		return events, nil
	}

	calName := primaryCal.Summary
	if calName == "" {
		calName = primaryCal.Id
	}
	fmt.Printf("Google Calendar: マイカレンダーを処理中: %s\n", calName)

	calendarEvents, err := cc.collectEventsFromCalendar(primaryCal, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("マイカレンダー %s からのイベント収集に失敗しました: %w", calName, err)
	}
	fmt.Printf("   → %s: %d 件のイベントを収集しました\n", calName, len(calendarEvents))
	events = append(events, calendarEvents...)

	fmt.Printf("✅ 合計 %d 件の Google Calendar イベントを収集しました\n", len(events))
	return events, nil
}

// collectEventsFromCalendar は特定のカレンダーからイベントを収集します
func (cc *CalendarClient) collectEventsFromCalendar(cal *calendar.CalendarListEntry, startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	// タイムゾーンマネージャーを使用して時刻を変換
	startTimeInTZ := cc.timezoneManager.ConvertToTimezone(startTime)
	endTimeInTZ := cc.timezoneManager.ConvertToTimezone(endTime)

	// カレンダーからイベントを取得
	// 終了日全体を含むようAPI呼び出し用の終了時刻を調整
	apiEndTime := endTimeInTZ
	if endTimeInTZ.Hour() == 0 && endTimeInTZ.Minute() == 0 && endTimeInTZ.Second() == 0 {
		apiEndTime = endTimeInTZ.Add(24*time.Hour - time.Second)
	}
	
	eventsCall := cc.service.Events.List(cal.Id).
		TimeMin(startTimeInTZ.Format(time.RFC3339)).
		TimeMax(apiEndTime.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(2500)

	calName := cal.Summary
	if calName == "" {
		calName = cal.Id
	}

	pageNum := 1
	for {
		eventsResult, err := eventsCall.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get events: %w", err)
		}

		fmt.Printf("   [%s] ページ %d を処理中... (%d 件)\n", calName, pageNum, len(eventsResult.Items))
		beforePageEvents := len(events)

		for _, item := range eventsResult.Items {
			// Skip events that were cancelled
			if item.Status == "cancelled" {
				continue
			}

			// Fetch Gemini notes once per calendar item (Google Docs attachments only)
			attachments := cc.fetchDriveDocAttachments(item.Attachments)

			// Parse event times
			var eventStart, eventEnd time.Time
			if item.Start.DateTime != "" {
				eventStart, _ = time.Parse(time.RFC3339, item.Start.DateTime)
			} else if item.Start.Date != "" {
				eventStart, _ = time.Parse("2006-01-02", item.Start.Date)
			}

			if item.End.DateTime != "" {
				eventEnd, _ = time.Parse(time.RFC3339, item.End.DateTime)
			} else if item.End.Date != "" {
				eventEnd, _ = time.Parse("2006-01-02", item.End.Date)
			}

			// Check if event was created by the authenticated user in time range
			createdTime, err := time.Parse(time.RFC3339, item.Created)
			
			// Calculate actual end time for filtering
			actualEndTime := endTime
			if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
				actualEndTime = endTime.Add(24*time.Hour - time.Second)
			}
			
			if err == nil && createdTime.After(startTime) && createdTime.Before(actualEndTime) {
				// Only include events created by the authenticated user
				if item.Creator != nil && item.Creator.Email == cc.userID {
					event := &config.Event{
						ID:        fmt.Sprintf("calendar_event_created_%s_%s", cal.Id, item.Id),
						Service:   "google_calendar",
						Type:      "event_created",
						Title:     fmt.Sprintf("Created event: %s", item.Summary),
						Content:   item.Summary,
						Timestamp: createdTime,
						UserID:    item.Creator.Email,
						Metadata:  cc.createEventMetadata(cal, item, eventStart, eventEnd, "created", attachments),
						Attachments: attachments,
					}
					events = append(events, event)
				}
			}

			// Check if event was updated by the authenticated user in time range
			updatedTime, err := time.Parse(time.RFC3339, item.Updated)
			if err == nil && updatedTime.After(startTime) && updatedTime.Before(actualEndTime) && !updatedTime.Equal(createdTime) {
				// Only include updates made by the authenticated user
				if cc.isUserEventUpdater(item) {
					event := &config.Event{
						ID:        fmt.Sprintf("calendar_event_updated_%s_%s_%d", cal.Id, item.Id, updatedTime.Unix()),
						Service:   "google_calendar",
						Type:      "event_updated",
						Title:     fmt.Sprintf("Updated event: %s", item.Summary),
						Content:   item.Summary,
						Timestamp: updatedTime,
						UserID:    cc.userID,
						Metadata:  cc.createEventMetadata(cal, item, eventStart, eventEnd, "updated", attachments),
						Attachments: attachments,
					}
					events = append(events, event)
				}
			}

			// Check if event occurs in time range (for attendance tracking)
			if eventStart.After(startTime) && eventStart.Before(actualEndTime) {
				// Check if user is attending
				if cc.isUserAttending(item) {
					event := &config.Event{
						ID:        fmt.Sprintf("calendar_event_attended_%s_%s", cal.Id, item.Id),
						Service:   "google_calendar",
						Type:      "event_attended",
						Title:     fmt.Sprintf("Attended: %s", item.Summary),
						Content:   item.Summary,
						Timestamp: eventStart,
						UserID:    cc.userID,
						Metadata:  cc.createEventMetadata(cal, item, eventStart, eventEnd, "attended", attachments),
						Attachments: attachments,
					}
					events = append(events, event)
				}
			}
		}

		addedEvents := len(events) - beforePageEvents
		fmt.Printf("   [%s] ページ %d 完了: %d 件のイベントを生成（累積 %d 件）\n", calName, pageNum, addedEvents, len(events))

		// Check if there are more events
		if eventsResult.NextPageToken == "" {
			break
		}
		eventsCall.PageToken(eventsResult.NextPageToken)
		pageNum++
	}

	return events, nil
}

// isUserAttending checks if the authenticated user is attending the event
func (cc *CalendarClient) isUserAttending(event *calendar.Event) bool {
	// If no attendees list, assume user is attending if it's on their calendar
	if len(event.Attendees) == 0 {
		return true
	}

	for _, attendee := range event.Attendees {
		if attendee.Email == cc.userID {
			// Check response status
			return attendee.ResponseStatus == "accepted" || attendee.ResponseStatus == "tentative"
		}
	}

	// If user is the organizer, they're attending
	if event.Organizer != nil && event.Organizer.Email == cc.userID {
		return true
	}

	return false
}

// isUserEventUpdater checks if the authenticated user likely updated the event
func (cc *CalendarClient) isUserEventUpdater(event *calendar.Event) bool {
	// If user is the creator, they can update it
	if event.Creator != nil && event.Creator.Email == cc.userID {
		return true
	}

	// If user is the organizer, they can update it
	if event.Organizer != nil && event.Organizer.Email == cc.userID {
		return true
	}

	// If user is an attendee who can modify, they might have updated their status
	for _, attendee := range event.Attendees {
		if attendee.Email == cc.userID {
			// User is an attendee, so they might have updated their response
			return true
		}
	}

	return false
}

// getEventUpdater gets the email of who last updated the event
func (cc *CalendarClient) getEventUpdater(event *calendar.Event) string {
	// Try to get from attendees who might have updated their status
	for _, attendee := range event.Attendees {
		if attendee.Email == cc.userID {
			return attendee.Email
		}
	}

	// Fall back to organizer
	if event.Organizer != nil {
		return event.Organizer.Email
	}

	// Fall back to creator
	if event.Creator != nil {
		return event.Creator.Email
	}

	return cc.userID
}

// createEventMetadata creates metadata for a calendar event
func (cc *CalendarClient) createEventMetadata(cal *calendar.CalendarListEntry, event *calendar.Event, startTime, endTime time.Time, action string, attachments []config.EventAttachment) string {
	metadata := map[string]interface{}{
		"calendar_id":   cal.Id,
		"calendar_name": cal.Summary,
		"event_id":      event.Id,
		"action":        action,
		"start_time":    startTime.Format(time.RFC3339),
		"end_time":      endTime.Format(time.RFC3339),
		"location":      event.Location,
		"description":   event.Description,
		"html_link":     event.HtmlLink,
	}

	// Attachments are stored separately; keep only lightweight references in metadata.
	if len(attachments) > 0 {
		var refs []map[string]interface{}
		for _, a := range attachments {
			refs = append(refs, map[string]interface{}{
				"file_id":   a.FileID,
				"title":     a.Title,
				"mime_type": a.MimeType,
				"truncated": a.Truncated,
			})
		}
		metadata["gemini_note_files"] = refs
	}

	// Add attendee information
	if len(event.Attendees) > 0 {
		var attendees []map[string]interface{}
		for _, attendee := range event.Attendees {
			attendeeInfo := map[string]interface{}{
				"email":           attendee.Email,
				"response_status": attendee.ResponseStatus,
			}
			if attendee.DisplayName != "" {
				attendeeInfo["name"] = attendee.DisplayName
			}
			attendees = append(attendees, attendeeInfo)
		}
		metadata["attendees"] = attendees
		metadata["attendee_count"] = len(attendees)
	}

	// Add organizer information
	if event.Organizer != nil {
		metadata["organizer"] = map[string]interface{}{
			"email": event.Organizer.Email,
			"name":  event.Organizer.DisplayName,
		}
	}

	// Add recurrence information
	if len(event.Recurrence) > 0 {
		metadata["recurring"] = true
		metadata["recurrence_rules"] = event.Recurrence
	} else {
		metadata["recurring"] = false
	}

	// Add meeting information
	if event.ConferenceData != nil && len(event.ConferenceData.EntryPoints) > 0 {
		var meetingLinks []string
		for _, entryPoint := range event.ConferenceData.EntryPoints {
			if entryPoint.Uri != "" {
				meetingLinks = append(meetingLinks, entryPoint.Uri)
			}
		}
		if len(meetingLinks) > 0 {
			metadata["meeting_links"] = meetingLinks
		}
	}

	// Add event type information
	if event.EventType != "" {
		metadata["event_type"] = event.EventType
	}

	// Add visibility
	metadata["visibility"] = event.Visibility

	// Add all-day event flag
	metadata["all_day"] = (event.Start.Date != "" && event.End.Date != "")

	// Calculate duration
	if !startTime.IsZero() && !endTime.IsZero() {
		duration := endTime.Sub(startTime)
		metadata["duration_minutes"] = int(duration.Minutes())
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

func (cc *CalendarClient) fetchDriveDocAttachments(attachments []*calendar.EventAttachment) []config.EventAttachment {
	if !cc.options.ShouldFetchDriveAttachments() || cc.driveService == nil || len(attachments) == 0 {
		return nil
	}

	maxChars := cc.options.EffectiveAttachmentTextMaxChars()

	var notes []config.EventAttachment
	for _, a := range attachments {
		if a == nil || a.FileId == "" {
			continue
		}

		mimeType := a.MimeType
		title := a.Title
		if mimeType == "" || title == "" {
			if f, err := cc.driveService.Files.Get(a.FileId).Fields("mimeType,name,webViewLink").Do(); err == nil && f != nil {
				if mimeType == "" {
					mimeType = f.MimeType
				}
				if title == "" {
					title = f.Name
				}
			}
		}

		// Geminiメモ用: Googleドキュメントのみ対象（録画/その他添付は除外）
		if mimeType != "application/vnd.google-apps.document" {
			continue
		}

		exportMime := "text/plain"

		resp, err := cc.driveService.Files.Export(a.FileId, exportMime).Download()
		if err != nil || resp == nil || resp.Body == nil {
			continue
		}
		func() {
			defer resp.Body.Close()

			// Limit bytes to keep DB/export sane (approx up to 4 bytes per char)
			maxBytes := int64(maxChars*4 + 1024)
			b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
			if err != nil {
				return
			}

			text := string(b)
			runes := []rune(text)
			excerpt := text
			truncated := false
			if len(runes) > maxChars {
				excerpt = string(runes[:maxChars])
				truncated = true
			}

			notes = append(notes, config.EventAttachment{
				FileID:    a.FileId,
				Title:     title,
				MimeType:  mimeType,
				ExportAs:  exportMime,
				TextFull:  excerpt,
				Truncated: truncated,
			})
		}()
	}

	return notes
}

// GetCalendarList returns a list of available calendars
func (cc *CalendarClient) GetCalendarList() ([]*calendar.CalendarListEntry, error) {
	calendarList, err := cc.service.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get calendar list: %w", err)
	}

	return calendarList.Items, nil
}

// GetEventDetails gets detailed information about a specific event
func (cc *CalendarClient) GetEventDetails(calendarID, eventID string) (*calendar.Event, error) {
	event, err := cc.service.Events.Get(calendarID, eventID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get event details: %w", err)
	}

	return event, nil
}
