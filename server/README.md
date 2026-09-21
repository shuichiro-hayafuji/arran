# Spendable Today API

「この支出をしても大丈夫か」を判断する家計管理アプリのバックエンドAPIです。

Go標準ライブラリのHTTPサーバー、PostgreSQL、CSVインポート、支出相談用のLLM連携で構成されています。Go APIはホストで実行し、PostgreSQLだけをDockerで起動します。

## 必要環境

- Go 1.26以降
- Docker Desktop

## 起動

リポジトリのルート（このディレクトリ）で実行します。

```sh
export API_ADDR=127.0.0.1:8080
docker compose up -d postgres
export DATABASE_URL='postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable'
export USE_MOCK_LLM=true

go run ./cmd/api
```

同じ起動処理はスクリプトからも実行できます。

```sh
./scripts/start-server.sh
```

スクリプトは実行場所をリポジトリ直下へ合わせ、未指定の`API_ADDR`と`DATABASE_URL`を補完します。`USE_MOCK_LLM`は設定ファイルまたはアプリケーション既定値に従います。スクリプトの`API_ADDR`既定値は`0.0.0.0:8080`で、LAN内の別端末から接続できる設定です。値を変更する場合は、起動前に環境変数を指定してください。

```sh
API_ADDR=0.0.0.0:8080 USE_MOCK_LLM=false ./scripts/start-server.sh
```

## LAN内の別端末から接続

サーバーを起動したMacとクライアント端末を同じWi-Fiまたは同じLANへ接続し、MacのLAN IPでアクセスします。

```sh
curl http://192.168.11.7:8080/healthz
```

`192.168.11.7`は現在確認できるこのMacのWi-Fi IPです。Wi-Fiを変更するとIPが変わる可能性があるため、起動時にスクリプトが表示する`LAN access`のURLを使ってください。ゲストWi-Fiの端末分離が有効な場合は、同じWi-Fi名でも接続できません。

別のネットワークや外出先から接続する場合は、単なる`0.0.0.0`設定では到達できません。VPN／Tailscaleなどのプライベート経路が必要です。このAPIはBearer認証を行いますがTLSは終端しません。実運用ではHTTPSプロキシを使い、APIポートを直接公開しないでください。

## TailscaleでVPN接続

別のWi-Fiや外出先から接続する場合は、サーバーMacとクライアント端末の両方を同じTailscaleネットワーク（tailnet）へ参加させます。

Homebrewの`tailscale` formulaはCLIと`tailscaled` daemonを提供しますが、macOSの`Tailscale.app`は提供しません。その構成では`open -a Tailscale`ではなく、Homebrew serviceとCLIを使います。

```sh
sudo brew services start tailscale
sudo tailscale up --accept-dns=true
```

1. サーバーMacで上記のservice起動と`tailscale up`を実行します。
2. クライアント端末にTailscale公式アプリをインストールし、同じtailnetへログインします。
3. Macで`tailscale ip -4`を実行してVPN IP（通常は`100.x.y.z`）を確認します。
4. このサーバーを起動します。

```sh
./scripts/start-server.sh
```

5. クライアント端末からVPN IPで接続します。

```sh
curl http://100.x.y.z:8080/healthz
```

スクリプトはTailscale CLIが利用可能なら、起動時に`VPN access`として接続先を表示します。より安全に、APIをローカルだけで待ち受けてTailscale経由で公開する場合は、次の構成にできます。

```sh
API_ADDR=127.0.0.1:8080 ./scripts/start-server.sh
```

別のターミナルで次を実行します。

```sh
tailscale serve --bg http://127.0.0.1:8080
tailscale serve status
```

`tailscale serve`を使うと、tailnet内のHTTPS URLからローカルAPIへ中継できます。不要になった場合は、TailscaleのServe設定を確認したうえで`tailscale serve reset`を実行してください。

起動後の確認:

```sh
curl http://127.0.0.1:8080/healthz
```

```json
{"status":"ok"}
```

Docker ComposeはPostgreSQLを`127.0.0.1:5432`だけに公開します。Go APIはDockerへ入れず、ホストから`DATABASE_URL`で接続します。

## 設定

[config.example](./config.example)に設定例があります。

起動時にカレントディレクトリの`.env`を読み込み、存在しなければ`../.env`を読み込みます。通常は`server/.env`へ設定してください。シェルで指定済みの環境変数は`.env`より優先されます。別のファイルを使う場合は`ENV_FILE`へパスを指定できます。

| 環境変数 | デフォルト | 説明 |
| --- | --- | --- |
| `API_ADDR` | `127.0.0.1:8080` | HTTPサーバーの待受アドレス |
| `DATABASE_URL` | `postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable` | PostgreSQL接続URL |
| `USE_MOCK_LLM` | `true` | `true`ならローカルモック、`false`ならOpenAIを使用 |
| `OPENAI_API_KEY` | 空 | `USE_MOCK_LLM=false`のときに必要 |
| `OPENAI_MODEL` | `gpt-5.6-terra` | OpenAI Responses APIで使うモデル |
| `OPENAI_REASONING_EFFORT` | `medium` | OpenAI Responses APIのreasoning effort |
| `APP_ENVIRONMENT` | `local` | PagerDuty通知に含める環境名 |
| `PAGERDUTY_ROUTING_KEY` | 空 | PagerDuty Events API v2のIntegration routing key |
| `ENV_FILE` | 空 | 明示的に読み込むdotenvファイルのパス |

`API_ADDR`のアプリケーション既定値は`127.0.0.1:8080`ですが、`scripts/start-server.sh`はLANアクセス用に`0.0.0.0:8080`を既定値として設定します。

`API_ADDR=0.0.0.0:8080`にするとLANからも接続できます。HTTPはローカル開発用とし、実運用ではログイン資格情報の保護のためHTTPSを経由してください。

## API一覧

ヘルスチェックとログイン以外は`Authorization: Bearer <access_token>`が必要です。[方式比較・DBユーザー登録・既存データ移行](docs/authentication.md)を参照してください。

| メソッド | パス | 用途 |
| --- | --- | --- |
| `POST` | `/auth/login` | DB登録アカウントでログイン |
| `GET` | `/auth/me` | 現在のユーザーを確認 |
| `POST` | `/auth/logout` | 使用中のセッションを失効 |

### ヘルスチェック・プロフィール

| メソッド | パス | 用途 |
| --- | --- | --- |
| `GET` | `/health`, `/healthz` | 稼働確認 |
| `GET` | `/profile` | 家計プロフィール取得 |
| `PUT` | `/profile` | 家計プロフィール更新 |

### 取引明細

| メソッド | パス | 用途 |
| --- | --- | --- |
| `POST` | `/transactions/import/preview` | CSVを解析してインポート候補を確認 |
| `POST` | `/transactions/import/commit` | プレビュー済みデータを登録 |
| `GET` | `/transactions` | 明細一覧取得（`month`、`limit`指定可） |
| `PATCH` | `/transactions/{id}` | 明細カテゴリ更新 |
| `GET` | `/dashboard/monthly` | 月次支出集計（`month=YYYY-MM`指定可） |

CSVインポートは「プレビュー → 確定」の2段階です。自動判定できない列がある場合、プレビューに列マッピングが返されます。確定前のプレビューはサーバーのメモリに30分間保持され、サーバー再起動後は利用できません。

### 支出相談

| メソッド | パス | 用途 |
| --- | --- | --- |
| `POST` | `/consultations` | 新しい支出相談を開始 |
| `POST` | `/consultations/{id}/messages` | 相談に追加情報を送信 |
| `PATCH` | `/consultations/{id}/result` | 実際の支出結果・満足度・後悔度を記録 |
| `GET` | `/consultations` | 相談履歴一覧 |
| `GET` | `/consultations/{id}` | 相談詳細 |

相談時はプロフィール、当月の集計、最近の相談、記憶データ、関連明細をコンテキストとして使います。OpenAIに送る取引情報は日付・金額・カテゴリに限定され、CSVの生データや店舗名は除外されます。OpenAI利用時のリクエストには`store: false`が指定されます。

相談回答にはbounded agentic loopがあります。Goが計算した残額・カテゴリ・予算超過・verdictを`decision_facts`としてLLMへ渡し、Structured OutputをGoで検証します。不備がある場合は前回回答と検証指摘を渡して最大2回修正し、それでも不正ならモックへフォールバックします。金額とverdictの権限はGo側に残し、LLMは説明文の生成に限定します。

### 月次レビュー・記憶

| メソッド | パス | 用途 |
| --- | --- | --- |
| `POST` | `/reviews/monthly` | 月次の見直し候補を生成 |
| `GET` | `/reviews/latest` | 最新レビュー取得 |
| `GET` | `/memories` | 記憶一覧取得 |
| `POST` | `/memories` | 記憶追加 |
| `PATCH` | `/memories/{id}` | 記憶更新 |
| `DELETE` | `/memories/{id}` | 記憶削除 |

LLMが利用できない場合も、支出カテゴリ、サブスクリプションらしい定期支出、高額支出、後悔度などからローカル候補を生成します。

## アーキテクチャ

依存は内側（`domain`）へだけ向かいます。Application は OpenAI Responses API や PostgreSQL の DTO を知りません。相談 Agent は Go が計算した `decision_facts` をモデルに説明させ、Evaluator がモデル出力を再検証します。

```mermaid
flowchart TD
  F[Flutter] --> H[handler: HTTP transport]
  H --> A[application: use cases]
  A --> D[domain: rules and types]
  A --> AA[internal/agentadapter: DTO mapping]
  AA --> AG[agent submodule: orchestration and Model interface]
  A --> R[application Repository interface]
  O[infrastructure/openai] --> AG
  S[infrastructure/persistence/postgres] --> R
```

`server/agent` は独立 Go module として準備され、Git submodule として管理します。`internal/agentadapter` が domain 値を最小 DTO に変換し、店舗名・口座名・CSV raw data を Agent へ渡しません。現行の相談フローは不要な Tool 呼び出しを増やさず、Application が必要な集計を取得して Agent に渡します。

## ディレクトリ構成

```text
server/
├── agent/                     # arran_agent Git submodule（独立Go module）
├── cmd/api/main.go             # 起動、設定、依存関係の組み立て
├── internal/
│   ├── app/app.go              # アプリケーション組み立てとHTTPサーバー
│   ├── config/config.go        # 環境変数からの設定読み込み
│   ├── application/            # HTTP非依存のユースケース・Repository境界
│   ├── domain/                 # 機能境界ごとのアプリ共通データモデル
│   │   ├── profile.go          # 家計プロフィール
│   │   ├── transaction.go      # 取引明細
│   │   ├── import.go           # CSVインポート
│   │   ├── dashboard.go        # 月次集計
│   │   ├── consultation.go     # 支出相談
│   │   ├── advice.go           # LLMアドバイスの下書き
│   │   ├── memory.go           # 記憶データ
│   │   ├── review.go           # 月次レビュー
│   │   └── categories.go       # カテゴリ定義
│   ├── handler/                # HTTPルーティング、JSON入出力、HTTPエラー
│   ├── agentadapter/           # domain と Agent submodule のDTO変換
│   ├── csvimport/              # 文字コード、列、日付、金額の正規化
│   └── infrastructure/
│       ├── openai/             # Responses API、store:false、Structured Output schema
│       ├── persistence/postgres/ # PostgreSQL migration、Repository実装
│       └── persistence/sqlite/   # SQLiteテストadapter
├── migrations/
│   ├── embed.go                # SQLマイグレーションの埋め込み
│   └── 001_init.sql            # 旧SQLiteテスト用スキーマ
├── docs/openapi.yaml           # HTTP APIのOpenAPI定義
├── docs/database-schema.md     # テーブル定義とDBドキュメント生成方法
├── sample_data/                # CSVインポート用サンプル
├── config.example              # 環境変数の設定例
├── Dockerfile                  # Go APIコンテナイメージ（AWS配備用）
├── compose.yaml                # ローカルPostgreSQLの起動定義
├── .dockerignore               # コンテナへコピーしないファイル
├── Makefile                    # 開発・テスト・コンテナ用コマンド
├── scripts/start-server.sh     # ローカル起動スクリプト
├── go.mod
└── go.sum
```

リクエスト処理は次の依存方向です。

```text
Flutter → HTTP → handler → application → domain
                              ├── internal/agentadapter → agent submodule
                              └── Repository interface

infrastructure/openai → agent.Model interface
infrastructure/persistence/postgres → Repository interface
```

`internal`配下はGoの`internal`パッケージ規則により、このモジュールの外部から直接importできません。HTTPの詳細は`handler`、ユースケースは`application`、Goの金額・verdictルールは`domain`、Agentへの変換は`agentadapter`に分離されています。Agent module は domain を import しません。`domain`はHTTP、PostgreSQL、OpenAIに依存しません。

`app`は依存関係の組み立て、`config`は環境変数の解釈、PostgreSQL adapterはDBライフサイクルを担当します。SQLite adapterは既存の高速なRepositoryテストを維持するためだけに残しています。CSV importerは文字コード・正規化を伴うローカル入力処理であり、外部I/O adapterではなく`csvimport`に保持しています。Agent のソースと移行手順は`server/agent`と[Agent submodule migration](../docs/agent-submodule-migration.md)に分離しています。

## データベース

起動時に[PostgreSQL migration](./internal/infrastructure/persistence/postgres/001_init.sql)から未適用の番号付きSQLが順に実行され、次のテーブルが作成されます。

詳細な列定義、制約、JSON列、PostgreSQL CLIによる確認方法は[docs/database-schema.md](./docs/database-schema.md)を参照してください。

- `profiles`
- `transactions`
- `merchant_rules`
- `consultations`
- `consultation_messages`
- `memories`
- `monthly_reviews`
- `schema_migrations`
- `service_settings`
- `user_consultation_limits`
- `monthly_consultation_usage`
- `admin_notifications`
- `llm_usage`
- `llm_model_prices`
- `monthly_operating_costs`

取引の`fingerprint`には一意制約があり、同じCSVを再インポートしても重複登録されません。

## テスト

```sh
go test ./...
```

テストは各パッケージのソースと同じディレクトリに配置されています。CSVの文字コード・金額解析、Repositoryの重複排除と月次集計、HTTPの主要フロー、LLMレスポンスの解析を検証します。

## 開発用コマンド

```sh
make start     # 起動スクリプト経由でAPIを起動
make run       # APIを起動
make test      # 全テスト
make fmt       # gofmt相当
make build     # ローカルバイナリをbin/へ生成
make db-schema # Atlasで実DBのスキーマを表示
make up        # Docker Composeで起動
make down      # Docker Composeを停止
```

OpenAPI定義は[docs/openapi.yaml](./docs/openapi.yaml)にあります。ブラウザやSwagger系ツールで読み込んでAPI仕様を確認できます。

## Docker

```sh
docker compose up -d postgres
```

PostgreSQLデータはComposeの`spendable-today-postgres` volumeに保存されます。ComposeはDBだけを起動し、APIはホスト上で`go run ./cmd/api`として実行します。

既存の`data/*.db`は削除も上書きもしませんが、PostgreSQLへ自動コピーもしません。既存のSQLite開発データを引き継ぐ必要がある場合は、内容を確認したうえで別途一回限りの移行を実施してください。

## 実装上の補足

- `USE_MOCK_LLM=true`が開発時の既定値です。
- OpenAI呼び出しが失敗した場合、支出アドバイスと月次レビューはローカル実装へフォールバックします。
- CSVのカテゴリは店舗ルール、キーワード、未分類の順で決まります。
- プロフィールや相談結果などの永続データはPostgreSQLに保存されます。
- DBセッション認証・ユーザー分離・バージョン管理されたマイグレーションを実装しています。[方式比較・ユーザー登録・既存データ移行](docs/authentication.md)を参照してください。既存データは所有者を明示的に割り当てるまでAPIから見えません。
- `.env.example`ではなく、機密値を含めない`config.example`を設定例として管理します。
