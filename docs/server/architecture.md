# Serverのアーキテクチャ

`server/` はローカルとAWSの両方で使う唯一のGo REST API実装です。HTTP、業務ルール、Agent連携、永続化を分離し、金額と支出可否の最終判断をGo側で行います。

## 起動と依存性の組み立て

エントリポイントは `server/cmd/api/main.go` です。

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

通常は `server/` で `docker compose up -d postgres` を実行してから、ホスト上でGo APIを起動します。詳細な環境変数と起動手順は [server/README.md](../../server/README.md) を参照してください。

## レイヤーと責務

```text
Flutter → HTTP → handler → application → domain
                              ├── agentadapter → agent module
                              └── Repository interface

infrastructure/openai → agent.Model interface
infrastructure/persistence/postgres → Repository interface
```

- `internal/handler`: HTTPルーティング、JSON / multipartの入出力、HTTPエラーへの変換
- `internal/application`: 入力検証、CSVプレビュー、相談・レビュー・記憶のユースケースとRepository interface
- `internal/domain`: 型と純粋な支出ルール。`EvaluateSpending` が残額、予算超過、`verdict` を決定
- `agent`（独立Go module）: 相談のbounded loop、Model interface、Prompt、出力Evaluator、モック
- `internal/agentadapter`: domainの決定論的な評価結果をAgent DTOへ変換し、出力をdomainへ戻す
- `internal/infrastructure/persistence/postgres`: SQL、トランザクション、PostgreSQL migration、pgx driver
- `internal/infrastructure/persistence/sqlite`: Repositoryテスト用adapter。実行時の依存ではない
- `internal/infrastructure/openai`: Responses API、`store:false`、Structured Output schema
- `internal/csvimport`: 文字コード、列、日付、金額の正規化、カテゴリ分類、fingerprint生成

Agent moduleは `server` の `internal` やdomainをimportしません。生のCSV、店舗名、口座名、明細本文をAgentやOpenAIへ渡さない境界は `internal/agentadapter` とApplication側で維持します。

## 主なデータフロー

### CSVインポート

```text
POST /transactions/import/preview
  → 文字コード・列・日付・金額を正規化
  → 最大10件のプレビューを返却
  → preview_idで30分、プロセス内メモリに保持
POST /transactions/import/commit
  → fingerprintで重複排除
  → PostgreSQLへ確定保存
```

プレビュー段階ではDBへ書き込みません。CSVの元行は `raw_data_json` としてDBに保存されますが、LLMへ送る相談コンテキストからは除外します。

### 支出相談

Applicationがプロフィール、月次集計、直近相談、記憶、関連明細を取得し、Goがカテゴリ、残額、支出後残額、予算超過、`verdict`（`safe`、`caution`、`avoid`、`insufficient_data`）を計算します。LLMはGoが計算した事実の説明だけを生成し、出力はGoで検証します。不備・タイムアウト・APIエラー時は決定論的モックへフォールバックします。

### 月次レビュー

高額支出、カテゴリ傾向、サブスクリプションらしい定期支出、相談結果などから、ローカルで最大5件の見直し候補を作ります。OpenAIが利用できる場合は説明を補助しますが、失敗時はローカル候補を保存します。

## データとAPIの正本

- [OpenAPI定義](../../server/docs/openapi.yaml): HTTPリクエスト・レスポンス
- [DBスキーマ](../../server/docs/database-schema.md): テーブルと確認手順
- [ER図](../ER_DIAGRAM.md): 全体のデータ関係
- [PostgreSQL migration](../../server/internal/infrastructure/persistence/postgres/002_auth.sql): 実行時スキーマ

金額は整数の円、保存時刻はRFC 3339、月の判定は `Asia/Tokyo` を使います。

## 現在の制約

- DBへのユーザー直接登録が必要で、自己登録・MFA・パスワード復旧APIはありません。
- APIはTLSを終端しないため、実運用ではHTTPSプロキシが必要です。
- CSVプレビューはプロセス内メモリに30分保持され、API再起動で失われます。
- ローカル開発にはDocker上のPostgreSQLと、ホスト上で実行するGo APIが必要です。

## 関連資料

- [サーバー業務ルール](../harness/business/server.md)
- [サーバー実装ルール](../harness/implementation/server/architecture.md)
- [サーバー実装時の作法](../harness/implementation/server/conventions.md)
- [認証設計・運用](../../server/docs/authentication.md)
