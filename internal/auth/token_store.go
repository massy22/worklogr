package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// TokenStore は認証トークンの安全な保存と取得を管理します
type TokenStore struct {
	storePath string // ストアファイルのパス
	key       []byte // 暗号化キー
}

// StoredToken はセキュアストレージに保存されるトークンを表します
type StoredToken struct {
	ServiceName   string     `json:"service_name"`           // サービス名
	AccessToken   string     `json:"access_token"`           // アクセストークン
	RefreshToken  string     `json:"refresh_token,omitempty"` // リフレッシュトークン
	TokenType     string     `json:"token_type"`             // トークンタイプ
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`   // 有効期限
	Scopes        []string   `json:"scopes,omitempty"`       // 権限スコープ
	CreatedAt     time.Time  `json:"created_at"`             // 作成日時
	LastRefresh   *time.Time `json:"last_refresh,omitempty"` // 最後のリフレッシュ日時
	EncryptedData string     `json:"encrypted_data"`         // 暗号化されたデータ
}

// NewTokenStore は新しいセキュアトークンストアを作成します
func NewTokenStore(storePath string, passphrase string) (*TokenStore, error) {
	// ディレクトリが存在しない場合は作成
	dir := filepath.Dir(storePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("ストアディレクトリの作成に失敗しました: %w", err)
	}

	// パスフレーズから暗号化キーを生成
	hash := sha256.Sum256([]byte(passphrase))
	key := hash[:]

	return &TokenStore{
		storePath: storePath,
		key:       key,
	}, nil
}

// StoreToken はサービスのトークンを安全に保存します
func (ts *TokenStore) StoreToken(serviceName, accessToken, refreshToken string, expiresAt *time.Time, scopes []string) error {
	// トークンデータ構造を作成
	tokenData := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt,
		"scopes":        scopes,
	}

	// トークンデータをシリアライズ
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("トークンデータのシリアライズに失敗しました: %w", err)
	}

	// トークンデータを暗号化
	encryptedData, err := ts.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("トークンデータの暗号化に失敗しました: %w", err)
	}

	// 保存用トークン構造を作成
	storedToken := StoredToken{
		ServiceName:   serviceName,
		TokenType:     "access",
		CreatedAt:     time.Now(),
		EncryptedData: base64.StdEncoding.EncodeToString(encryptedData),
	}

	// 既存のトークンを読み込み
	tokens, err := ts.loadTokens()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("既存トークンの読み込みに失敗しました: %w", err)
	}

	// トークンを更新または追加
	found := false
	for i, token := range tokens {
		if token.ServiceName == serviceName {
			tokens[i] = storedToken
			found = true
			break
		}
	}
	if !found {
		tokens = append(tokens, storedToken)
	}

	// トークンをファイルに保存
	return ts.saveTokens(tokens)
}

// GetToken はサービスのトークンを取得して復号化します
func (ts *TokenStore) GetToken(serviceName string) (*StoredToken, error) {
	tokens, err := ts.loadTokens()
	if err != nil {
		return nil, fmt.Errorf("トークンの読み込みに失敗しました: %w", err)
	}

	// サービスのトークンを検索
	for _, token := range tokens {
		if token.ServiceName == serviceName {
			// トークンデータを復号化
			encryptedData, err := base64.StdEncoding.DecodeString(token.EncryptedData)
			if err != nil {
				return nil, fmt.Errorf("暗号化データのデコードに失敗しました: %w", err)
			}

			decryptedData, err := ts.decrypt(encryptedData)
			if err != nil {
				return nil, fmt.Errorf("トークンデータの復号化に失敗しました: %w", err)
			}

			// 復号化されたデータを解析
			var tokenData map[string]interface{}
			if err := json.Unmarshal(decryptedData, &tokenData); err != nil {
				return nil, fmt.Errorf("復号化されたトークンデータの解析に失敗しました: %w", err)
			}

			// トークンフィールドを設定
			if accessToken, ok := tokenData["access_token"].(string); ok {
				token.AccessToken = accessToken
			}
			if refreshToken, ok := tokenData["refresh_token"].(string); ok {
				token.RefreshToken = refreshToken
			}
			if scopes, ok := tokenData["scopes"].([]interface{}); ok {
				token.Scopes = make([]string, len(scopes))
				for i, scope := range scopes {
					if s, ok := scope.(string); ok {
						token.Scopes[i] = s
					}
				}
			}
			if expiresAtStr, ok := tokenData["expires_at"].(string); ok && expiresAtStr != "" {
				if expiresAt, err := time.Parse(time.RFC3339, expiresAtStr); err == nil {
					token.ExpiresAt = &expiresAt
				}
			}

			return &token, nil
		}
	}

	return nil, fmt.Errorf("サービス %s のトークンが見つかりません", serviceName)
}

// DeleteToken はサービスのトークンを削除します
func (ts *TokenStore) DeleteToken(serviceName string) error {
	tokens, err := ts.loadTokens()
	if err != nil {
		return fmt.Errorf("トークンの読み込みに失敗しました: %w", err)
	}

	// サービスのトークンを除去
	filteredTokens := make([]StoredToken, 0, len(tokens))
	for _, token := range tokens {
		if token.ServiceName != serviceName {
			filteredTokens = append(filteredTokens, token)
		}
	}

	return ts.saveTokens(filteredTokens)
}

// ListServices は保存されたトークンを持つサービスのリストを返します
func (ts *TokenStore) ListServices() ([]string, error) {
	tokens, err := ts.loadTokens()
	if err != nil {
		return nil, fmt.Errorf("トークンの読み込みに失敗しました: %w", err)
	}

	services := make([]string, len(tokens))
	for i, token := range tokens {
		services[i] = token.ServiceName
	}

	return services, nil
}

// UpdateRefreshTime はサービストークンの最後のリフレッシュ時刻を更新します
func (ts *TokenStore) UpdateRefreshTime(serviceName string) error {
	tokens, err := ts.loadTokens()
	if err != nil {
		return fmt.Errorf("トークンの読み込みに失敗しました: %w", err)
	}

	// リフレッシュ時刻を更新
	for i, token := range tokens {
		if token.ServiceName == serviceName {
			now := time.Now()
			tokens[i].LastRefresh = &now
			break
		}
	}

	return ts.saveTokens(tokens)
}

// encrypt はAES-GCMを使用してデータを暗号化します
func (ts *TokenStore) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(ts.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decrypt はAES-GCMを使用してデータを復号化します
func (ts *TokenStore) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(ts.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("暗号文が短すぎます")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// loadTokens はストアファイルからトークンを読み込みます
func (ts *TokenStore) loadTokens() ([]StoredToken, error) {
	data, err := os.ReadFile(ts.storePath)
	if err != nil {
		return nil, err
	}

	var tokens []StoredToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("トークンストアの解析に失敗しました: %w", err)
	}

	return tokens, nil
}

// saveTokens はトークンをストアファイルに保存します
func (ts *TokenStore) saveTokens(tokens []StoredToken) error {
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("トークンのシリアライズに失敗しました: %w", err)
	}

	return os.WriteFile(ts.storePath, data, 0600)
}
