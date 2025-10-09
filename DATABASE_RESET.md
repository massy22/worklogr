# データベースリセット方法

## 概要

worklogrのデータベースをリセットする方法を説明します。

## 方法1: 物理削除（推奨）

### 手順
```bash
# データベースファイルを削除
rm worklogr.db

# 確認
./worklogr status
```

### メリット
- ✅ 最も簡単で確実
- ✅ ディスク容量を完全に解放
- ✅ データベースファイルが新規作成される

## 方法2: SQLiteコマンドでテーブル削除

### 手順
```bash
# SQLiteでデータベースに接続
sqlite3 worklogr.db

# テーブル一覧確認
.tables

# 全テーブルのデータを削除
DELETE FROM events;

# 確認
SELECT COUNT(*) FROM events;

# 終了
.quit
```

### メリット
- ✅ テーブル構造は保持
- ✅ 特定のサービスのみ削除可能

## 方法3: 特定サービスのデータのみ削除

### 手順
```bash
# SQLiteでデータベースに接続
sqlite3 worklogr.db

# Slackのデータのみ削除
DELETE FROM events WHERE service = 'slack';

# 確認
SELECT service, COUNT(*) FROM events GROUP BY service;

# 終了
.quit
```

### メリット
- ✅ 他のサービスのデータは保持
- ✅ 部分的なリセットが可能

## 使用場面

### 完全リセットが必要な場合
- 設定変更後（例：ユーザーメッセージのみ収集に変更）
- 大量の不要データが混入した場合
- データベース構造を変更した場合

### 部分リセットが適している場合
- 特定のサービスのみ再収集したい場合
- 特定期間のデータのみ削除したい場合

## 注意事項

### バックアップ
重要なデータがある場合は、事前にバックアップを作成：
```bash
# データベースのバックアップ
cp worklogr.db worklogr.db.backup

# 復元（必要な場合）
cp worklogr.db.backup worklogr.db
```

### 設定ファイルは削除されない
データベースリセットでは設定ファイル（config.yaml）は影響を受けません。

## リセット後の確認

### 1. ステータス確認
```bash
./worklogr status
```

期待される結果：
```
Database Statistics:
===================
Total          : 0 events
```

### 2. 再収集テスト
```bash
# 短期間でテスト収集
./worklogr collect --start "1h" --end "now"

# 結果確認
./worklogr status
```

## トラブルシューティング

### データベースファイルが削除できない場合
```bash
# プロセスが使用中の可能性
lsof worklogr.db

# 強制削除
sudo rm worklogr.db
```

### 権限エラーの場合
```bash
# 権限確認
ls -la worklogr.db

# 権限変更
chmod 644 worklogr.db
