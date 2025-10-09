# worklogr 開発ドキュメント

## プロジェクト概要

worklogrは、各種サービスから指定期間のイベントを収集し、AI報告書生成に最適化されたJSON/CSV形式で出力するGolang製CLIツールです。

## アーキテクチャ

### ディレクトリ構造

```
worklogr/
├── cmd/worklogr/main.go          # エントリーポイント
├── internal/
│   ├── auth/                     # 認証関連
│   │   └── gcloud.go            # Google Cloud認証
│   ├── collector/               # イベント収集エンジン
│   │   └── collector.go         # メインコレクター
│   ├── config/                  # 設定管理
│   │   └── config.go            # 設定構造体とロード
│   ├── database/                # データベース層
│   │   └── sqlite.go            # SQLite操作
│   ├── exporter/                # 出力形式
│   │   ├── csv.go               # CSV出力
│   │   └── json.go              # JSON出力
│   ├── services/                # 各サービス連携
│   │   ├── calendar.go          # Google Calendar
│   │   ├── github.go            # GitHub
│   │   ├── slack.go             # Slack
│   │   └── retry_client.go      # リトライ機能
│   └── utils/                   # ユーティリティ
│       └── timezone.go          # タイムゾーン管理
├── config.yaml.example         # 設定ファイル例
├── .gitignore                   # Git除外設定
└── README.md                    # ユーザー向けドキュメント
```

## 現在の実装状況

### ✅ 実装済み

#### Slack
- **認証**: User OAuth Token (xoxp-)
- **収集方式**: Search API使用
- **収集内容**: 全チャンネル（パブリック、プライベート、DM）のメッセージ
- **最適化**: 効率的な検索クエリ

#### GitHub
- **認証**: Personal Access Token (ghp-)
- **収集方式**: Search API使用（大幅高速化済み）
- **収集内容**:
  - 自分のコミット
  - 自分が作成したPR（作成/マージ/クローズ）
  - 自分が作成/クローズしたIssue
  - 自分が行ったPRレビュー（コメント本文付き）
- **パフォーマンス**: N×5回のAPIコール → 4〜5回に削減

#### Google Calendar
- **認証**: gcloud認証のみ（OAuth設定不要）
- **収集内容**: イベント作成/更新/参加
- **設定**: Application Default Credentials使用

#### データベース
- **SQLite**: ローカルデータ管理
- **スキーマ**: イベント統一形式
- **機能**: 重複排除、時系列ソート

#### 出力形式
- **JSON**: AI最適化形式
- **CSV**: 表形式
- **統一タイムゾーン**: 全サービス対応

## 非対応サービス

### Notion
- **対応していません**
- **理由**: Notion APIの制限により報告書に必要な正確な編集履歴情報が取得できないため
- **詳細**: 途中の編集者や編集過程の追跡が不可能で、最終更新者の情報のみ取得可能

## 技術的詳細

### 認証方式

| サービス | 認証方式 | 設定場所 | 備考 |
|---------|---------|---------|------|
| Slack | User OAuth Token | config.yaml | xoxp-で始まる |
| GitHub | Personal Access Token | config.yaml | ghp-で始まる |
| Google Calendar | gcloud認証 | なし | ADC使用 |

### パフォーマンス最適化

#### GitHub Search API活用
```
従来: 全リポジトリ取得 → 各リポジトリで5種類のAPI実行
新方式: Search APIで期間とユーザー指定で直接検索
結果: N×5回 → 4〜5回のAPIコールに削減
```

#### 検索クエリ例
```
commits: author:username created:2024-01-01..2024-01-02
issues: author:username type:issue created:2024-01-01..2024-01-02
PRs: author:username type:pr merged:2024-01-01..2024-01-02
reviews: reviewed-by:username type:pr updated:2024-01-01..2024-01-02
```

### データ構造

#### 統一イベント形式
```go
type Event struct {
    ID        string    `json:"id"`
    Service   string    `json:"service"`
    Type      string    `json:"type"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
    UserID    string    `json:"user_id"`
    Metadata  string    `json:"metadata"`
}
```

## 開発ガイド

### 環境セットアップ

1. **Go環境**: Go 1.21以上
2. **依存関係**: `go mod tidy`
3. **ビルド**: `go build -o worklogr cmd/worklogr/main.go`

### 設定ファイル

```yaml
database_path: "./worklogr.db"
timezone: "Asia/Tokyo"

slack:
  enabled: true
  access_token: "xoxp-your-token"

github:
  enabled: true
  access_token: "ghp_your-token"

google_calendar:
  enabled: true
  # gcloud認証のみ
```

### テスト実行

```bash
# 設定確認
./worklogr config show

# イベント収集（テスト）
./worklogr collect --start "2024-09-20" --end "2024-09-21" --services github

# 出力確認
./worklogr export --format json --start "2024-09-20" --end "2024-09-21"
```

## 今後の開発タスク

### 優先度High

1. **エラーハンドリング強化**
   - レート制限対応
   - ネットワークエラー処理
   - リトライ機能強化

### 優先度Medium

2. **新サービス対応**
   - Microsoft Teams
   - Discord
   - Jira

3. **出力形式拡張**
   - Markdown形式
   - AI特化形式の改善

### 優先度Low

4. **パフォーマンス改善**
   - 並列処理
   - キャッシュ機能

5. **UI改善**
   - プログレスバー
   - 詳細ログ

### 非対応（技術的制限）

6. **Notion対応**
   - API制限により対応困難
   - 編集履歴の不完全性
   - 報告書生成には不適切

## トラブルシューティング

### よくある問題

1. **GitHub API制限**
   - 解決: Personal Access Tokenの権限確認
   - 参考: `TROUBLESHOOTING.md`

2. **Slack認証エラー**
   - 解決: `search:read`スコープの確認
   - 参考: `SLACK_TROUBLESHOOTING.md`

3. **Google Calendar認証**
   - 解決: `gcloud auth application-default login`実行
   - 確認: `./worklogr gcloud status`

### デバッグ方法

```bash
# 詳細ログ出力
./worklogr collect --start "2024-09-20" --end "2024-09-21" --services github -v

# 設定確認
./worklogr config show

# データベース確認
sqlite3 worklogr.db "SELECT * FROM events LIMIT 5;"
```

## Cline開発再開時のチェックリスト

### 1. 環境確認
- [ ] Go 1.21以上がインストール済み
- [ ] 依存関係が最新（`go mod tidy`）
- [ ] ビルドが成功（`go build -o worklogr cmd/worklogr/main.go`）

### 2. 設定確認
- [ ] `config.yaml`が存在し、適切に設定済み
- [ ] 各サービスのアクセストークンが有効
- [ ] Google Calendar認証が有効（`./worklogr gcloud status`）

### 3. 動作確認
- [ ] `./worklogr config show`で設定表示
- [ ] `./worklogr collect`でイベント収集テスト
- [ ] `./worklogr export`で出力テスト

### 4. 開発環境
- [ ] VSCodeまたは適切なエディタ
- [ ] Go拡張機能インストール済み
- [ ] `.gitignore`が適切に設定済み

### 5. 既知の制限事項
- [ ] Notionは現在無効化中（ライブラリ互換性問題）
- [ ] GitHub Search APIのレート制限に注意
- [ ] Google Calendar認証はgcloudのみ

## 参考資料

- [GitHub API Documentation](https://docs.github.com/en/rest)
- [Slack API Documentation](https://api.slack.com/)
- [Google Calendar API](https://developers.google.com/calendar)
- [Go Documentation](https://golang.org/doc/)

## 最終更新

- **日付**: 2024-09-21
- **バージョン**: v1.0.0
- **主要変更**: GitHub Search API最適化、Google Calendar gcloud認証化、Notion一時無効化
