# Slack User Token トラブルシューティング

## invalid_auth エラーの解決方法

### 1. User Token Scopes の確認・設定

#### 手順
1. https://api.slack.com/apps にアクセス
2. 作成したアプリを選択
3. 左メニューから「OAuth & Permissions」を選択
4. 「User Token Scopes」セクションを確認

#### 必要なスコープ（全て追加してください）
```
channels:history    - Read messages in public channels
channels:read       - View basic information about public channels
groups:history      - Read messages in private channels
groups:read         - View basic information about private channels
im:history          - Read messages in direct messages
im:read             - View basic information about direct messages
mpim:history        - Read messages in group direct messages
mpim:read           - View basic information about group direct messages
users:read          - View people in a workspace
reactions:read      - View emoji reactions and their associated content
```

### 2. アプリの再インストール

スコープを追加した後は、必ずアプリを再インストールしてください：

1. 「OAuth & Permissions」ページで「Reinstall to Workspace」をクリック
2. 権限を確認して「Allow」をクリック
3. 新しい User OAuth Token をコピー

### 3. トークンの更新

新しいトークンを config.yaml に設定：
```yaml
slack:
  enabled: true
  access_token: "新しいxoxp-トークン"
```

### 4. 検証

```bash
./worklogr okta validate
./worklogr status
```

## よくある問題

### 問題1: スコープが不足している
**症状**: invalid_auth エラー
**解決**: 上記の10個のスコープを全て追加

### 問題2: アプリを再インストールしていない
**症状**: スコープを追加してもエラーが続く
**解決**: 「Reinstall to Workspace」を実行

### 問題3: Bot Token を使用している
**症状**: xoxb- で始まるトークンを使用
**解決**: User OAuth Token (xoxp-) を使用

### 問題4: ワークスペースの権限不足
**症状**: アプリのインストールができない
**解決**: ワークスペース管理者に相談

## 確認コマンド

```bash
# トークン検証
./worklogr okta validate

# 設定確認
./worklogr config show

# サービス状態確認
./worklogr status

# テスト収集
./worklogr collect --start yesterday --end today
