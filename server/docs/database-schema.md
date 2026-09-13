# PostgreSQL スキーマ

[ER図（認証・家計・相談データ）](../../docs/ER_DIAGRAM.md)でテーブル間の関係と全カラムを確認できます。

実行時の正本は`internal/infrastructure/persistence/postgres/001_init.sql`と`002_auth.sql`です。Go APIは起動時に未適用バージョンをトランザクション内で適用します。[認証・既存データ移行](authentication.md)も参照してください。ローカルではDocker ComposeのPostgreSQL 16に接続します。

```sh
docker compose up -d postgres
export DATABASE_URL='postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable'
psql "$DATABASE_URL" -c '\\dt'
```

## テーブル

| テーブル | 役割 | 主な制約 |
| --- | --- | --- |
| `users` | DB直接登録のアカウント | ユーザー名一意、password_hash、is_active |
| `auth_sessions` | 失効可能なセッション | トークンのSHA-256、user_id、期限、資格情報スナップショット |
| `auth_attempts` | 共有ログイン回数制限 | ハッシュ化キー、回数、期間終端 |
| `schema_migrations` | 適用済みスキーマバージョン | `version` 主キー |
| `profiles` | ユーザーごとの家計プロフィール | `id = 1`、`user_id` 一意 |
| `transactions` | CSVから取り込む支出・入金明細 | `(user_id, fingerprint)` 一意、日付・カテゴリ索引 |
| `merchant_rules` | 加盟店正規化名とカテゴリのローカル規則 | `(user_id, normalized_merchant)` 一意 |
| `consultations` | 支出相談、助言、verdict、支出後結果 | `id` は `BIGSERIAL` |
| `consultation_messages` | 相談に紐づく会話 | `consultation_id` 外部キー、親削除時に連鎖削除 |
| `memories` | 再利用する判断傾向 | `id` は `BIGSERIAL` |
| `monthly_reviews` | 月次レビューと候補JSON | `id` は `BIGSERIAL` |

金額はすべて円の整数として`BIGINT`に保存します。JSON形式の業務データは`TEXT`列へJSON文字列として保存します。業務時刻はRFC 3339文字列、認証関連日時とスキーマ適用日時は`TIMESTAMPTZ`です。

すべての業務テーブルに`user_id`外部キーを追加しています。既存データ保全のためNULLを許容しますが、API経由の作成では認証済み所有者を必ず設定します。所有者NULLのデータはAPIからアクセスできません。相談メッセージは`(consultation_id, user_id)`の複合外部キーでも所有者を検証します。

## ローカルDBの操作

状態を確認する場合:

```sh
docker compose ps
psql "$DATABASE_URL" -c '\\dt'
psql "$DATABASE_URL" -c 'SELECT version, applied_at FROM schema_migrations ORDER BY version'
```

開発データを初期化する必要がある場合は、APIを停止した上で、対象のComposeボリュームを明示して削除してください。これはデータを復元できない操作です。

```sh
docker compose down -v
```

`internal/infrastructure/persistence/sqlite` と `server/migrations/` は既存のRepositoryテストを維持するために残しています。ローカルAPIとAWS配備で使うDBはPostgreSQLであり、新しいアプリケーションデータをSQLiteへ保存しません。
