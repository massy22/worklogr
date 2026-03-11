package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokenStoreRoundTripAndRefreshTimestamp(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "tokens", "store.json")
	expiresAt := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	ts, err := NewTokenStore(storePath, "passphrase")
	if err != nil {
		t.Fatalf("NewTokenStore returned error: %v", err)
	}

	if err := ts.StoreToken("github", "access-token", "refresh-token", &expiresAt, []string{"repo", "user:email"}); err != nil {
		t.Fatalf("StoreToken returned error: %v", err)
	}

	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("failed to read token store: %v", err)
	}
	if strings.Contains(string(raw), "access-token") || strings.Contains(string(raw), "refresh-token") {
		t.Fatalf("expected token store file to keep tokens encrypted")
	}

	token, err := ts.GetToken("github")
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}

	if token.ServiceName != "github" {
		t.Fatalf("expected service github, got %q", token.ServiceName)
	}
	if token.AccessToken != "access-token" || token.RefreshToken != "refresh-token" {
		t.Fatalf("unexpected token payload: %+v", token)
	}
	if token.ExpiresAt == nil || !token.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expiresAt %v, got %+v", expiresAt, token.ExpiresAt)
	}
	if len(token.Scopes) != 2 || token.Scopes[0] != "repo" || token.Scopes[1] != "user:email" {
		t.Fatalf("unexpected token scopes: %+v", token.Scopes)
	}

	if err := ts.UpdateRefreshTime("github"); err != nil {
		t.Fatalf("UpdateRefreshTime returned error: %v", err)
	}

	refreshed, err := ts.GetToken("github")
	if err != nil {
		t.Fatalf("GetToken after refresh returned error: %v", err)
	}
	if refreshed.LastRefresh == nil {
		t.Fatalf("expected last refresh to be populated")
	}
}

func TestTokenStoreListAndDeleteServices(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "tokens", "store.json")

	ts, err := NewTokenStore(storePath, "passphrase")
	if err != nil {
		t.Fatalf("NewTokenStore returned error: %v", err)
	}

	if err := ts.StoreToken("slack", "slack-token", "", nil, []string{"search:read"}); err != nil {
		t.Fatalf("StoreToken slack returned error: %v", err)
	}
	if err := ts.StoreToken("github", "github-token", "", nil, []string{"repo"}); err != nil {
		t.Fatalf("StoreToken github returned error: %v", err)
	}

	services, err := ts.ListServices()
	if err != nil {
		t.Fatalf("ListServices returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d (%v)", len(services), services)
	}

	if err := ts.DeleteToken("slack"); err != nil {
		t.Fatalf("DeleteToken returned error: %v", err)
	}
	if _, err := ts.GetToken("slack"); err == nil {
		t.Fatalf("expected deleted token lookup to fail")
	}

	services, err = ts.ListServices()
	if err != nil {
		t.Fatalf("ListServices after delete returned error: %v", err)
	}
	if len(services) != 1 || services[0] != "github" {
		t.Fatalf("expected remaining service github, got %v", services)
	}
}
