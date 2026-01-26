// Package auth provides authentication managers and token utilities.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarAuthManager はGoogle Calendar認証を管理します（OAuth認証とgcloud認証の両方をサポート）
type CalendarAuthManager struct {
	*BaseAuthManager
	validator    *TokenValidator // トークンバリデーター
	store        *TokenStore     // トークンストア
	clientID     string          // OAuthクライアントID
	clientSecret string          // OAuthクライアントシークレット
	useGCloud    bool            // gcloud認証を使用するかどうか
}

// NewCalendarAuthManager は新しいGoogle Calendar認証マネージャーを作成します
func NewCalendarAuthManager(config *AuthConfig, store *TokenStore, clientID, clientSecret string) *CalendarAuthManager {
	return &CalendarAuthManager{
		BaseAuthManager: NewBaseAuthManager("calendar", config),
		validator:       NewTokenValidator(),
		store:          store,
		clientID:       clientID,
		clientSecret:   clientSecret,
		useGCloud:      false, // デフォルトはOAuth認証
	}
}

// NewCalendarAuthManagerWithGCloud はgcloud認証を使用するGoogle Calendar認証マネージャーを作成します
func NewCalendarAuthManagerWithGCloud(config *AuthConfig, store *TokenStore) *CalendarAuthManager {
	return &CalendarAuthManager{
		BaseAuthManager: NewBaseAuthManager("calendar", config),
		validator:       NewTokenValidator(),
		store:          store,
		useGCloud:      true, // gcloud認証を使用
	}
}

// ValidateToken はGoogle Calendarアクセストークンを検証します
func (c *CalendarAuthManager) ValidateToken(ctx context.Context) (*AuthStatus, error) {
	// gcloud認証を使用する場合
	if c.useGCloud {
		return c.validateGCloudToken(ctx)
	}

	// OAuth認証を使用する場合
	if c.config.AccessToken == "" {
		c.UpdateStatus(false, "アクセストークンが設定されていません")
		return c.GetAuthStatus(), nil
	}

	// トークンが期限切れかチェック
	if c.IsTokenExpired() {
		// 自動リフレッシュが有効な場合はリフレッシュを試行
		if c.config.AutoRefresh && c.config.RefreshToken != "" {
			if err := c.RefreshToken(ctx); err != nil {
				c.UpdateStatus(false, fmt.Sprintf("トークンリフレッシュに失敗: %v", err))
				return c.GetAuthStatus(), nil
			}
		} else {
			c.UpdateStatus(false, "トークンの有効期限が切れています")
			return c.GetAuthStatus(), nil
		}
	}

	// Google APIでトークンを検証
	status, err := c.validator.ValidateGoogleToken(ctx, c.config.AccessToken)
	if err != nil {
		c.UpdateStatus(false, fmt.Sprintf("トークン検証エラー: %v", err))
		return c.GetAuthStatus(), err
	}

	// 内部状態を更新
	c.UpdateStatus(status.IsValid, status.ErrorMessage)
	
	// 有効な場合はトークンを保存
	if status.IsValid && c.store != nil {
		if err := c.store.StoreToken("calendar", c.config.AccessToken, c.config.RefreshToken, c.config.TokenExpiresAt, []string{"https://www.googleapis.com/auth/calendar.readonly"}); err != nil {
			// エラーをログに記録するが検証は失敗させない
			fmt.Printf("警告: Calendarトークンの保存に失敗しました: %v\n", err)
		}
	}

	return c.GetAuthStatus(), nil
}

// validateGCloudToken はgcloud認証を使用してトークンを検証します
func (c *CalendarAuthManager) validateGCloudToken(ctx context.Context) (*AuthStatus, error) {
	// gcloud認証が利用可能かチェック
	if !c.IsGCloudAuthenticated() {
		c.UpdateStatus(false, "gcloud認証が利用できません")
		return c.GetAuthStatus(), nil
	}

	// gcloudからトークンを取得
	token, err := c.GetGoogleCredentials([]string{
		calendar.CalendarReadonlyScope,
		calendar.CalendarEventsReadonlyScope,
	})
	if err != nil {
		c.UpdateStatus(false, fmt.Sprintf("gcloud認証情報の取得に失敗: %v", err))
		return c.GetAuthStatus(), err
	}

	// トークンを設定に保存
	c.config.AccessToken = token.AccessToken
	c.config.RefreshToken = token.RefreshToken
	c.config.TokenExpiresAt = &token.Expiry

	// 認証状態を更新
	c.UpdateStatus(true, "")
	return c.GetAuthStatus(), nil
}

// RefreshToken refreshes the Google Calendar access token
func (c *CalendarAuthManager) RefreshToken(ctx context.Context) error {
	if c.config.RefreshToken == "" {
		return fmt.Errorf("リフレッシュトークンが設定されていません")
	}

	if c.clientID == "" || c.clientSecret == "" {
		return fmt.Errorf("クライアントIDまたはクライアントシークレットが設定されていません")
	}

	// Prepare refresh request
	data := url.Values{}
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("refresh_token", c.config.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("リフレッシュリクエストの作成に失敗: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("リフレッシュリクエストに失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("リフレッシュリクエストが失敗しました: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("レスポンスの解析に失敗: %w", err)
	}

	// Update configuration with new token
	c.config.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		c.config.RefreshToken = tokenResp.RefreshToken
	}
	
	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		c.config.TokenExpiresAt = &expiresAt
	}

	// Update store
	if c.store != nil {
		if err := c.store.StoreToken("calendar", c.config.AccessToken, c.config.RefreshToken, c.config.TokenExpiresAt, []string{"https://www.googleapis.com/auth/calendar.readonly"}); err != nil {
			fmt.Printf("Warning: Failed to store refreshed Calendar token: %v\n", err)
		}
		if err := c.store.UpdateRefreshTime("calendar"); err != nil {
			fmt.Printf("Warning: Failed to update refresh time: %v\n", err)
		}
	}

	c.UpdateStatus(true, "")
	return nil
}

// GetTokenInfo returns Google Calendar token information
func (c *CalendarAuthManager) GetTokenInfo() *TokenInfo {
	info := c.BaseAuthManager.GetTokenInfo()
	info.Scopes = []string{"https://www.googleapis.com/auth/calendar.readonly"}
	
	if c.store != nil {
		if storedToken, err := c.store.GetToken("calendar"); err == nil {
			info.LastRefresh = storedToken.LastRefresh
		}
	}
	
	return info
}

// IsHealthy checks if the Google Calendar authentication is healthy
func (c *CalendarAuthManager) IsHealthy(ctx context.Context) bool {
	status, err := c.ValidateToken(ctx)
	return err == nil && status.IsValid
}

// GetAuthURL returns the Google OAuth authorization URL
func (c *CalendarAuthManager) GetAuthURL(clientID, redirectURI string, scopes []string) string {
	if len(scopes) == 0 {
		scopes = []string{"https://www.googleapis.com/auth/calendar.readonly"}
	}
	
	scopeStr := ""
	for i, scope := range scopes {
		if i > 0 {
			scopeStr += "%20"
		}
		scopeStr += url.QueryEscape(scope)
	}
	
	return fmt.Sprintf("https://accounts.google.com/o/oauth2/auth?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&access_type=offline", 
		clientID, url.QueryEscape(redirectURI), scopeStr)
}

// UpdateTokenFromStore updates the token from secure storage
func (c *CalendarAuthManager) UpdateTokenFromStore() error {
	if c.store == nil {
		return fmt.Errorf("トークンストアが設定されていません")
	}

	storedToken, err := c.store.GetToken("calendar")
	if err != nil {
		return fmt.Errorf("ストアからのトークン取得に失敗: %w", err)
	}

	c.config.AccessToken = storedToken.AccessToken
	c.config.RefreshToken = storedToken.RefreshToken
	c.config.TokenExpiresAt = storedToken.ExpiresAt

	return nil
}

// ClearToken clears the stored token
func (c *CalendarAuthManager) ClearToken() error {
	c.config.AccessToken = ""
	c.config.RefreshToken = ""
	c.config.TokenExpiresAt = nil
	
	c.UpdateStatus(false, "トークンがクリアされました")

	if c.store != nil {
		return c.store.DeleteToken("calendar")
	}

	return nil
}

// GetLastValidationTime returns the last time the token was validated
func (c *CalendarAuthManager) GetLastValidationTime() time.Time {
	return c.status.LastChecked
}

// ShouldRefresh determines if the token should be refreshed
func (c *CalendarAuthManager) ShouldRefresh() bool {
	// Check if token is expired or will expire soon
	if c.config.TokenExpiresAt != nil {
		timeUntilExpiry := time.Until(*c.config.TokenExpiresAt)
		return timeUntilExpiry < 5*time.Minute
	}
	
	// Also refresh if validation failed and we have a refresh token
	return !c.status.IsValid && c.config.RefreshToken != ""
}

// GetServiceSpecificInfo returns Google Calendar-specific authentication information
func (c *CalendarAuthManager) GetServiceSpecificInfo() map[string]interface{} {
	return map[string]interface{}{
		"service":           "calendar",
		"supports_refresh":  true,
		"auth_url_template": "https://accounts.google.com/o/oauth2/auth",
		"required_scopes":   []string{"https://www.googleapis.com/auth/calendar.readonly"},
		"token_type":        "Bearer",
		"refresh_threshold": "5m",
	}
}

// GetCalendarAccess checks if the token has access to calendars
func (c *CalendarAuthManager) GetCalendarAccess(ctx context.Context) (bool, error) {
	if !c.status.IsValid {
		return false, fmt.Errorf("トークンが無効です")
	}

	// This would typically make an API call to check calendar access
	// For now, we'll assume access if the token is valid
	return true, nil
}

// GetUserProfile retrieves Google user profile information
func (c *CalendarAuthManager) GetUserProfile(ctx context.Context) (map[string]interface{}, error) {
	if !c.status.IsValid {
		return nil, fmt.Errorf("トークンが無効です")
	}

	// This would typically make an API call to get user profile
	// For now, return basic info
	return map[string]interface{}{
		"service": "calendar",
		"valid":   true,
	}, nil
}

// gcloud認証関連のメソッド

// GetGoogleCredentials はgcloud認証からGoogle認証情報を取得します
func (c *CalendarAuthManager) GetGoogleCredentials(scopes []string) (*oauth2.Token, error) {
	// gcloud認証から認証情報を取得を試行
	ctx := context.Background()
	
	// まずApplication Default Credentials (ADC)を試行
	creds, err := google.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		return nil, fmt.Errorf("デフォルト認証情報が見つかりません。'gcloud auth application-default login'を実行してください: %w", err)
	}

	// トークンソースを取得
	tokenSource := creds.TokenSource
	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("認証情報からトークンの取得に失敗しました: %w", err)
	}

	return token, nil
}

// GetCalendarToken はGoogle Calendar専用のトークンを取得します
func (c *CalendarAuthManager) GetCalendarToken() (*oauth2.Token, error) {
	scopes := []string{
		calendar.CalendarReadonlyScope,
		calendar.CalendarEventsReadonlyScope,
	}
	return c.GetGoogleCredentials(scopes)
}

// IsGCloudAuthenticated はgcloud認証が利用可能かチェックします
func (c *CalendarAuthManager) IsGCloudAuthenticated() bool {
	// gcloud設定ディレクトリが存在するかチェック
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	gcloudConfigDir := filepath.Join(homeDir, ".config", "gcloud")
	if _, statErr := os.Stat(gcloudConfigDir); os.IsNotExist(statErr) {
		return false
	}

	// デフォルト認証情報の取得を試行
	ctx := context.Background()
	_, err = google.FindDefaultCredentials(ctx, calendar.CalendarReadonlyScope)
	return err == nil
}

// SetupGCloudAuth はgcloud認証のセットアップ手順を提供します
func (c *CalendarAuthManager) SetupGCloudAuth() string {
	return `gcloud認証を使用するには、以下のコマンドを実行してください:

1. Google Cloud SDKがインストールされていない場合はインストール:
   https://cloud.google.com/sdk/docs/install

2. Googleアカウントで認証:
   gcloud auth login

3. Calendar/Drive APIスコープでApplication Default Credentialsをセットアップ:
   gcloud auth application-default login --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/calendar.readonly,https://www.googleapis.com/auth/calendar.events.readonly,https://www.googleapis.com/auth/drive.readonly

4. Google Cloudプロジェクトを設定（必須）:
   gcloud config set project YOUR_PROJECT_ID
   
   注意: プロジェクトがない場合は以下で作成してください:
   https://console.cloud.google.com/projectcreate

5. Application Default Credentialsのクォータプロジェクトを更新:
   gcloud auth application-default set-quota-project YOUR_PROJECT_ID
   
   注意: これによりAPIクォータがプロジェクトで適切に追跡されます。
   成功メッセージ: "Quota project was added to ADC which can be used by 
   Google client libraries for billing and quota."

6. Google CloudプロジェクトでCalendar APIを有効化:
   gcloud services enable calendar-json.googleapis.com
   
   注意: これによりプロジェクト全体でCalendar APIが有効になります。
   このプロジェクトを使用するすべてのアプリケーションがCalendar APIにアクセスできます。

7. Google CloudプロジェクトでDrive APIを有効化（Geminiメモ等の添付取得用）:
   gcloud services enable drive.googleapis.com

これらの手順を完了すると、worklogrは自動的にgcloud認証情報を使用します。

注意: "insufficient authentication scopes"エラーが発生した場合は、
正しいスコープで手順3を再実行してください。`
}

// CreateCalendarService はgcloud認証を使用してGoogle Calendarサービスを作成します
func (c *CalendarAuthManager) CreateCalendarService() (*calendar.Service, error) {
	ctx := context.Background()
	
	// gcloudから認証情報を取得
	creds, err := google.FindDefaultCredentials(ctx, 
		calendar.CalendarReadonlyScope,
		calendar.CalendarEventsReadonlyScope,
	)
	if err != nil {
		return nil, fmt.Errorf("デフォルト認証情報が見つかりません: %w", err)
	}

	// カレンダーサービスを作成
	service, err := calendar.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("カレンダーサービスの作成に失敗しました: %w", err)
	}

	return service, nil
}

// SetUseGCloud はgcloud認証の使用を設定します
func (c *CalendarAuthManager) SetUseGCloud(useGCloud bool) {
	c.useGCloud = useGCloud
}

// IsUsingGCloud はgcloud認証を使用しているかどうかを返します
func (c *CalendarAuthManager) IsUsingGCloud() bool {
	return c.useGCloud
}
