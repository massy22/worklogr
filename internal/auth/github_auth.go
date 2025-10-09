package auth

import (
	"context"
	"fmt"
	"time"
)

// GitHubAuthManager manages GitHub authentication
type GitHubAuthManager struct {
	*BaseAuthManager
	validator *TokenValidator
	store     *TokenStore
}

// NewGitHubAuthManager creates a new GitHub authentication manager
func NewGitHubAuthManager(config *AuthConfig, store *TokenStore) *GitHubAuthManager {
	return &GitHubAuthManager{
		BaseAuthManager: NewBaseAuthManager("github", config),
		validator:       NewTokenValidator(),
		store:          store,
	}
}

// ValidateToken validates the GitHub access token
func (g *GitHubAuthManager) ValidateToken(ctx context.Context) (*AuthStatus, error) {
	if g.config.AccessToken == "" {
		g.UpdateStatus(false, "アクセストークンが設定されていません")
		return g.GetAuthStatus(), nil
	}

	// Check if token is expired
	if g.IsTokenExpired() {
		g.UpdateStatus(false, "トークンの有効期限が切れています")
		return g.GetAuthStatus(), nil
	}

	// Validate token with GitHub API
	status, err := g.validator.ValidateGitHubToken(ctx, g.config.AccessToken)
	if err != nil {
		g.UpdateStatus(false, fmt.Sprintf("トークン検証エラー: %v", err))
		return g.GetAuthStatus(), err
	}

	// Update internal status
	g.UpdateStatus(status.IsValid, status.ErrorMessage)
	
	// Store token if valid
	if status.IsValid && g.store != nil {
		if err := g.store.StoreToken("github", g.config.AccessToken, g.config.RefreshToken, g.config.TokenExpiresAt, []string{"repo", "user"}); err != nil {
			// Log error but don't fail validation
			fmt.Printf("Warning: Failed to store GitHub token: %v\n", err)
		}
	}

	return g.GetAuthStatus(), nil
}

// RefreshToken refreshes the GitHub access token
func (g *GitHubAuthManager) RefreshToken(ctx context.Context) error {
	if g.config.RefreshToken == "" {
		return fmt.Errorf("リフレッシュトークンが設定されていません")
	}

	// GitHub personal access tokens don't expire automatically
	// This would typically require generating a new token
	return fmt.Errorf("GitHubトークンのリフレッシュには新しいトークンの生成が必要です")
}

// GetTokenInfo returns GitHub token information
func (g *GitHubAuthManager) GetTokenInfo() *TokenInfo {
	info := g.BaseAuthManager.GetTokenInfo()
	info.Scopes = []string{"repo", "user", "read:org"}
	return info
}

// IsHealthy checks if the GitHub authentication is healthy
func (g *GitHubAuthManager) IsHealthy(ctx context.Context) bool {
	status, err := g.ValidateToken(ctx)
	return err == nil && status.IsValid
}

// GetAuthURL returns the GitHub OAuth authorization URL
func (g *GitHubAuthManager) GetAuthURL(clientID, redirectURI string, scopes []string) string {
	if len(scopes) == 0 {
		scopes = []string{"repo", "user", "read:org"}
	}
	
	scopeStr := ""
	for i, scope := range scopes {
		if i > 0 {
			scopeStr += "%20"
		}
		scopeStr += scope
	}
	
	return fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&scope=%s&redirect_uri=%s", 
		clientID, scopeStr, redirectURI)
}

// UpdateTokenFromStore updates the token from secure storage
func (g *GitHubAuthManager) UpdateTokenFromStore() error {
	if g.store == nil {
		return fmt.Errorf("トークンストアが設定されていません")
	}

	storedToken, err := g.store.GetToken("github")
	if err != nil {
		return fmt.Errorf("ストアからのトークン取得に失敗: %w", err)
	}

	g.config.AccessToken = storedToken.AccessToken
	g.config.RefreshToken = storedToken.RefreshToken
	g.config.TokenExpiresAt = storedToken.ExpiresAt

	return nil
}

// ClearToken clears the stored token
func (g *GitHubAuthManager) ClearToken() error {
	g.config.AccessToken = ""
	g.config.RefreshToken = ""
	g.config.TokenExpiresAt = nil
	
	g.UpdateStatus(false, "トークンがクリアされました")

	if g.store != nil {
		return g.store.DeleteToken("github")
	}

	return nil
}

// GetLastValidationTime returns the last time the token was validated
func (g *GitHubAuthManager) GetLastValidationTime() time.Time {
	return g.status.LastChecked
}

// ShouldRefresh determines if the token should be refreshed
func (g *GitHubAuthManager) ShouldRefresh() bool {
	// GitHub personal access tokens don't typically expire
	// but check if validation failed
	return !g.status.IsValid && g.config.RefreshToken != ""
}

// GetServiceSpecificInfo returns GitHub-specific authentication information
func (g *GitHubAuthManager) GetServiceSpecificInfo() map[string]interface{} {
	return map[string]interface{}{
		"service":           "github",
		"supports_refresh":  false,
		"auth_url_template": "https://github.com/login/oauth/authorize",
		"required_scopes":   []string{"repo", "user", "read:org"},
		"token_type":        "token",
		"token_url":         "https://github.com/settings/tokens",
	}
}

// GetRepositoryAccess checks if the token has access to repositories
func (g *GitHubAuthManager) GetRepositoryAccess(ctx context.Context) (bool, error) {
	if !g.status.IsValid {
		return false, fmt.Errorf("トークンが無効です")
	}

	// This would typically make an API call to check repository access
	// For now, we'll assume access if the token is valid
	return true, nil
}

// GetUserInfo retrieves GitHub user information
func (g *GitHubAuthManager) GetUserInfo(ctx context.Context) (map[string]interface{}, error) {
	if !g.status.IsValid {
		return nil, fmt.Errorf("トークンが無効です")
	}

	// This would typically make an API call to get user info
	// For now, return basic info
	return map[string]interface{}{
		"service": "github",
		"valid":   true,
	}, nil
}
