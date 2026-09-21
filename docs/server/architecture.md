# サーバーのアーキテクチャ

- A-S01: `handler → application → domain` を基本依存とする。handlerへSQL・金融計算・プロバイダー呼出しを置かない。domainはHTTP・DB・LLMに依存しない。
- A-S02: applicationはユースケースとRepository等の境界を所有する。PostgreSQL、OpenAI、独立Agent、agentadapterの実装に直接依存しない。必要な境界は`Config`から注入し、既定実装をapplication内で構築しない。
- A-S03: `agentadapter` はDomainと独立AgentのDTO変換、支出評価の接続を担当する。取引情報は許可項目を明示転記し、Domainを丸ごとJSON化して渡さない。
- A-S04: `server/agent` は独立Go module・Git submodule。親serverのinternalをimportしない。オーケストレーション・Model境界・検証・fallbackを担当し、金融ルールの正本を移さない。
- A-S05: OpenAI固有処理は `infrastructure/openai`、SQLとmigrationは `infrastructure/persistence/postgres`。SQLiteは既存テスト用で、本番の切替先にしない。
- A-S06: `app` は構築、`config` は設定解釈、`auth` は認証、`identity` はユーザー識別の伝達、`csvimport` はローカル入力の正規化を担当する。
- A-S07: API実装は `server/` の一つだけとし、HTTP契約は [OpenAPI](../../server/docs/openapi.yaml) に合わせる。

## パッケージの責務と所有する境界

| パッケージ | 責務 | 主な公開型・interface | 生成場所 |
| --- | --- | --- | --- |
| `internal/domain` | 金額、相談、取引、レビュー等の業務データと決定論的評価 | `Profile`、`Transaction`、`Consultation`、`EvaluateSpending`等。外部技術のinterfaceは持たない | 値はapplicationとRepository実装が生成 |
| `internal/application` | HTTPに依存しないユースケース、入力検証、処理順序 | `Application`、`Config`、用途別Repository interface、3つのAgent interface | `internal/app`が`application.New`を呼ぶ |
| `internal/handler` | HTTPの入力・出力とapplication errorのHTTP変換 | `New(*application.Application) http.Handler` | `internal/app` |
| `internal/agentadapter` | domainと独立AgentのDTO変換、Agent呼出し前の支出評価 | `New`。具象adapterは非公開 | `internal/app`。applicationは生成しない |
| `internal/app` | production依存の生成と接続、HTTP Serverの所有 | `App`、`New` | `cmd/api` |
| `internal/infrastructure/openai` | Agentの`Model`を満たすOpenAI Responses API adapter | `NewOpenAIClient` | `internal/app` |
| `internal/infrastructure/persistence/postgres` | production DB、migration、application/auth境界の実装 | `Repository`、`Open` | `internal/app` |
| `internal/infrastructure/persistence/sqlite` | ローカルテスト用Repository実装 | `Repository`、`Open` | テストだけで生成 |
| `internal/auth` | セッション認証とHTTP middleware | `Store`、`Service`、`New` | `internal/app` |
| `internal/config` | 環境変数と設定値の解釈 | `Config`、`Load` | `cmd/api` |
| `internal/identity` | 認証済み利用者IDのcontext伝達 | context helper | authが設定し、applicationとRepositoryが参照 |
| `internal/csvimport` | CSVのdecode、正規化、fingerprint生成 | `Options`、`ParseResult`、`Parse` | applicationから関数として使用 |
| `server/agent` | LLM呼出し、出力検証、上限付き再試行、fallback | `Agent`、`Model`、入出力DTO、`New` | `agentadapter`が生成し、Model実装は`app`が選ぶ |

`Application`は複数ユースケースの入口をまとめるため大きいが、永続化境界はプロフィール、取引、相談、記憶、レビューに分割する。機能別Serviceへの分割は呼出し側とトランザクション境界を変えるため、このIssueでは行わない。新しいRepository操作は、用途が一致する小さいinterfaceへ追加する。

公開識別子は、別パッケージが型を指定する境界、DTO、生成関数、またはinterfaceの実装に必要なメソッドに限定する。今回の棚卸しでは、handlerの具象型、agentadapterの具象型、Agentのorchestratorと修正用interface、OpenAIの具象client、同一パッケージの検証helperを非公開にした。domainの業務型、applicationが所有するinterface、Agentと親server間のDTO・Model、各adapterの生成関数はパッケージ境界を越えるため公開を維持する。

循環依存はGoコンパイラでも失敗するが、コンパイル前に意図しない逆向き依存を説明できるよう、チェッカーは`internal`パッケージごとの許可先を列挙する。新しいパッケージは依存方針を登録しない限り失敗させ、既存パッケージから未許可の`internal`参照も失敗させる。

## 自動検査とレビューの境界

`tools/server_architecture.py`はproductionのGoファイルを対象に、次を検査する。`*_test.go`はテスト用の組立てで層を横断できるためimport規則の対象外とするが、通常の`go vet`と`go test`ではコンパイルする。

- `domain`からHTTP、DB、LLM、他の`internal`パッケージへの依存を禁止する。
- applicationからagentadapter、独立Agent、infrastructureへの依存を禁止する。
- 独立Agentから親serverの`internal`への依存を禁止する。
- OpenAIとPostgreSQLの実装を`internal/app`以外から参照することを禁止する。
- パッケージ名と配置の不一致、および依存方針を登録していない新しい`internal`パッケージを検出する。
- Agentへ渡す取引DTOを、日付・金額・カテゴリの3項目に固定する。

一方、金融判断と説明生成を名前やimportだけで完全には区別できない。`domain.EvaluateSpending`より前にAgentを呼んでいないこと、既存DTOの各項目へ別の意味の値を詰めていないこと、自由入力やプロフィールを含む送信範囲が目的に対して最小であることは差分レビューで確認する。チェッカーの成功だけで、金融判断がAgent側へ移っていない、または送信データが安全であるとは判定しない。

公開識別子の必要性も意味上の判断を伴うため自動検査しない。新しい公開型・関数を追加する場合は、利用する別パッケージと所有責務を差分レビューで確認する。

依存例外を追加する場合は、チェッカーの`DependencyException`をimport元・import先・ルールID・例外IDの完全一致で登録し、同じ例外ID、理由、許可範囲、解消条件を[運用・検証範囲](../harness/WORKFLOW.md)へ記録する。ディレクトリ単位の除外は登録しない。現在、登録済みの依存例外はない。
