# PostgreSQL スキーマ

`withdrawal_requests` は本人の申し込み時刻と、運営者が記録した初回確認時刻（`reviewed_at`）を保持する。確認時刻は利用停止・データ削除・本人通知の完了を意味しない。runtimeは申し込みを記録・照会できるが、確認時刻の変更や行の削除はできない。バックアップの復元で記録は巻き戻り得る。

[ER図（認証・家計・相談データ）](../../docs/ER_DIAGRAM.md)でテーブル間の関係と全カラムを確認できます。

実行時の正本は`internal/infrastructure/persistence/postgres/`の番号付きSQLです。本番では、配備時の `migrate` コマンドが未適用バージョンをトランザクション内で適用します。通常のAPIは移行履歴を読み取り、バージョンが一致した場合にだけ起動します。ローカル・開発環境では起動時に自動で移行します。[DBの起動手順](../README.md)を参照してください。[認証・既存データ移行](authentication.md)も参照してください。ローカルではDocker ComposeのPostgreSQL 16に接続します。

```sh
docker compose up -d postgres
export DATABASE_URL='postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable'
psql "$DATABASE_URL" -c '\\dt'
```

## テーブル

| テーブル | 役割 | 主な制約 |
| --- | --- | --- |
| `users` | DB直接登録のアカウント | ユーザー名一意、password_hash、is_active |
| `ai_consent_policy` | 全サーバー共通の説明版と本文 | singleton。初期値NULLは未設定として同意・送信を拒否 |
| `ai_consent_policy_versions` | 版と説明本文の対応履歴 | 版を再利用して異なる本文を示すことを拒否 |
| `ai_consent_policy_lock` | 版変更と送信許可の排他制御 | singleton。runtimeは共有ロックを取得可能 |
| `ai_user_consents` | 利用者別の同意と撤回保留 | `user_id`主キー、版と撤回状態 |
| `ai_consent_inflight` | 開始が許可された未完了の外部送信 | ランダムなID主キー、所有者FK、明示終了まで保持 |
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
| `csv_import_previews` | 未確定の全明細・本人確認と30分期限 | `(user_id,id)`主キー、所有者FK、期限索引。空行成功時だけresultを保持 |
| `csv_identity_file_rows` | scope付きCSV物理行と保存明細の一対一対応 | owner/file/source/account/type/rowの主キー、同scope/明細の一意性、所有者付き明細FK |
| `csv_identity_aliases` | CSV確認済みstrategyキーと双方snapshot | `(user_id,row_key)`主キー、所有者付き明細FK、scope/行/明細を行対応へ結ぶ複合FK |
| `transactions` | CSVから取り込む支出・入金明細 | `(user_id, fingerprint)` 一意、日付・カテゴリ索引 |
| `merchant_rules` | 加盟店正規化名とカテゴリのローカル規則 | `(user_id, normalized_merchant)` 一意 |
| `consultations` | 支出相談、助言、verdict、支出後結果 | `id` は `BIGSERIAL` |
| `consultation_messages` | 相談に紐づく会話 | `consultation_id` 外部キー、親削除時に連鎖削除 |
| `chat_requests` | 所有者ごとの論理受付、予約、期限、取消し、結果snapshot | `(user_id,request_id)`主キー、処理中は所有者ごとに1件 |
| `chat_states` | 利用者ごとの会話記憶と複数購入・訂正履歴JSON | `user_id`主キー・外部キー、version |
| `chat_threads` | 原チャットの会話と購入投影JSON | `consultation_id`主キー、所有者付き複合FK |
| `chat_purchase_schedule` | 6時間後の確認対象検索 | `(user_id,purchase_id)`主キー、所有者付き相談FK、期限・送信済み索引 |
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

## 購入確認の保存

010 migrationで`chat_purchase_schedule`を追加する。利用者の`chat_states.payload.purchases`が履歴の正本であり、予定額、実額、分類、確認日時、残高・自由予算へ反映済みの実額、更新根拠、変更前のイベントを保持する。期限索引は保存時に同じtransactionで再構成する。009の旧予定は最初の所有者状態読取で取り込み、引継ぎ前の旧予定もworkerが検索する。相談を削除すると期限索引もCASCADEで削除する。所有者未割当の旧相談は取り込まない。

確認workerとモデル応答は利用者状態のversionとrow lockで直列化し、原チャットへの複数更新を原子的に保存する。SQLiteは004のテストスキーマを使い、本番保証は専用PostgreSQLで別に検証する。実績反映の金融計算はLLMが担い、SQL・Goでは金額を再計算しない。

011 migrationの`chat_requests`は受付metadataと保存結果を保持する。期限はUnix epochミリ秒のBIGINT、結果はThread JSON。受付と月次予約、結果と業務保存、失敗と予約解放をそれぞれ同じtxで確定する。[受付契約](consultation-requests.md)を参照。

012のCSVキー移行は[明細同一性・移行契約](csv-identity.md)を参照。transactionsにidentity_version/legacy_fingerprint/provider_transaction_id/import_file_keyを追加する。分類・row ID・原文を保持し、aliasと同時に原子的に移行する。

## 購入とCSVの対応（013 / SQLite007）

purchase_csv_decisionsはowner/transaction/purchaseを主キーとし、same/distinct、原CSV snapshot、最新購入revision、包含状態とpending/needs_information/appliedを保持する。sameはowner付きtransactionとpurchaseの各々にpartial unique indexを持つ。purchase_csv_receiptsはowner/ref/purchase_revisionごとの金融結果、balance_resolved/budget_resolved、state versionを保存する。銀行明細と対応への複合FKでownerを一致させる。購入本体は既存chat state JSONを維持する。

CSVキー/alias/file-row対応と照合の保存は同じtx、金融receiptと相談/state/イベントの保存は既存SaveChatTurnの同じtx。移行は既存SQLを変更せず新番号で適用し、DDL途中失敗と同時起動を両DB回帰で検証する。[照合契約](purchase-csv-reconciliation.md)を参照。

## 取込管理の014 / SQLite008

transactionsのeffective_date/amount/categoryとrevisionは訂正投影、legacy_retainedは旧不明証拠の保護。effective_transactionsは他のactive batch evidenceも含め有効性を判定する。csv_import_batches（owner/id、owner/commit_key一意）、csv_import_evidence（owner/batch/row/type、owner付きbank FK）、csv_mutations（owner/id・同payload結果）、transaction_change_events（owner付きmutation FK・immutable前後）、csv_financial_receipts（owner/ref・方向/coverage/axis/status）、csv_financial_items（owner/ref/transaction・両owner付きFK）を追加する。SQLiteの新remember merchant規則はmerchant_rules_ownedでowner分離し、owner0の旧規則は保持する。旧file-rowのscopeが証明できる単位だけを日時不明のlegacy履歴へ移し、不明単位は原行保護。詳細は[取込管理](csv-import-management.md)。

## プレビューの015 / SQLite009

`csv_import_previews`は`user_id`・`id`を主キーとし、全明細と内部確認情報をversion付きJSONの`payload`に保存する。`expires_at`はUnix epochナノ秒のBIGINTで、所有者と期限の索引を持つ。PostgreSQLはusers FK、SQLiteは既存のテスト用owner方式を維持する。`result`はNULLが未確定、0行成功時だけJSON receiptを保持し、その際payloadを空にする。通常の成功結果は既存csv_import_batchesのcommit_key/resultを再利用してpreview行を同じtxで削除する。期限切れ未確定行は同所有者の次回preview保存時に削除する。既存明細・履歴・receiptの変更や移し替えはない。

## 外部AI送信同意の016（実装途中）

同意の付与・送信許可・撤回の各トランザクションはDBの既定値にかかわらずREAD COMMITTEDを指定する。説明版の更新triggerもREAD COMMITTED以外のトランザクションからの変更を拒否し、固定snapshotによる撤回・送信記録の見落としを防ぐ。OpenAIへの単一同期要求でHTTP 200の `status` が `completed`、`failed`、`cancelled`、`incomplete` と明示された場合は、受信した応答上は終端と扱う。HTTP 202、`queued`、`in_progress`、status欠落、JSON不正は結果不明として記録を残す。これらの判定はプロバイダー内部での実行終了を独立に保証しない。

送信許可は専用singletonの `ai_consent_policy_lock` を共有ロックし、説明更新は同じ行を排他ロックする。runtimeにポリシー本体の更新権限は与えず、ロック行にはSELECTとUPDATEのみを許可する。撤回APIは20秒以内に完了しなければ503を返し、完了したとは扱わない。外部HTTP応答は2MiB以内で最後まで受信できた場合だけ完了を記録し、通信失敗・超過・5xxなどの結果不明は未完了として保持する。401・403・429の完全な応答は要求の拒否として扱うが、外部プロバイダー内部の処理終了を独立して保証するものではない。

`ai_consent_policy`は全サーバー共通の現行版と表示文で、初期値は両方NULL。`ai_consent_policy_versions`は版と本文の対応を保持し、同じ版の本文変更や異なる本文での再使用を拒否する。同意付与と送信許可はこの版と本人の同意の双方をDBで検証する。許可時は未完了送信IDを先にコミットし、コールバックが正常終了した場合のみ削除する。撤回は本人の同意を無効化し新規付与・送信を拒否する状態を保存してから、本人の未完了IDがなくなるまで完了を返さない。複数サーバー間でも同じDBで直列化する。

コールバックのエラー、プロセス終了、DB接続断、完了記録失敗など結果が不明な場合、IDは残り続け、撤回と版変更は未完了となる。自動で期限切れにしない。外部側での完了確認、通知、監査、権限を含む安全な解消手順は未実装で、運用に供する前に必要である。DBの版更新は未完了IDがあると拒否され、変更時は旧版の許可を消す。説明文・版と運用権限は未確定である。

OpenAIの各HTTP送信前にストアを確認する。本文・版の設定とサーバー設定が一致しなければ送信を拒否する。単一HTTP送信の正常応答後に記録を削除するが、外部処理の終了を示す契約・安全な復旧方法は未確認であり、一般公開前に確認する必要がある。複数送信をまとめて途中の結果不明を握りつぶす利用は安全ではない。

同一利用者の撤回が保留状態の間に記録された複数の撤回要求は、同じ保留を待つ。未完了送信がなくなり最初の要求が保留を解除した後、本人が明示的に再同意すれば、別の撤回要求の呼び出し元が完了を受け取る前でも新しい送信を開始できる。先に記録された要求が再同意を取り消すことはない。一方、後続の撤回が再び保留を設定した後に先の要求が状態を確認すると、先の要求も後続の撤回の保留解除まで待つ。結果不明の新しい送信があれば、先の要求も未完了のままとなる場合がある。保留解除後に新しく開始した撤回は、再同意後の許可も対象となる。

専用DBの切断試験は、送信許可後にGoの接続プールを閉じて完了記録を書けなくする方法である。実ネットワークの切断、DBプロセスの強制終了、送信プロセスのクラッシュや外部サービス側の処理終了までは確認していない。

退会は申込を保存し、毎日日本時間0時のバッチで会員を無効化し、全セッションを失効する。相談・取引を含む既存データは保持する。受付・実行時には未解決の相談と外部AI送信を確認し、安全に処理できない場合は受付を拒否または未完了として再試行する。古いバックアップからの復元時には退会申込・無効化が巻き戻るため、独立証跡で照合できるまでAPIを公開しない。

本人行ロックと書込みを同一トランザクションに収める退会前排出の試作部品があるが、既存 HTTP 操作やバックグラウンド処理には接続していない。別接続への書込みや外部副作用は防止できず、全操作に対する排出の保証ではない。全 API・worker、接続断・再試行・ロック順序と復元照合を検証するまで退会受付は強制停止する。
