package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TokenValidator は異なるサービスのトークン検証機能を提供します
type TokenValidator struct {
	httpClient *http.Client // HTTPクライアント
}

// NewTokenValidator は新しいトークンバリデーターを作成します
func NewTokenValidator() *TokenValidator {
	return &TokenValidator{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ValidateSlackToken はSlackアクセストークンを検証します
func (tv *TokenValidator) ValidateSlackToken(ctx context.Context, token string) (*AuthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://slack.com/api/auth.test", nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗しました: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := tv.httpClient.Do(req)
	if err != nil {
		return &AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: fmt.Sprintf("ネットワークエラー: %v", err),
			TokenType:    "access",
		}, nil
	}
	defer resp.Body.Close()

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
		User  string `json:"user,omitempty"`
		Team  string `json:"team,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		return &AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: fmt.Sprintf("レスポンスの解析に失敗しました: %v", err),
			TokenType:    "access",
		}, nil
	}

	status := &AuthStatus{
		IsValid:     slackResp.OK,
		LastChecked: time.Now(),
		TokenType:   "access",
	}

	if !slackResp.OK {
		status.ErrorMessage = fmt.Sprintf("Slack APIエラー: %s", slackResp.Error)
	}

	return status, nil
}

// ValidateGitHubToken はGitHubアクセストークンを検証します
func (tv *TokenValidator) ValidateGitHubToken(ctx context.Context, token string) (*AuthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗しました: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := tv.httpClient.Do(req)
	if err != nil {
		return &AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: fmt.Sprintf("ネットワークエラー: %v", err),
			TokenType:    "access",
		}, nil
	}
	defer resp.Body.Close()

	status := &AuthStatus{
		IsValid:     resp.StatusCode == http.StatusOK,
		LastChecked: time.Now(),
		TokenType:   "access",
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			status.ErrorMessage = fmt.Sprintf("GitHub APIエラー: %s", errorResp.Message)
		} else {
			status.ErrorMessage = fmt.Sprintf("HTTPエラー: %d", resp.StatusCode)
		}
	}

	return status, nil
}

// ValidateGoogleToken はGoogleアクセストークンを検証します
func (tv *TokenValidator) ValidateGoogleToken(ctx context.Context, token string) (*AuthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v1/tokeninfo?access_token="+token, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗しました: %w", err)
	}

	resp, err := tv.httpClient.Do(req)
	if err != nil {
		return &AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: fmt.Sprintf("ネットワークエラー: %v", err),
			TokenType:    "access",
		}, nil
	}
	defer resp.Body.Close()

	var tokenInfo struct {
		Audience  string `json:"audience"`
		Scope     string `json:"scope"`
		ExpiresIn int    `json:"expires_in"`
		Error     string `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return &AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: fmt.Sprintf("レスポンスの解析に失敗しました: %v", err),
			TokenType:    "access",
		}, nil
	}

	status := &AuthStatus{
		IsValid:     resp.StatusCode == http.StatusOK && tokenInfo.Error == "",
		LastChecked: time.Now(),
		TokenType:   "access",
	}

	if tokenInfo.Error != "" {
		status.ErrorMessage = fmt.Sprintf("Google APIエラー: %s", tokenInfo.Error)
	} else if resp.StatusCode != http.StatusOK {
		status.ErrorMessage = fmt.Sprintf("HTTPエラー: %d", resp.StatusCode)
	} else if tokenInfo.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenInfo.ExpiresIn) * time.Second)
		status.ExpiresAt = &expiresAt
	}

	return status, nil
}

// ValidateTokenByService は特定のサービスのトークンを検証します
func (tv *TokenValidator) ValidateTokenByService(ctx context.Context, serviceName, token string) (*AuthStatus, error) {
	switch serviceName {
	case "slack":
		return tv.ValidateSlackToken(ctx, token)
	case "github":
		return tv.ValidateGitHubToken(ctx, token)
	case "calendar", "google":
		return tv.ValidateGoogleToken(ctx, token)
	default:
		return nil, fmt.Errorf("サポートされていないサービス: %s", serviceName)
	}
}

// HealthChecker は認証サービスのヘルスチェック機能を提供します
type HealthChecker struct {
	validator *TokenValidator // トークンバリデーター
}

// NewHealthChecker は新しいヘルスチェッカーを作成します
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		validator: NewTokenValidator(),
	}
}

// CheckServiceHealth はサービスの包括的なヘルスチェックを実行します
func (hc *HealthChecker) CheckServiceHealth(ctx context.Context, serviceName, token string) (*AuthValidationResult, error) {
	status, err := hc.validator.ValidateTokenByService(ctx, serviceName, token)
	if err != nil {
		return nil, fmt.Errorf("トークンの検証に失敗しました: %w", err)
	}

	result := &AuthValidationResult{
		ServiceName: serviceName,
		Status:      status,
		Suggestions: []string{},
	}

	// 検証結果に基づく提案を追加
	if !status.IsValid {
		result.Suggestions = append(result.Suggestions, "トークンが無効です。再認証を行ってください。")
		
		if status.ErrorMessage != "" {
			switch serviceName {
			case "slack":
				if contains(status.ErrorMessage, "invalid_auth") {
					result.Suggestions = append(result.Suggestions, "Slackアプリの権限を確認してください。")
				}
			case "github":
				if contains(status.ErrorMessage, "401") {
					result.Suggestions = append(result.Suggestions, "GitHubトークンの権限スコープを確認してください。")
				}
			case "calendar", "google":
				if contains(status.ErrorMessage, "invalid_token") {
					result.Suggestions = append(result.Suggestions, "Googleアカウントの認証を更新してください。")
				}
			}
		}
	} else {
		// トークンが有効な場合、有効期限の警告をチェック
		if status.ExpiresAt != nil {
			timeUntilExpiry := time.Until(*status.ExpiresAt)
			if timeUntilExpiry < 24*time.Hour {
				result.Suggestions = append(result.Suggestions, "トークンの有効期限が近づいています。更新を検討してください。")
			}
		}
	}

	return result, nil
}

// CheckAllServicesHealth は複数のサービスのヘルスチェックを実行します
func (hc *HealthChecker) CheckAllServicesHealth(ctx context.Context, tokens map[string]string) ([]*AuthValidationResult, error) {
	results := make([]*AuthValidationResult, 0, len(tokens))

	for serviceName, token := range tokens {
		result, err := hc.CheckServiceHealth(ctx, serviceName, token)
		if err != nil {
			// エラー情報を含む結果を作成
			result = &AuthValidationResult{
				ServiceName: serviceName,
				Status: &AuthStatus{
					IsValid:      false,
					LastChecked:  time.Now(),
					ErrorMessage: err.Error(),
					TokenType:    "access",
				},
				Suggestions: []string{"ヘルスチェック中にエラーが発生しました。"},
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// contains は文字列に部分文字列が含まれているかチェックします（大文字小文字を区別しない）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr ||
		      containsSubstring(s, substr))))
}

// containsSubstring はシンプルな部分文字列検索を実行します
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
