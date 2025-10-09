package auth

import (
	"context"
	"fmt"
	"time"
)

// SlackAuthManager はSlack認証を管理します
type SlackAuthManager struct {
	*BaseAuthManager
	validator *TokenValidator // トークンバリデーター
	store     *TokenStore     // トークンストア
}

// NewSlackAuthManager は新しいSlack認証マネージャーを作成します
func NewSlackAuthManager(config *AuthConfig, store *TokenStore) *SlackAuthManager {
	return &SlackAuthManager{
		BaseAuthManager: NewBaseAuthManager("slack", config),
		validator:       NewTokenValidator(),
		store:          store,
	}
}

// ValidateToken はSlackアクセストークンを検証します
func (s *SlackAuthManager) ValidateToken(ctx context.Context) (*AuthStatus, error) {
	if s.config.AccessToken == "" {
		s.UpdateStatus(false, "アクセストークンが設定されていません")
		return s.GetAuthStatus(), nil
	}

	// トークンが期限切れかチェック
	if s.IsTokenExpired() {
		s.UpdateStatus(false, "トークンの有効期限が切れています")
		return s.GetAuthStatus(), nil
	}

	// Slack APIでトークンを検証
	status, err := s.validator.ValidateSlackToken(ctx, s.config.AccessToken)
	if err != nil {
		s.UpdateStatus(false, fmt.Sprintf("トークン検証エラー: %v", err))
		return s.GetAuthStatus(), err
	}

	// 内部状態を更新
	s.UpdateStatus(status.IsValid, status.ErrorMessage)
	
	// 有効な場合はトークンを保存
	if status.IsValid && s.store != nil {
		if err := s.store.StoreToken("slack", s.config.AccessToken, s.config.RefreshToken, s.config.TokenExpiresAt, []string{"chat:write", "channels:read"}); err != nil {
			// エラーをログに記録するが検証は失敗させない
			fmt.Printf("警告: Slackトークンの保存に失敗しました: %v\n", err)
		}
	}

	return s.GetAuthStatus(), nil
}

// RefreshToken はSlackアクセストークンをリフレッシュします
func (s *SlackAuthManager) RefreshToken(ctx context.Context) error {
	if s.config.RefreshToken == "" {
		return fmt.Errorf("リフレッシュトークンが設定されていません")
	}

	// SlackはOAuth2と同じ方法でのトークンリフレッシュをサポートしていません
	// 通常は再認証が必要です
	return fmt.Errorf("Slackトークンのリフレッシュには再認証が必要です")
}

// GetTokenInfo はSlackトークン情報を返します
func (s *SlackAuthManager) GetTokenInfo() *TokenInfo {
	info := s.BaseAuthManager.GetTokenInfo()
	info.Scopes = []string{"chat:write", "channels:read", "users:read"}
	return info
}

// IsHealthy はSlack認証が正常かどうかをチェックします
func (s *SlackAuthManager) IsHealthy(ctx context.Context) bool {
	status, err := s.ValidateToken(ctx)
	return err == nil && status.IsValid
}

// GetAuthURL はSlack OAuth認証URLを返します
func (s *SlackAuthManager) GetAuthURL(clientID, redirectURI string, scopes []string) string {
	if len(scopes) == 0 {
		scopes = []string{"chat:write", "channels:read", "users:read"}
	}
	
	scopeStr := ""
	for i, scope := range scopes {
		if i > 0 {
			scopeStr += ","
		}
		scopeStr += scope
	}
	
	return fmt.Sprintf("https://slack.com/oauth/v2/authorize?client_id=%s&scope=%s&redirect_uri=%s", 
		clientID, scopeStr, redirectURI)
}

// UpdateTokenFromStore はセキュアストレージからトークンを更新します
func (s *SlackAuthManager) UpdateTokenFromStore() error {
	if s.store == nil {
		return fmt.Errorf("トークンストアが設定されていません")
	}

	storedToken, err := s.store.GetToken("slack")
	if err != nil {
		return fmt.Errorf("ストアからのトークン取得に失敗: %w", err)
	}

	s.config.AccessToken = storedToken.AccessToken
	s.config.RefreshToken = storedToken.RefreshToken
	s.config.TokenExpiresAt = storedToken.ExpiresAt

	return nil
}

// ClearToken は保存されたトークンをクリアします
func (s *SlackAuthManager) ClearToken() error {
	s.config.AccessToken = ""
	s.config.RefreshToken = ""
	s.config.TokenExpiresAt = nil
	
	s.UpdateStatus(false, "トークンがクリアされました")

	if s.store != nil {
		return s.store.DeleteToken("slack")
	}

	return nil
}

// GetLastValidationTime は最後にトークンが検証された時刻を返します
func (s *SlackAuthManager) GetLastValidationTime() time.Time {
	return s.status.LastChecked
}

// ShouldRefresh はトークンをリフレッシュすべきかどうかを判定します
func (s *SlackAuthManager) ShouldRefresh() bool {
	// Slackトークンは通常期限切れにならないが、検証が失敗した場合をチェック
	return !s.status.IsValid && s.config.RefreshToken != ""
}

// GetServiceSpecificInfo はSlack固有の認証情報を返します
func (s *SlackAuthManager) GetServiceSpecificInfo() map[string]interface{} {
	return map[string]interface{}{
		"service":           "slack",
		"supports_refresh":  false,
		"auth_url_template": "https://slack.com/oauth/v2/authorize",
		"required_scopes":   []string{"chat:write", "channels:read", "users:read"},
		"token_type":        "Bearer",
	}
}
