# PostgreSQL スキーマ

[ER図（認証・家計・相談データ）](../../docs/ER_DIAGRAM.md)でテーブル間の関係と全カラムを確認できます。

実行時の正本は`internal/infrastructure/persistence/postgres/`の番号付きSQLです。Go APIは起動時に未適用バージョンをトランザクション内で適用します。[認証・既存データ移行](authentication.md)も参照してください。ローカルではDocker ComposeのPostgreSQL 16に接続します。

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
| `service_settings` | 全利用者向けの運用設定 | `key` 主キー、値は0以上 |
| `user_consultation_limits` | 利用者別の月間相談上限 | `user_id` 主キー・外部キー、値は0以上 |
| `monthly_consultation_usage` | 日本時間の月ごとの新規相談数 | `(user_id, month)` 主キー、同時更新をupsertで直列化 |
| `admin_notifications` | 上限到達通知の重複防止と成否 | `(user_id, month, notification_type)` 主キー |
| `llm_usage` | OpenAIの呼出単位のtoken数 | 利用者・操作・モデル・token数・発生時刻のみ |
| `llm_model_prices` | モデル単価の履歴 | `(model, effective_from)` 主キー、公式資料URLと確認日時 |
| `monthly_operating_costs` | 月次のインフラ費用とサポート実績 | `month` 主キー、USD、件数、分数 |
| `profiles` | ユーザーごとの家計プロフィール | `id = 1`、`user_id` 一意 |
| `transactions` | CSVから取り込む支出・入金明細 | `(user_id, fingerprint)` 一意、日付・カテゴリ索引 |
| `merchant_rules` | 加盟店正規化名とカテゴリのローカル規則 | `(user_id, normalized_merchant)` 一意 |
| `consultations` | 支出相談、助言、verdict、支出後結果 | `id` は `BIGSERIAL` |
| `consultation_messages` | 相談に紐づく会話 | `consultation_id` 外部キー、親削除時に連鎖削除 |
| `memories` | 再利用する判断傾向 | `id` は `BIGSERIAL` |
| `monthly_reviews` | 月次レビューと候補JSON | `id` は `BIGSERIAL` |

金額はすべて円の整数として`BIGINT`に保存します。JSON形式の業務データは`TEXT`列へJSON文字列として保存します。業務時刻はRFC 3339文字列、認証関連日時とスキーマ適用日時は`TIMESTAMPTZ`です。

すべての業務テーブルに`user_id`外部キーを追加しています。既存データ保全のためNULLを許容しますが、API経由の作成では認証済み所有者を必ず設定します。所有者NULLのデータはAPIからアクセスできません。相談メッセージは`(consultation_id, user_id)`の複合外部キーでも所有者を検証します。

## 月間相談上限の変更

月間相談上限は、利用者別の値があればそれを優先し、なければ`default_monthly_consultation_limit`を使います。0は外部AIを利用しない設定として有効です。負数はDB制約で拒否されます。

```sql
-- 全利用者向けの既定値を変更する
UPDATE service_settings
SET integer_value = 30, updated_at = CURRENT_TIMESTAMP
WHERE key = 'default_monthly_consultation_limit';

-- user_id = 42だけ月12回へ変更する
INSERT INTO user_consultation_limits(user_id, monthly_limit)
VALUES (42, 12)
ON CONFLICT (user_id) DO UPDATE
SET monthly_limit = EXCLUDED.monthly_limit,
    updated_at = CURRENT_TIMESTAMP;

-- 利用者別設定を解除し、全体の既定値へ戻す
DELETE FROM user_consultation_limits WHERE user_id = 42;
```

初版は管理者がDBで変更します。管理者画面・管理者APIからの変更は後続課題として扱います。

無料利用者数は`service_settings.free_user_limit`で管理し、初期値は30人です。31人目の有効ユーザー登録、または上限到達後の再有効化はDB triggerが拒否します。利用停止したユーザーは人数へ含みません。

新規相談は`monthly_consultation_usage`へ日本時間の`YYYY-MM`単位で予約します。30回目までは外部モデルを利用でき、31回目以降はGoのfallbackを使います。上限到達後は追加メッセージ、メモリ抽出、月次レビューもfallbackを使います。追加メッセージ自体は件数を増やしません。相談の保存前に処理が失敗した場合は予約を戻します。

上限到達通知は`admin_notifications`で利用者・月ごとに一意化します。PagerDuty送信成功後は`delivered`、失敗時はエラー本文を`failed`として記録し、次の上限超過相談で再試行します。

## 月次原価の集計

OpenAI応答のtoken数は`llm_usage`へ保存します。相談文、家計情報、モデル出力は保存しません。月末にAWS等の実請求額とサポート実績を登録します。

```sql
INSERT INTO monthly_operating_costs(
  month, infrastructure_cost_usd, support_case_count, support_minutes, notes
) VALUES ('2026-09', 0, 0, 0, 'AWS請求確定後に実額へ更新')
ON CONFLICT (month) DO UPDATE SET
  infrastructure_cost_usd = EXCLUDED.infrastructure_cost_usd,
  support_case_count = EXCLUDED.support_case_count,
  support_minutes = EXCLUDED.support_minutes,
  notes = EXCLUDED.notes,
  recorded_at = CURRENT_TIMESTAMP;
```

登録後、[月次原価集計SQL](monthly-cost-report.sql)を実行します。

```sh
psql "$DATABASE_URL" -v report_month=2026-09 -f docs/monthly-cost-report.sql
```

分母は、その月に新しい相談を1回以上開始した利用者です。結果には月次サマリーと利用者別内訳を同じ行で返し、対象月の実利用者が0人でも`user_id`がNULLのサマリー行を1行返します。金額換算するのはOpenAIとインフラの実費で、サポートは合意どおり時間を利用者数で按分して併記します。

`monthly_unpriced_api_call_count`が1件以上の場合、モデル単価が未登録の呼出しを0ドルと誤認しないよう、`monthly_measured_cash_cost_usd`と平均原価をNULLにします。利用者別の`measured_cash_cost_usd`も同様です。`monthly_operating_costs`が未登録の場合も測定済み原価はNULLとなり、`operating_cost_recorded`で判別できます。`monthly_priced_api_cost_usd`は単価を適用できた呼出しだけの小計であり、単価網羅が不完全なときは確定原価として扱いません。

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
