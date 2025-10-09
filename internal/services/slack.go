package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/iriam/worklogr/internal/auth"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/utils"
	"github.com/slack-go/slack"
)

// SlackClient はSlack API操作を処理します
type SlackClient struct {
	client          *slack.Client
	retryClient     *RetryableSlackClient
	userID          string
	maxRetries      int
	timezoneManager *utils.TimezoneManager
	authManager     auth.AuthManager
}

// NewSlackClient はリトライ機能付きの新しいSlackクライアントを作成します
func NewSlackClient(token string, cfg *config.Config) (*SlackClient, error) {
	maxRetries := 3 // デフォルトリトライ回数

	// リトライ可能なクライアントを作成
	retryClient := NewRetryableSlackClient(token, maxRetries)
	client := retryClient.GetClient()

	// 認証されたユーザーを識別するためユーザー情報を取得
	var authTest *slack.AuthTestResponse
	err := retryClient.RetryableAPICall("auth.test", func() error {
		var err error
		authTest, err = client.AuthTest()
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("Slack認証に失敗しました: %w", err)
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

	return &SlackClient{
		client:          client,
		retryClient:     retryClient,
		userID:          authTest.UserID,
		maxRetries:      maxRetries,
		timezoneManager: timezoneManager,
	}, nil
}

// NewSlackClientWithAuth は認証マネージャー付きの新しいSlackクライアントを作成します
func NewSlackClientWithAuth(authManager auth.AuthManager, cfg *config.Config) (*SlackClient, error) {
	// 認証状態を確認
	ctx := context.Background()
	status, err := authManager.ValidateToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("認証検証に失敗しました: %w", err)
	}

	if !status.IsValid {
		return nil, fmt.Errorf("無効な認証状態: %s", status.ErrorMessage)
	}

	// トークン情報を取得
	tokenInfo := authManager.GetTokenInfo()
	if tokenInfo.TokenType != "access" {
		return nil, fmt.Errorf("無効なトークンタイプ: %s", tokenInfo.TokenType)
	}

	// 認証設定からアクセストークンを取得
	authConfig := authManager.(*auth.SlackAuthManager).GetConfig()
	
	// 既存のコンストラクタを使用してクライアントを作成
	slackClient, err := NewSlackClient(authConfig.AccessToken, cfg)
	if err != nil {
		return nil, fmt.Errorf("Slackクライアントの作成に失敗しました: %w", err)
	}

	// 認証マネージャーを設定
	slackClient.authManager = authManager

	return slackClient, nil
}

// ValidateAuthentication は認証状態を検証します
func (sc *SlackClient) ValidateAuthentication(ctx context.Context) error {
	if sc.authManager == nil {
		return fmt.Errorf("認証マネージャーが設定されていません")
	}

	status, err := sc.authManager.ValidateToken(ctx)
	if err != nil {
		return fmt.Errorf("認証検証に失敗しました: %w", err)
	}

	if !status.IsValid {
		return fmt.Errorf("認証が無効です: %s", status.ErrorMessage)
	}

	return nil
}

// RefreshAuthenticationIfNeeded は必要に応じて認証をリフレッシュします
func (sc *SlackClient) RefreshAuthenticationIfNeeded(ctx context.Context) error {
	if sc.authManager == nil {
		return nil // 認証マネージャーがない場合はスキップ
	}

	// Slackは通常リフレッシュをサポートしないため、検証のみ実行
	return sc.ValidateAuthentication(ctx)
}

// GetAuthStatus は認証状態を取得します
func (sc *SlackClient) GetAuthStatus() *auth.AuthStatus {
	if sc.authManager == nil {
		return &auth.AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: "認証マネージャーが設定されていません",
			TokenType:    "access",
		}
	}

	return sc.authManager.GetAuthStatus()
}

// CollectSlackEvents は指定された時間範囲内でSlackからイベントを収集します
func (sc *SlackClient) CollectSlackEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	fmt.Printf("ユーザー %s の Slack イベントを %s から %s まで収集中\n",
		sc.userID, startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"))

	// 包括的なメッセージ収集のため検索ベースのアプローチを使用
	fmt.Println("包括的な収集のため検索ベースの収集を使用中...")
	searchEvents, err := sc.collectMessagesViaSearch(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("検索ベースの収集に失敗しました: %w", err)
	}
	
	fmt.Printf("✅ Search API による収集が成功しました！ %d 件のメッセージを発見\n", len(searchEvents))
	events = append(events, searchEvents...)

	// 注意: ダイレクトメッセージは既に検索結果に含まれています
	// Search API使用時は別途DM収集は不要です

	fmt.Printf("合計 %d 件の Slack イベントを収集しました\n", len(events))
	return events, nil
}


// collectMessagesViaSearch はSlack Search APIを使用してユーザーメッセージを包括的に収集します
func (sc *SlackClient) collectMessagesViaSearch(startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	// タイムゾーンマネージャーを使用して設定されたタイムゾーンに時刻を変換
	startTimeInTZ := sc.timezoneManager.ConvertToTimezone(startTime)
	endTimeInTZ := sc.timezoneManager.ConvertToTimezone(endTime)

	// 適切な日付ロジックでSlack検索用の日付をフォーマット
	// Slackの検索フィルターは以下のように動作します:
	// - before:YYYY-MM-DD は "YYYY-MM-DD 00:00:00" より前のメッセージを検索
	// - after:YYYY-MM-DD は "YYYY-MM-DD 23:59:59" より後のメッセージを検索
	
	// 開始日については、開始日のメッセージを含めるため
	// 開始日の前日を "after" に指定する必要があります
	startDateForSearch := startTimeInTZ.AddDate(0, 0, -1).Format("2006-01-02")
	
	// 終了日については、終了日のメッセージを含めるため
	// 終了日の翌日を "before" に指定する必要があります
	endDateForSearch := endTimeInTZ.AddDate(0, 0, 1).Format("2006-01-02")

	// 認証されたユーザーからのメッセージを検索
	query := fmt.Sprintf("from:@%s after:%s before:%s", sc.userID, startDateForSearch, endDateForSearch)
	
	fmt.Printf("クエリでメッセージを検索中: %s (%s から %s をカバー)\n", 
		query, startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))

	var searchResult *slack.SearchMessages
	err := sc.retryClient.RetryableAPICall("search.messages", func() error {
		var err error
		searchResult, err = sc.client.SearchMessages(query, slack.SearchParameters{
			Sort:      "timestamp",
			Highlight: false,
			Count:     1000,
		})
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("メッセージ検索に失敗しました: %w", err)
	}

	fmt.Printf("検索で %d 件のメッセージを発見\n", len(searchResult.Matches))

	// 検索結果を処理
	for _, match := range searchResult.Matches {
		// タイムスタンプを解析
		ts, err := strconv.ParseFloat(match.Timestamp, 64)
		if err != nil {
			continue
		}
		timestamp := time.Unix(int64(ts), 0)

		// 正確な時間範囲でフィルタリング（検索は日付を使用、正確な時刻が必要）
		// 終了時刻が00:00:00の場合、その日全体を含むよう延長
		actualEndTime := endTime
		if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
			actualEndTime = endTime.Add(24*time.Hour - time.Second)
		}
		
		if timestamp.Before(startTime) || timestamp.After(actualEndTime) {
			continue
		}

		// ボットメッセージをスキップ
		if match.User != sc.userID {
			continue
		}

		// より良いコンテキストのためチャンネル名を取得
		channelName := match.Channel.Name
		if channelName == "" {
			channelName = match.Channel.ID
		}

		// イベントを作成
		event := &config.Event{
			ID:        fmt.Sprintf("slack_search_%s_%s", match.Channel.ID, match.Timestamp),
			Service:   "slack",
			Type:      "message",
			Title:     fmt.Sprintf("Message in #%s", channelName),
			Content:   match.Text,
			Timestamp: timestamp,
			UserID:    match.User,
			Metadata:  sc.createSearchMessageMetadata(match),
		}

		events = append(events, event)
	}

	// より多くの結果がある場合はページネーションを処理
	if searchResult.Paging.Pages > 1 {
		fmt.Printf("追加ページを処理中 (全 %d ページ)...\n", searchResult.Paging.Pages)
		
		for page := 2; page <= searchResult.Paging.Pages && page <= 10; page++ { // 10ページに制限
			time.Sleep(1 * time.Second) // レート制限
			
			var pageResult *slack.SearchMessages
			err := sc.retryClient.RetryableAPICall("search.messages (page)", func() error {
				var err error
				pageResult, err = sc.client.SearchMessages(query, slack.SearchParameters{
					Sort:      "timestamp",
					Highlight: false,
					Count:     1000,
					Page:      page,
				})
				return err
			})

			if err != nil {
				fmt.Printf("警告: ページ %d の取得に失敗しました: %v\n", page, err)
				continue
			}

			fmt.Printf("ページ %d を処理中 (%d 件のメッセージ)\n", page, len(pageResult.Matches))

			for _, match := range pageResult.Matches {
				ts, err := strconv.ParseFloat(match.Timestamp, 64)
				if err != nil {
					continue
				}
				timestamp := time.Unix(int64(ts), 0)

				// Filter by exact time range (search uses dates, we need precise times)
				// If end time is at 00:00:00, extend it to include the entire day
				actualEndTime := endTime
				if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
					actualEndTime = endTime.Add(24*time.Hour - time.Second)
				}
				
				if timestamp.Before(startTime) || timestamp.After(actualEndTime) {
					continue
				}

				if match.User != sc.userID {
					continue
				}

				channelName := match.Channel.Name
				if channelName == "" {
					channelName = match.Channel.ID
				}

				event := &config.Event{
					ID:        fmt.Sprintf("slack_search_%s_%s", match.Channel.ID, match.Timestamp),
					Service:   "slack",
					Type:      "message",
					Title:     fmt.Sprintf("Message in #%s", channelName),
					Content:   match.Text,
					Timestamp: timestamp,
					UserID:    match.User,
					Metadata:  sc.createSearchMessageMetadata(match),
				}

				events = append(events, event)
			}
		}
	}

	fmt.Printf("検索で %d 件のメッセージを収集しました\n", len(events))
	return events, nil
}


// createSearchMessageMetadata は検索結果メッセージのメタデータを作成します
func (sc *SlackClient) createSearchMessageMetadata(match slack.SearchMessage) string {
	metadata := map[string]interface{}{
		"channel_id":   match.Channel.ID,
		"channel_name": match.Channel.Name,
		"message_ts":   match.Timestamp,
		"permalink":    match.Permalink,
		"via_search":   true,
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}
