package auth

import (
	"context"
	"time"
)

// AuthStatus はサービスの認証状態を表します
type AuthStatus struct {
	IsValid      bool       `json:"is_valid"`      // 認証が有効かどうか
	ExpiresAt    *time.Time `json:"expires_at,omitempty"` // トークンの有効期限
	LastChecked  time.Time  `json:"last_checked"`  // 最後に確認した時刻
	ErrorMessage string     `json:"error_message,omitempty"` // エラーメッセージ
	TokenType    string     `json:"token_type"`    // トークンタイプ: "access", "refresh", "gcloud"
}

// AuthManager は統一認証管理のためのインターフェースです
type AuthManager interface {
	ValidateToken(ctx context.Context) (*AuthStatus, error) // トークンを検証する
	RefreshToken(ctx context.Context) error                 // トークンをリフレッシュする
	GetAuthStatus() *AuthStatus                              // 認証状態を取得する
	IsTokenExpired() bool                                    // トークンが期限切れかチェックする
	GetTokenInfo() *TokenInfo                                // トークン情報を取得する
}

// TokenInfo は認証トークンのメタデータを含みます
type TokenInfo struct {
	TokenType    string     `json:"token_type"`              // トークンタイプ
	Scopes       []string   `json:"scopes,omitempty"`        // 権限スコープ
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`    // 有効期限
	RefreshToken string     `json:"-"`                       // リフレッシュトークン（シリアライズしない）
	LastRefresh  *time.Time `json:"last_refresh,omitempty"`  // 最後のリフレッシュ時刻
}

// AuthConfig は認証の拡張設定です
type AuthConfig struct {
	AccessToken            string        `yaml:"access_token"`             // アクセストークン
	RefreshToken           string        `yaml:"refresh_token"`            // リフレッシュトークン
	TokenExpiresAt         *time.Time    `yaml:"token_expires_at,omitempty"` // トークン有効期限
	AutoRefresh            bool          `yaml:"auto_refresh"`             // 自動リフレッシュ
	ValidationInterval     time.Duration `yaml:"validation_interval"`      // 検証間隔
	MaxRetries             int           `yaml:"max_retries"`              // 最大リトライ回数
	RetryBackoffMultiplier float64       `yaml:"retry_backoff_multiplier"` // リトライバックオフ倍率
}

// AuthValidationResult は検証結果を含みます
type AuthValidationResult struct {
	ServiceName string      `json:"service_name"`        // サービス名
	Status      *AuthStatus `json:"status"`              // 認証状態
	Suggestions []string    `json:"suggestions,omitempty"` // 提案事項
}

// BaseAuthManager はすべての認証マネージャーに共通の機能を提供します
type BaseAuthManager struct {
	serviceName string      // サービス名
	config      *AuthConfig // 認証設定
	status      *AuthStatus // 認証状態
}

// NewBaseAuthManager は新しいベース認証マネージャーを作成します
func NewBaseAuthManager(serviceName string, config *AuthConfig) *BaseAuthManager {
	return &BaseAuthManager{
		serviceName: serviceName,
		config:      config,
		status: &AuthStatus{
			IsValid:     false,
			LastChecked: time.Now(),
			TokenType:   "access",
		},
	}
}

// GetAuthStatus は現在の認証状態を返します
func (b *BaseAuthManager) GetAuthStatus() *AuthStatus {
	return b.status
}

// IsTokenExpired はトークンが期限切れかどうかをチェックします
func (b *BaseAuthManager) IsTokenExpired() bool {
	if b.config.TokenExpiresAt == nil {
		return false
	}
	return time.Now().After(*b.config.TokenExpiresAt)
}

// GetTokenInfo はトークンのメタデータを返します
func (b *BaseAuthManager) GetTokenInfo() *TokenInfo {
	return &TokenInfo{
		TokenType:   b.status.TokenType,
		ExpiresAt:   b.config.TokenExpiresAt,
		LastRefresh: nil, // 具体的な実装で設定される
	}
}

// UpdateStatus は認証状態を更新します
func (b *BaseAuthManager) UpdateStatus(isValid bool, errorMessage string) {
	b.status.IsValid = isValid
	b.status.LastChecked = time.Now()
	b.status.ErrorMessage = errorMessage
	b.status.ExpiresAt = b.config.TokenExpiresAt
}

// GetServiceName はサービス名を返します
func (b *BaseAuthManager) GetServiceName() string {
	return b.serviceName
}

// GetConfig は認証設定を返します
func (b *BaseAuthManager) GetConfig() *AuthConfig {
	return b.config
}
