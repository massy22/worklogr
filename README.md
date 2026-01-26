# worklogr

マルチサービス統合イベント収集CLIツール - AI報告書生成のためのデータ収集

## 概要

worklogrは、各種サービスから指定期間のイベントを取得し、報告書生成に最適化されたJSON/CSV形式で出力するGolang製CLIツールです。

## 特徴

- **マルチサービス対応**: Slack、GitHub、Google Calendarからイベントを収集
- **シンプル認証**: アクセストークンベースの簡単設定
- **SQLiteデータベース**: ローカルでのデータ管理とキャッシュ
- **AI最適化出力**: GPT-5やGeminiでの報告書生成に最適化されたJSON形式
- **時系列統合**: 複数サービスのイベントを時系列で統合
- **柔軟な出力形式**: JSON、CSV、AI最適化JSON、サマリー付きCSV
- **統一タイムゾーン**: 全サービスで統一されたタイムゾーン処理

## 対応サービスとイベント

### Slack
- メッセージ（チャンネル、DM、プライベートチャンネル）
- 全チャンネル横断検索による包括的収集

### GitHub
- コミット（自分が作成したもの）
- プルリクエスト作成/マージ/クローズ（自分が作成したもの）
- プルリクエストレビュー（自分が行ったレビューとコメント）
- Issue作成/クローズ（自分が作成/クローズしたもの）

### Google Calendar
- イベント作成/更新
- イベント参加
- 会議出席記録

## 非対応サービス

### Notion
- **対応していません**
- **理由**: Notion APIの制限により報告書に必要な正確な編集履歴情報が取得できないため
- **詳細**: 途中の編集者や編集過程の追跡が不可能で、最終更新者の情報のみ取得可能

## インストール

### 前提条件

- Go 1.21以上
- 各サービスのアクセストークン

### ビルド

```bash
git clone https://github.com/iriam/worklogr.git
cd worklogr
go mod tidy
go build -o worklogr cmd/worklogr/main.go
```

## 設定

### 1. 設定ファイルの作成

worklogrは以下の順序で設定ファイルを検索します：

1. **現在のディレクトリ**: `./config.yaml`
2. **ホームディレクトリ**: `~/.worklogr/config.yaml`

#### 設定ファイルの作成方法

**方法1: 現在のディレクトリで使用（推奨）**
```bash
# テンプレートをコピーして設定ファイルを作成
cp config.yaml.example config.yaml

# 設定ファイルを編集
vim config.yaml
```

**方法2: ホームディレクトリで使用**
```bash
# ホームディレクトリに設定ディレクトリを作成
mkdir -p ~/.worklogr

# テンプレートをコピー
cp config.yaml.example ~/.worklogr/config.yaml

# 設定ファイルを編集
vim ~/.worklogr/config.yaml
```

### 2. アクセストークンの取得

各サービスでアクセストークンを取得し、設定ファイルに記載してください。

#### Slack
1. [Slack API](https://api.slack.com/apps)でアプリを作成
2. OAuth & Permissions で以下のスコープを追加:
   - `search:read` (メッセージ検索用 - 必須)
3. User OAuth Token (xoxp-で始まる) をコピー

**注意**: worklogrはSlack Search APIを使用して全チャンネル（パブリック、プライベート、DM）からメッセージを効率的に収集します。`search:read`スコープのみで十分です。

#### GitHub
1. [GitHub Settings](https://github.com/settings/tokens) → Personal access tokens → Tokens (classic)
2. 新しいトークンを生成し、以下のスコープを選択:
   - `repo` (リポジトリアクセス)
   - `user:email` (ユーザー情報)
   - `read:org` (組織情報)
3. 生成されたトークン (ghp-で始まる) をコピー

#### Google Calendar
Google Calendarはgcloud認証のみを使用します：

```bash
# Google Cloud SDKのインストール（未インストールの場合）
# https://cloud.google.com/sdk/docs/install

# Google認証
gcloud auth login

# Calendar APIスコープ付きでApplication Default Credentials設定
gcloud auth application-default login --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/calendar.readonly,https://www.googleapis.com/auth/calendar.events.readonly,https://www.googleapis.com/auth/drive.readonly

# Google Cloud プロジェクトを設定（必須）
gcloud config set project YOUR_PROJECT_ID

# Application Default Credentialsのクォータプロジェクトを更新
gcloud auth application-default set-quota-project YOUR_PROJECT_ID

# Calendar APIを有効化
gcloud services enable calendar-json.googleapis.com

# Drive APIを有効化（Geminiメモ等の添付取得用）
gcloud services enable drive.googleapis.com

# gcloud認証状態の確認
./worklogr gcloud status
```

### 3. 設定ファイルの例

```yaml
# worklogr configuration file
# データベースパス（相対パスまたは絶対パス）
database_path: "./worklogr.db"

# タイムゾーン設定（全サービス共通）
# 有効な値: Asia/Tokyo, UTC, America/New_York, Europe/London など
# 詳細: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
timezone: "Asia/Tokyo"

# サービス設定
slack:
  enabled: true
  # Slack User Token (xoxp-で始まる)
  # 取得方法: https://api.slack.com/apps → Your Apps → OAuth & Permissions
  # 必要なスコープ: search:read
  access_token: "xoxp-your-slack-user-token"

github:
  enabled: true
  # GitHub Personal Access Token
  # 取得方法: GitHub Settings → Developer settings → Personal access tokens
  # 必要なスコープ: repo, user:email, read:org
  access_token: "ghp_your-github-token"

google_calendar:
  enabled: true
  # Google Calendar は gcloud 認証のみを使用
  # セットアップ: gcloud auth application-default login
```
