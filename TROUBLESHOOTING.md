# トラブルシューティング

## IDEでコンパイルエラーが表示される場合

### 1. Go言語サーバー（gopls）のリスタート

VSCodeを使用している場合：
1. `Cmd+Shift+P` (Mac) または `Ctrl+Shift+P` (Windows/Linux)
2. "Go: Restart Language Server" を実行

### 2. Go Modulesの再初期化

```bash
go mod tidy
go mod download
go mod verify
```

### 3. Go言語サーバーのキャッシュクリア

```bash
go clean -modcache
go clean -cache
```

### 4. VSCodeの設定確認

`.vscode/settings.json` に以下を追加：

```json
{
    "go.useLanguageServer": true,
    "go.languageServerExperimentalFeatures": {
        "diagnostics": true,
        "documentLink": true
    },
    "go.lintTool": "golangci-lint",
    "go.buildOnSave": "package",
    "go.vetOnSave": "package"
}
```

### 5. 依存関係の確認

```bash
go list -m all
```

### 6. 手動でのコンパイル確認

```bash
# 全体のビルド
go build ./...

# 特定のパッケージ
go build ./internal/services/

# メインアプリケーション
go build -o worklogr cmd/worklogr/main.go
```

## 実際のエラーがある場合

### コンパイルエラーの確認

```bash
go build -v ./...
```

### 静的解析

```bash
go vet ./...
golangci-lint run
```

### テストの実行

```bash
go test ./...
```

## 現在の状態

- ✅ `go build` は成功
- ✅ `go vet` はエラーなし
- ✅ アプリケーションは正常動作
- ✅ 全てのコマンドが機能

IDEでエラーが表示されていても、実際のコンパイルは成功しているため、Go言語サーバーの表示上の問題と考えられます。
