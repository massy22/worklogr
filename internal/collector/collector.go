package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/iriam/worklogr/internal/auth"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/database"
	"github.com/iriam/worklogr/internal/services"
	"github.com/iriam/worklogr/internal/utils"
)

// EventCollector は複数のサービスからのイベント収集を管理します
type EventCollector struct {
	config          *config.Config
	db              *database.DatabaseManager
	services        map[string]ServiceClient
	timezoneManager *utils.TimezoneManager
	authManagers    map[string]auth.AuthManager
	tokenStore      *auth.TokenStore
}

// ServiceClient はすべてのサービスクライアントのインターフェースです
type ServiceClient interface {
	CollectEvents(startTime, endTime time.Time) ([]*config.Event, error)
}

// SlackServiceClient はServiceClientインターフェースを実装するためSlackクライアントをラップします
type SlackServiceClient struct {
	client *services.SlackClient
}

func (s *SlackServiceClient) CollectEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	return s.client.CollectSlackEvents(startTime, endTime)
}

// GitHubServiceClient はServiceClientインターフェースを実装するためGitHubクライアントをラップします
type GitHubServiceClient struct {
	client *services.GitHubClient
}

func (g *GitHubServiceClient) CollectEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	return g.client.CollectGitHubEvents(startTime, endTime)
}


// CalendarServiceClient はServiceClientインターフェースを実装するためCalendarクライアントをラップします
type CalendarServiceClient struct {
	client *services.CalendarClient
}

func (c *CalendarServiceClient) CollectEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	return c.client.CollectCalendarEvents(startTime, endTime)
}

// NewEventCollector は新しいイベントコレクターを作成します
func NewEventCollector(cfg *config.Config, db *database.DatabaseManager) *EventCollector {
	// タイムゾーンマネージャーを作成
	timezoneManager, err := cfg.GetTimezoneManager()
	if err != nil {
		// 設定のタイムゾーンが無効な場合はデフォルトタイムゾーンにフォールバック
		timezoneManager, _ = utils.NewTimezoneManager("Asia/Tokyo")
	}

	return &EventCollector{
		config:          cfg,
		db:              db,
		services:        make(map[string]ServiceClient),
		timezoneManager: timezoneManager,
		authManagers:    make(map[string]auth.AuthManager),
	}
}

// NewEventCollectorWithAuth は認証マネージャー付きの新しいイベントコレクターを作成します
func NewEventCollectorWithAuth(cfg *config.Config, db *database.DatabaseManager, tokenStore *auth.TokenStore) *EventCollector {
	// タイムゾーンマネージャーを作成
	timezoneManager, err := cfg.GetTimezoneManager()
	if err != nil {
		// 設定のタイムゾーンが無効な場合はデフォルトタイムゾーンにフォールバック
		timezoneManager, _ = utils.NewTimezoneManager("Asia/Tokyo")
	}

	ec := &EventCollector{
		config:          cfg,
		db:              db,
		services:        make(map[string]ServiceClient),
		timezoneManager: timezoneManager,
		authManagers:    make(map[string]auth.AuthManager),
		tokenStore:      tokenStore,
	}

	// 認証マネージャーを初期化
	ec.initializeAuthManagers()

	return ec
}

// initializeAuthManagers は認証マネージャーを初期化します
func (ec *EventCollector) initializeAuthManagers() {
	// Slack認証マネージャーを初期化
	if ec.config.Slack.Enabled {
		authConfig := ec.config.Slack.ToAuthConfig()
		// config.AuthConfigをauth.AuthConfigに変換
		authAuthConfig := &auth.AuthConfig{
			AccessToken:            authConfig.AccessToken,
			RefreshToken:           authConfig.RefreshToken,
			TokenExpiresAt:         authConfig.TokenExpiresAt,
			AutoRefresh:            authConfig.AutoRefresh,
			ValidationInterval:     authConfig.ValidationInterval,
			MaxRetries:             authConfig.MaxRetries,
			RetryBackoffMultiplier: authConfig.RetryBackoffMultiplier,
		}
		slackAuth := auth.NewSlackAuthManager(authAuthConfig, ec.tokenStore)
		ec.authManagers["slack"] = slackAuth
	}

	// GitHub認証マネージャーを初期化
	if ec.config.GitHub.Enabled {
		authConfig := ec.config.GitHub.ToAuthConfig()
		// config.AuthConfigをauth.AuthConfigに変換
		authAuthConfig := &auth.AuthConfig{
			AccessToken:            authConfig.AccessToken,
			RefreshToken:           authConfig.RefreshToken,
			TokenExpiresAt:         authConfig.TokenExpiresAt,
			AutoRefresh:            authConfig.AutoRefresh,
			ValidationInterval:     authConfig.ValidationInterval,
			MaxRetries:             authConfig.MaxRetries,
			RetryBackoffMultiplier: authConfig.RetryBackoffMultiplier,
		}
		githubAuth := auth.NewGitHubAuthManager(authAuthConfig, ec.tokenStore)
		ec.authManagers["github"] = githubAuth
	}

	// Google Calendar認証マネージャーを初期化
	if ec.config.GoogleCal.Enabled {
		authConfig := ec.config.GoogleCal.ToAuthConfig()
		// config.AuthConfigをauth.AuthConfigに変換
		authAuthConfig := &auth.AuthConfig{
			AccessToken:            authConfig.AccessToken,
			RefreshToken:           authConfig.RefreshToken,
			TokenExpiresAt:         authConfig.TokenExpiresAt,
			AutoRefresh:            authConfig.AutoRefresh,
			ValidationInterval:     authConfig.ValidationInterval,
			MaxRetries:             authConfig.MaxRetries,
			RetryBackoffMultiplier: authConfig.RetryBackoffMultiplier,
		}
		calendarAuth := auth.NewCalendarAuthManager(authAuthConfig, ec.tokenStore, ec.config.GoogleCal.ClientID, ec.config.GoogleCal.ClientSecret)
		ec.authManagers["calendar"] = calendarAuth
	}
}

// InitializeServices は有効なすべてのサービスクライアントを初期化します
func (ec *EventCollector) InitializeServices() error {
	// Slackクライアントを初期化
	if ec.config.Slack.Enabled && ec.config.Slack.AccessToken != "" {
		slackClient, err := services.NewSlackClient(ec.config.Slack.AccessToken, ec.config)
		if err != nil {
			fmt.Printf("警告: Slackクライアントの初期化に失敗しました: %v\n", err)
		} else {
			ec.services["slack"] = &SlackServiceClient{client: slackClient}
			fmt.Println("Slack client initialized successfully")
		}
	}

	// GitHubクライアントを初期化
	if ec.config.GitHub.Enabled && ec.config.GitHub.AccessToken != "" {
		githubClient, err := services.NewGitHubClientWithConfig(ec.config.GitHub.AccessToken, ec.config)
		if err != nil {
			fmt.Printf("警告: GitHubクライアントの初期化に失敗しました: %v\n", err)
		} else {
			ec.services["github"] = &GitHubServiceClient{client: githubClient}
			fmt.Println("GitHub client initialized successfully")
		}
	}


	// Google Calendarクライアントを初期化
	if ec.config.GoogleCal.Enabled {
		// gcloud認証のみを使用
		calendarClient, err := services.NewCalendarClient(ec.config)
		if err != nil {
			fmt.Printf("警告: Google Calendarクライアントの初期化に失敗しました: %v\n", err)
		} else {
			ec.services["google_calendar"] = &CalendarServiceClient{client: calendarClient}
			fmt.Println("Google Calendar client initialized successfully")
		}
	}

	if len(ec.services) == 0 {
		return fmt.Errorf("有効または適切に設定されたサービスがありません")
	}

	fmt.Printf("Initialized %d service(s)\n", len(ec.services))
	return nil
}

// CollectEvents は指定された時間範囲内で有効なすべてのサービスからイベントを収集します
func (ec *EventCollector) CollectEvents(startTime, endTime time.Time, serviceNames []string) ([]*config.Event, error) {
	var allEvents []*config.Event

	// 収集対象のサービスを決定
	servicesToCollect := ec.services
	if len(serviceNames) > 0 {
		servicesToCollect = make(map[string]ServiceClient)
		for _, serviceName := range serviceNames {
			if client, exists := ec.services[serviceName]; exists {
				servicesToCollect[serviceName] = client
			} else {
				fmt.Printf("警告: サービス '%s' は利用できないか設定されていません\n", serviceName)
			}
		}
	}

	if len(servicesToCollect) == 0 {
		return nil, fmt.Errorf("収集可能なサービスがありません")
	}

	fmt.Printf("Collecting events from %s to %s\n", startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"))

	// 各サービスからイベントを収集
	for serviceName, client := range servicesToCollect {
		fmt.Printf("Collecting events from %s...\n", serviceName)
		
		events, err := client.CollectEvents(startTime, endTime)
		if err != nil {
			fmt.Printf("Error collecting events from %s: %v\n", serviceName, err)
			continue
		}

		fmt.Printf("Collected %d events from %s\n", len(events), serviceName)
		allEvents = append(allEvents, events...)
	}

	// イベントをタイムスタンプでソート
	ec.sortEventsByTimestamp(allEvents)

	fmt.Printf("Total events collected: %d\n", len(allEvents))
	return allEvents, nil
}

// CollectAndStore はイベントを収集してデータベースに保存します
func (ec *EventCollector) CollectAndStore(startTime, endTime time.Time, serviceNames []string) error {
	events, err := ec.CollectEvents(startTime, endTime, serviceNames)
	if err != nil {
		return fmt.Errorf("イベント収集に失敗しました: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("No events found in the specified time range")
		return nil
	}

	// イベントをデータベースに保存
	fmt.Printf("Storing %d events in database...\n", len(events))
	if err := ec.db.InsertEvents(events); err != nil {
		return fmt.Errorf("イベント保存に失敗しました: %w", err)
	}

	fmt.Println("Events stored successfully")
	return nil
}

// GetStoredEvents はデータベースからイベントを取得します
func (ec *EventCollector) GetStoredEvents(startTime, endTime time.Time, serviceNames []string) ([]*config.Event, error) {
	return ec.db.GetEvents(startTime, endTime, serviceNames)
}


// GetServiceStatus はすべてのサービスの状態を返します
func (ec *EventCollector) GetServiceStatus() map[string]ServiceStatus {
	status := make(map[string]ServiceStatus)

	services := []string{"slack", "github", "google_calendar"}
	for _, serviceName := range services {
		var serviceStatus ServiceStatus
		serviceStatus.Name = serviceName
		serviceStatus.Enabled = ec.isServiceEnabled(serviceName)
		serviceStatus.Authenticated = ec.isServiceAuthenticated(serviceName)
		serviceStatus.Initialized = ec.isServiceInitialized(serviceName)

		status[serviceName] = serviceStatus
	}

	return status
}

// ServiceStatus はサービスの状態を表します
type ServiceStatus struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Authenticated bool   `json:"authenticated"`
	Initialized   bool   `json:"initialized"`
}

// isServiceAuthenticated はサービスが有効なアクセストークンを持っているかチェックします
func (ec *EventCollector) isServiceAuthenticated(serviceName string) bool {
	switch serviceName {
	case "slack":
		return ec.config.Slack.AccessToken != ""
	case "github":
		return ec.config.GitHub.AccessToken != ""
	case "google_calendar":
		// Google Calendarはgcloud認証またはアクセストークンを使用可能
		return ec.config.GoogleCal.AccessToken != "" || ec.isGCloudAuthenticated()
	default:
		return false
	}
}

// isGCloudAuthenticated はgcloud認証が利用可能かチェックします
func (ec *EventCollector) isGCloudAuthenticated() bool {
	// これは簡単なチェックです - 実際にはgcloud認証情報を検証したい場合があります
	return true // Google Calendarが有効な場合はgcloudが利用可能と仮定
}

// isServiceEnabled は設定でサービスが有効になっているかチェックします
func (ec *EventCollector) isServiceEnabled(serviceName string) bool {
	switch serviceName {
	case "slack":
		return ec.config.Slack.Enabled
	case "github":
		return ec.config.GitHub.Enabled
	case "google_calendar":
		return ec.config.GoogleCal.Enabled
	default:
		return false
	}
}

// isServiceInitialized はサービスクライアントが初期化されているかチェックします
func (ec *EventCollector) isServiceInitialized(serviceName string) bool {
	_, exists := ec.services[serviceName]
	return exists
}

// sortEventsByTimestamp はイベントをタイムスタンプの昇順でソートします
func (ec *EventCollector) sortEventsByTimestamp(events []*config.Event) {
	// タイムスタンプ順序のためのシンプルなバブルソート
	n := len(events)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if events[j].Timestamp.After(events[j+1].Timestamp) {
				events[j], events[j+1] = events[j+1], events[j]
			}
		}
	}
}

// GetAvailableServices は利用可能なサービス名のリストを返します
func (ec *EventCollector) GetAvailableServices() []string {
	var services []string
	for serviceName := range ec.services {
		services = append(services, serviceName)
	}
	return services
}

// GetEnabledServices は設定から有効なサービス名のリストを返します
func (ec *EventCollector) GetEnabledServices() []string {
	var services []string
	
	if ec.config.Slack.Enabled {
		services = append(services, "slack")
	}
	if ec.config.GitHub.Enabled {
		services = append(services, "github")
	}
	if ec.config.GoogleCal.Enabled {
		services = append(services, "google_calendar")
	}

	return services
}

// ValidateTimeRange は時間範囲が妥当であることを検証します
func (ec *EventCollector) ValidateTimeRange(startTime, endTime time.Time) error {
	if startTime.After(endTime) {
		return fmt.Errorf("開始時刻は終了時刻より後にできません")
	}

	if endTime.After(time.Now()) {
		return fmt.Errorf("終了時刻は未来にできません")
	}

	// 時間範囲が大きすぎないかチェック（1年以上）
	if endTime.Sub(startTime) > 365*24*time.Hour {
		return fmt.Errorf("時間範囲は1年を超えることはできません")
	}

	return nil
}

// InitializeServicesWithAuth は認証マネージャーを使用してサービスクライアントを初期化します
func (ec *EventCollector) InitializeServicesWithAuth() error {
	if len(ec.authManagers) == 0 {
		return fmt.Errorf("認証マネージャーが初期化されていません")
	}

	// Slackクライアントを認証マネージャーで初期化
	if slackAuth, exists := ec.authManagers["slack"]; exists {
		slackClient, err := services.NewSlackClientWithAuth(slackAuth, ec.config)
		if err != nil {
			fmt.Printf("警告: 認証マネージャーでのSlackクライアント初期化に失敗しました: %v\n", err)
		} else {
			ec.services["slack"] = &SlackServiceClient{client: slackClient}
			fmt.Println("Slack client initialized with auth manager successfully")
		}
	}

	// GitHubクライアントを認証マネージャーで初期化
	if githubAuth, exists := ec.authManagers["github"]; exists {
		githubClient, err := services.NewGitHubClientWithAuth(githubAuth, ec.config)
		if err != nil {
			fmt.Printf("警告: 認証マネージャーでのGitHubクライアント初期化に失敗しました: %v\n", err)
		} else {
			ec.services["github"] = &GitHubServiceClient{client: githubClient}
			fmt.Println("GitHub client initialized with auth manager successfully")
		}
	}

	// Google Calendarクライアントは既存の方法を使用（gcloud認証）
	if ec.config.GoogleCal.Enabled {
		calendarClient, err := services.NewCalendarClient(ec.config)
		if err != nil {
			fmt.Printf("警告: Google Calendarクライアントの初期化に失敗しました: %v\n", err)
		} else {
			ec.services["google_calendar"] = &CalendarServiceClient{client: calendarClient}
			fmt.Println("Google Calendar client initialized successfully")
		}
	}

	if len(ec.services) == 0 {
		return fmt.Errorf("有効または適切に設定されたサービスがありません")
	}

	fmt.Printf("Initialized %d service(s) with auth managers\n", len(ec.services))
	return nil
}

// ValidateAllAuthentications はすべてのサービスの認証状態を検証します
func (ec *EventCollector) ValidateAllAuthentications(ctx context.Context) map[string]*auth.AuthValidationResult {
	results := make(map[string]*auth.AuthValidationResult)

	for serviceName, authManager := range ec.authManagers {
		status, err := authManager.ValidateToken(ctx)
		if err != nil {
			results[serviceName] = &auth.AuthValidationResult{
				ServiceName: serviceName,
				Status: &auth.AuthStatus{
					IsValid:      false,
					LastChecked:  time.Now(),
					ErrorMessage: err.Error(),
					TokenType:    "access",
				},
				Suggestions: []string{"認証検証中にエラーが発生しました"},
			}
			continue
		}

		result := &auth.AuthValidationResult{
			ServiceName: serviceName,
			Status:      status,
			Suggestions: []string{},
		}

		// 認証状態に基づく提案を追加
		if !status.IsValid {
			result.Suggestions = append(result.Suggestions, "認証が無効です。再認証を行ってください。")
		} else if status.ExpiresAt != nil {
			timeUntilExpiry := time.Until(*status.ExpiresAt)
			if timeUntilExpiry < 24*time.Hour {
				result.Suggestions = append(result.Suggestions, "トークンの有効期限が近づいています。")
			}
		}

		results[serviceName] = result
	}

	return results
}

// RefreshExpiredTokens は期限切れのトークンをリフレッシュします
func (ec *EventCollector) RefreshExpiredTokens(ctx context.Context) error {
	var errors []string

	for serviceName, authManager := range ec.authManagers {
		if authManager.IsTokenExpired() {
			fmt.Printf("Refreshing expired token for %s...\n", serviceName)
			if err := authManager.RefreshToken(ctx); err != nil {
				errorMsg := fmt.Sprintf("Failed to refresh token for %s: %v", serviceName, err)
				errors = append(errors, errorMsg)
				fmt.Printf("警告: %s\n", errorMsg)
			} else {
				fmt.Printf("Successfully refreshed token for %s\n", serviceName)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("トークンリフレッシュエラー: %v", errors)
	}

	return nil
}

// GetAuthManager は指定されたサービスの認証マネージャーを取得します
func (ec *EventCollector) GetAuthManager(serviceName string) (auth.AuthManager, bool) {
	authManager, exists := ec.authManagers[serviceName]
	return authManager, exists
}

// GetAllAuthStatuses はすべてのサービスの認証状態を取得します
func (ec *EventCollector) GetAllAuthStatuses() map[string]*auth.AuthStatus {
	statuses := make(map[string]*auth.AuthStatus)

	for serviceName, authManager := range ec.authManagers {
		statuses[serviceName] = authManager.GetAuthStatus()
	}

	return statuses
}

// UpdateAuthConfig は認証設定を更新します
func (ec *EventCollector) UpdateAuthConfig(serviceName string, authConfig *auth.AuthConfig) error {
	// auth.AuthConfigをconfig.AuthConfigに変換
	configAuthConfig := &config.AuthConfig{
		AccessToken:            authConfig.AccessToken,
		RefreshToken:           authConfig.RefreshToken,
		TokenExpiresAt:         authConfig.TokenExpiresAt,
		AutoRefresh:            authConfig.AutoRefresh,
		ValidationInterval:     authConfig.ValidationInterval,
		MaxRetries:             authConfig.MaxRetries,
		RetryBackoffMultiplier: authConfig.RetryBackoffMultiplier,
	}

	// 設定を更新
	switch serviceName {
	case "slack":
		ec.config.Slack.UpdateFromAuthConfig(configAuthConfig)
	case "github":
		ec.config.GitHub.UpdateFromAuthConfig(configAuthConfig)
	case "calendar":
		ec.config.GoogleCal.UpdateFromAuthConfig(configAuthConfig)
	default:
		return fmt.Errorf("未知のサービス: %s", serviceName)
	}

	// 認証マネージャーを再初期化
	ec.initializeAuthManagers()

	return nil
}
