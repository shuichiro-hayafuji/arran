# 認証・認可

## 方式の比較と採用理由

DBに管理者がユーザーを登録し、Flutterのネイティブアプリから単一のGo APIを使う前提です。

| 方式 | 利点 | 今回の判断 |
| --- | --- | --- |
| 外部IdP / OIDC + PKCE | MFA、SSO、復旧をサービスへ委譲できる | 将来の候補。今回は外部アカウント管理が不要で、DB直接登録という要件と合わない |
| JWT + refresh token | 複数APIで署名だけによる検証ができる | 即時失効とrefreshのローテーションに追加の状態管理が必要。現状では複雑さに見合わない |
| Cookie + DBセッション | WebではHttpOnlyによる保護が可能 | ネイティブ主体ではBearerの方が扱いやすい。Web対応時はCSRF対策と併せて再検討 |
| ランダムBearer + DBセッション | 即時失効、ユーザー無効化確認が簡単 | **採用**。DB照会1回/リクエストを許容し、構成を小さく保つ |

パスワードはGo標準のPBKDF2-HMAC-SHA256（600,000回、ランダム16バイトsalt、32バイト出力）で保存します。Argon2idも候補ですが、今回は標準ライブラリで実装できる適切なワークファクターのPBKDF2を選びました。ハッシュの形式は固定し、DBの不正なパラメータによる過剰計算を防ぎます。平文や高速なSHA-256だけのパスワード保存はしません。

## APIとセッション

- 公開: `GET /health`、`GET /healthz`、`POST /auth/login`、CORS preflight。
- `POST /auth/login`: `{"username":"alice","password":"..."}`。成功時は`access_token`、`token_type: Bearer`、`expires_at`、`user: {id, username}`。
- 保護API: `Authorization: Bearer <access_token>`が必要。将来追加したパスもデフォルトで保護されます。
- `GET /auth/me`: 現在のユーザーを確認。
- `POST /auth/logout`: 使用中のセッションをDBから削除し、204を返す。他端末のセッションは維持。
- 未ログイン、期限切れ、失効、無効アカウントは401。DB障害は503で、認証を迂回しません。
- トークンは暗号学的乱数32バイト（256 bit）。DBにはそのSHA-256のみ保存。7日の絶対期限で、自動延長・refresh tokenはありません。期限切れは再ログインします。
- セッション作成時のパスワードハッシュも照合するため、DBでパスワードを変更すると旧セッションは使用できません。
- ユーザー名とパスワードの不一致・未知ユーザー・無効ユーザーは同じ401応答。未知ユーザーにもダミーハッシュの照合を実行します。
- DB共有のログイン制限: ユーザー名ごとに5回/分、接続元IPごとに20回/分（成功を含む）。429と`Retry-After: 60`。同一プロセスのパスワード計算は同時4件まで。
- `X-Forwarded-For`は信用せず、TCP接続元を使います。プロキシ経由ではIP制限を共有するため、本番の規模が増えたら信頼プロキシの設定とエッジ側制限を設計してください。
- 認証応答・業務応答は`Cache-Control: no-store`。ログイン本文、パスワード、Authorizationヘッダーをログに出しません。

API自体はTLSを終端しません。資格情報を扱うため、実運用ではHTTPSのリバースプロキシを経由してください。既存のHTTP接続はローカル開発専用です。DBユーザーにはアプリ運用に必要な権限だけを付与してください（起動時マイグレーションにはDDL権限も必要です）。

## ユーザーを直接DBに登録する

1. APIを起動し、`001_init.sql`と`002_auth.sql`を適用します。ユーザーが0件でも起動でき、ヘルスチェックは200、業務APIは401になります。
2. `server/`で次を実行します。パスワードをコマンド引数やシェル履歴に含めず、表示されたハッシュだけをコピーします。

```sh
python3 -c 'import getpass; print(getpass.getpass("Password: "))' | go run ./cmd/passwordhash
```

パスワードは12〜1024 UTF-8バイト。ユーザー名は3〜128文字のASCII小文字・数字・`_.@+-`で、先頭は英数字です。ログイン時にはユーザー名のみ前後空白除去と小文字化をします。

3. 対象DBの`psql`で登録します。`DATABASE_URL`は通常の環境設定から指定してください。

```sql
\set ON_ERROR_STOP on
\prompt 'Username: ' username
\prompt 'Generated password hash: ' password_hash
INSERT INTO users(username, password_hash)
VALUES (lower(trim(:'username')), :'password_hash')
RETURNING id, username;
```

登録API、自己サインアップ、管理者向けHTTP APIは設けていません。パスワードの再設定も同じツールで新しいハッシュを作り、DBの`password_hash`を更新します。

アカウントを無効化し、再有効化後も旧セッションを使えなくする例（`psql`）:

```sql
\prompt 'Username to disable: ' username
BEGIN;
UPDATE users SET is_active = FALSE WHERE username = :'username';
DELETE FROM auth_sessions
WHERE user_id IN (SELECT id FROM users WHERE username = :'username');
COMMIT;
```

## データの所有権と既存データ

全業務テーブルに`user_id`を追加し、認証済みリクエストのcontextから所有者を決めます。bodyやqueryから所有者は受け付けません。全SELECT・UPDATE・DELETEは所有者条件を含み、INSERTも同じ所有者を設定します。他ユーザーのIDは404です。プロフィールはユーザー単位で一意、取引の重複排除と加盟店ルールの一意制約もユーザー単位です。相談メッセージは`(consultation_id, user_id)`の複合外部キーでも親の所有者を検証します。

CSVプレビューにも所有者を保存します。他ユーザーのcommitは拒否し、正しい所有者のプレビューを消費しません。プレビューはプロセス内で30分保持するため、API再起動で失われます。複数インスタンス運用にはプレビュー保存先の共有化が別途必要です。

既存レコードの`user_id`はNULLのまま保持し、APIからは見えません。最初に登録されたユーザーへ勝手に割り当てません。次は**既存データのすべてが指定した1ユーザーのものだと確認できた場合のみ**実行する移行例です。DBをバックアップし、対象ユーザーが新規プロフィール等を作る前に実行してください。既存の所有データと一意制約が衝突した場合は全体がロールバックされるので、データの統合方針を先に決めます。

```sql
\set ON_ERROR_STOP on
\prompt 'Owner username for all legacy data: ' username
SELECT id AS owner_id FROM users WHERE username = :'username' \gset
BEGIN;
LOCK TABLE profiles, transactions, merchant_rules, consultations,
  consultation_messages, memories, monthly_reviews IN EXCLUSIVE MODE;
UPDATE profiles SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
UPDATE transactions SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
UPDATE merchant_rules SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
UPDATE consultations SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
UPDATE consultation_messages SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
UPDATE memories SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
UPDATE monthly_reviews SET user_id = :'owner_id'::bigint WHERE user_id IS NULL;
COMMIT;
```

`schema_migrations`で適用済みバージョンを確認し、マイグレーション全体をトランザクションとadvisory lockで保護します。SQLite adapterは従来の単体テスト用で、認証付き本番サーバーの保存先はPostgreSQLです。

## mobile

ログイン画面、ルートガード、Bearer送信、ログアウトを実装しています。トークンと期限を`flutter_secure_storage`でiOS Keychain / Androidの暗号化ストレージに保存し、パスワードは保存しません。保存キーはAPI接続先ごとに分けます。Androidのバックアップは無効、iOSは端末移行不可・ロック解除中のみ読める設定です。

起動時に端末のセッションを復元し、最初の業務APIでサーバー側の有効性も確認します。401または絶対期限到達時にはログイン画面へ戻ります。通信障害でセッションは消さず再試行できます。ログアウトのサーバー失効に失敗した場合もエラーを表示して再試行します。

セッション切替時はRiverpodのProviderScopeを作り直し、前ユーザーのプロフィール、CSVプレビュー、相談等の画面キャッシュと通信クライアントを破棄します。遅れて届いた旧トークンの401は新しいセッションに影響しません。

## 検証

```sh
cd server
go test -race ./...
# テスト用DBのURLを設定すると、独立した一時スキーマで実DBの所有権・移行・失効を検証
ARRAN_TEST_DATABASE_URL='postgres://test_user:test_password@127.0.0.1:5432/test_db?sslmode=disable' \
  go test -race ./internal/infrastructure/persistence/postgres -run TestPostgresOwnershipAndSessions -v
cd ../mobile
flutter pub get
flutter analyze
flutter test
```

統合テストはURL未設定時に明示的にskipします。DBのschema作成権限が必要です。テストは既存publicテーブルに触れず、生成した一時スキーマを終了時に削除します。

2026-09-13の作業環境ではGoテスト・race検査とFlutter静的解析を実施。Dockerソケットに接続できないためPostgreSQL実DBテストは未実行、Flutterテストはlocalhostソケット作成制限により起動できませんでした。iOS/Android実機でのKeychain/暗号化ストレージ、ログイン・再起動復元・ログアウト・アカウント切替は別途動作確認が必要です。
