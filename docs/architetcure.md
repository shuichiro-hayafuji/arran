# Spendable Today 現在のアーキテクチャ

最終確認日: 2026-08-09

## 1. 位置づけ

Spendable Today は、支出前に予算・価値観・過去の判断を踏まえた助言を受けるMVPです。登録ユーザーごとに家計データを分離します。現在の実行系は、Flutterモバイルアプリ、ホスト上のGo REST API、Docker上のPostgreSQLで構成されます。

`server/` はローカルとAWSの両方で使う唯一のAPI実装です。`infra/` はこの同じ`server/`コンテナをECSへ配備します。`backend/`は廃止しました。

## 2. 全体構成

### 現行のローカル構成

```text
┌──────────────────────────────┐
│ mobile/                      │
│ Flutter iOS / Android        │
│                              │
│ Screen                       │
│   └─ ViewModel / Riverpod    │
│       └─ Feature Repository  │
│           └─ Dio ApiClient   │
└──────────────┬───────────────┘
               │ HTTP JSON / multipart
               │ API_BASE_URL
               ▼
┌──────────────────────────────┐
│ server/                      │
│ Go net/http REST API         │
│                              │
│ Handler → Application        │
│             ├─ Domain rules  │
│             ├─ Agent adapter → Agent submodule → Model │──── OpenAI Responses API (optional)
│             └─ Repository interface │
└──────────────┬───────────────┘
               │ PostgreSQL wire protocol
               ▼
┌──────────────────────────────┐
│ Docker Compose               │
│ PostgreSQL 16                │
│ 127.0.0.1:5432 のみ公開      │
└──────────────────────────────┘
```

通常は `docker compose up -d postgres` でDBを起動してから、Go APIをMac上で起動します。iOS Simulatorは `127.0.0.1:8080` に接続します。Android Emulatorは `10.0.2.2:8080`、物理端末はMacのLAN IPを `API_BASE_URL` に指定します。

## 3. モバイルアプリ

### 技術構成

- Flutter/Dart。Flutterのバージョンは `mobile/.fvmrc` で固定し、コマンドはFVM経由で実行する。
- `flutter_riverpod` / Riverpod: 状態管理と依存性注入。
- `go_router`: 画面遷移。
- `dio`: Go APIとのHTTP通信。
- `freezed`: 状態・Intentなどの不変データ型。
- `mobile/packages/file_picker`: CSV選択用の同梱ローカルプラグイン。

### レイヤーと責務

```text
features/<feature>/
├── screens/       UI、ユーザー操作の受け付け
├── providers/     ViewModel、状態、Repositoryの組み立て
├── repository/    API呼び出しとDTO変換
├── repository/dto JSONとの変換
└── domain/        画面が扱うドメインモデル
```

全機能で共有する通信経路は `mobile/lib/core/network/ApiClient` です。APIのURLはビルド時の `--dart-define=API_BASE_URL=...` から読み込み、未指定時は `http://127.0.0.1:8080` を使います。Flutter側にOpenAI APIキーは保持しません。

主な機能境界は次のとおりです。

- `profile`: 月収、残高、固定費、自由予算、貯蓄目標、価値観、助言の厳しさ。
- `transactions`: CSVプレビュー・確定、明細一覧、カテゴリ変更。
- `dashboard`: 月次集計。
- `consultation`: 支出相談、追加メッセージ、支出後の結果記録。
- `review`: 月次の見直し候補。
- `memories`: 相談結果から得た判断傾向の保存・編集・削除。

## 4. Go API

### 起動と依存性の組み立て

エントリポイントは `server/cmd/api/main.go` です。設定を読み込み、`server/internal/app` が次の依存関係を組み立てます。

```text
cmd/api/main.go
  → config.Config
  → app.New
      → postgres.Open(PostgreSQL + migration)
      → agent.MockClient または infrastructure/openai
      → application.New + agentadapter
      → handler.New
      → net/http.Server
```

HTTP層は標準ライブラリの `net/http` と `ServeMux` を使います。ヘルスチェック、CORS、ログ、panic recovery、タイムアウトがHTTP境界にあります。

### レイヤー構成

- `internal/handler`: HTTPルーティング、JSON/multipartの入出力、HTTPエラーへの変換。
- `internal/application`: 入力検証、CSVプレビュー管理、相談・レビュー・記憶のユースケースとRepository interface。
- `internal/domain`: 型と純粋な支出ルール。`EvaluateSpending`が残額・予算超過・verdictを決定する。
- `agent`（Git submodule）: 相談のbounded loop、Model interface、Prompt、出力Evaluator、モック。`internal`をimportしない。
- `internal/agentadapter`: domainの決定論的な評価結果をAgent DTOへ変換し、出力をdomainへ戻す。生のCSV・店舗名・口座名はここで遮断する。
- `internal/infrastructure/persistence/postgres`: SQL、トランザクション、PostgreSQL migration、pgx driver。
- `internal/infrastructure/persistence/sqlite`: 既存の高速なRepositoryテスト用adapter。実行時の依存ではない。
- `internal/infrastructure/openai`: Responses API、`store:false`、Structured Output schema。
- `internal/csvimport`: UTF-8/BOM、Shift_JIS/CP932、列判定、日付・金額正規化、カテゴリ分類、fingerprint生成。

## 5. データフロー

### CSVインポート

```text
FlutterでCSV選択
  → POST /transactions/import/preview
  → 文字コード・列・日付・金額を正規化
  → 最大10件のプレビューを返却
  → サーバーのメモリにpreview_idで30分保持
  → POST /transactions/import/commit
  → fingerprintで重複排除しPostgreSQLへ確定保存
```

プレビュー段階ではDBへ書き込みません。確定時に、日付・金額・摘要・取込元などから生成したfingerprintを一意キーとして登録します。CSVの元行は `raw_data_json` としてDBに保存されますが、LLMへ送る相談コンテキストからは除外されます。

### 支出相談

```text
Flutter → POST /consultations
  → Applicationがプロフィール、月次集計、直近相談、記憶、直近明細を取得
  → Goがカテゴリ、残額、支出後残額、予算超過、verdictを計算
  → MockまたはOpenAIが説明文をStructured Outputで生成
  → Goが必須項目とverdict/カテゴリの一致を検証
  → OpenAI利用時は最大3回まで生成・修正
  → 不備・タイムアウト・APIエラー時は決定論的モックへフォールバック
  → ConsultationとメッセージをPostgreSQLへ保存
```

金額と最終 `verdict`（`safe`、`caution`、`avoid`、`insufficient_data`）の権限はGo側にあります。LLMは、Goが計算した事実を説明する役割です。金額が不足している場合は追加質問を返します。

支出結果の記録時には、満足度・後悔度・理由を保存し、LLMまたはモックが再利用可能なメモリ候補を抽出します。信頼度0.7未満、根拠または内容が空の候補は保存しません。

### 月次レビュー

Applicationがローカルで高額支出、カテゴリ傾向、サブスクリプションらしい定期支出、相談結果などから見直し候補を作ります。OpenAIが有効な場合はAgentが候補の説明を補助し、失敗時はローカル候補をそのまま保存します。候補数は最大5件です。

## 6. LLMとデータ保護

- 初期設定は決定論的なローカルモック（`USE_MOCK_LLM=true`）です。
- OpenAI利用時はGo APIからResponses APIを呼び、`store: false` を指定します。
- 送信対象はプロフィール、集計値、予定額、カテゴリ、日付・金額・カテゴリに限定した関連明細、相談や結果の要約です。
- CSVのraw data、加盟店名、口座名、明細本文はOpenAIへ送信しません。加盟店名によるカテゴリ分類もローカルルールで行い、OpenAIには依頼しません。
- OpenAIのStructured Outputが不正、検証不一致、タイムアウト、APIエラーになった場合も、モック回答でユーザー体験を継続します。

## 7. PostgreSQLデータモデル

[ER図](ER_DIAGRAM.md)に、認証追加後の全テーブル・カラム・外部キー・一意制約を示します。

`server/internal/infrastructure/persistence/postgres/001_init.sql`と`002_auth.sql`が実行時のスキーマを定義します。主なテーブルは次のとおりです。

| テーブル | 役割 |
| --- | --- |
| `users`, `auth_sessions`, `auth_attempts` | DB登録アカウント、失効可能なセッション、ログイン回数制限 |
| `profiles` | ユーザーごとの家計プロフィール（`user_id`一意） |
| `transactions` | CSVから取り込んだ支出・入金明細とfingerprint |
| `merchant_rules` | 加盟店名とカテゴリのローカルルール |
| `consultations` | 相談、助言、verdict、支出後の結果 |
| `consultation_messages` | 相談のユーザー・アシスタントメッセージ |
| `memories` | 再利用する判断傾向 |
| `monthly_reviews` | 月次レビューと候補JSON |

金額は浮動小数点ではなく整数の円で扱います。時刻は保存時にRFC 3339、月の判定は日本時間（Asia/Tokyo）を使います。詳細は [server/docs/database-schema.md](../server/docs/database-schema.md) を参照してください。

## 8. API境界

現行APIはDBセッションによるBearer認証とユーザー所有者による認可を行います。[認証設計・運用](../server/docs/authentication.md)を参照してください。

```text
GET/PUT  /profile
POST     /transactions/import/preview
POST     /transactions/import/commit
GET      /transactions
PATCH    /transactions/{id}
GET      /dashboard/monthly
POST     /consultations
POST     /consultations/{id}/messages
PATCH    /consultations/{id}/result
GET      /consultations, /consultations/{id}
POST     /reviews/monthly
GET      /reviews/latest
GET/POST/PATCH/DELETE /memories...
GET      /health, /healthz
```

APIのリクエスト・レスポンス定義は [server/docs/openapi.yaml](../server/docs/openapi.yaml) を正本とします。

## 9. 実行・配備形態

### ローカルMVP（現行の主経路）

1. `server/` でGo APIを起動する。
2. `docker compose up -d postgres` でPostgreSQLを起動し、APIは`DATABASE_URL`で接続して起動時にマイグレーションを適用する。
3. `mobile/` をFVM経由で起動し、`API_BASE_URL`でAPIへ接続する。

`server/compose.yaml` はPostgreSQLだけをDockerで起動します。Go APIはホスト上で起動し、`127.0.0.1:5432`へ接続します。

### AWS CDK構成

`infra/` は、次のAWSリソースを作るCDK構成です。

```text
Internet
  → internet-facing ALB
  → ECS Fargate（private subnet、:8080）
       ├─ RDS PostgreSQL 16
       └─ Secrets Manager（OpenAIキー）
```

CDKは`server/`のDockerfileをビルドし、同じPostgreSQL adapterをRDSへ接続します。RDSの接続情報は環境変数とSecrets Managerから渡します。

## 10. 現在の制約

- DBに管理者がユーザーを直接登録します。自己登録・MFA・パスワード復旧APIはありません。
- APIはTLSを終端しません。実運用ではHTTPSプロキシが必要です。ログインAPIにはDB共有の回数制限があります。
- CSVプレビューはプロセス内メモリに30分保持され、API再起動で失われます。
- ローカル開発にはDocker上のPostgreSQLと、その起動後にホスト上で実行するGo APIが必要です。
- AWSは未デプロイです。デプロイ前にCDK synth、RDS接続、およびコンテナヘルスチェックを確認する必要があります。
- Androidも考慮していますが、主な実行確認対象はiOS Simulatorです。

## 11. 関連ドキュメント

- [README.md](../README.md): MVPの目的、起動方法、設計判断
- [server/README.md](../server/README.md): Go APIの詳細、環境変数、エンドポイント
- [server/docs/openapi.yaml](../server/docs/openapi.yaml): API契約
- [server/docs/database-schema.md](../server/docs/database-schema.md): PostgreSQLスキーマ
- [infra/README.md](../infra/README.md): AWS CDK配備案
- [infrastructure/README.md](../infrastructure/README.md): ローカル開発用設定
