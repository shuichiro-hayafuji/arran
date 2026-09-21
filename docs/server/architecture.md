# サーバーのアーキテクチャ

- A-S01: 業務処理への依存を内向きに保つ。現行の`handler → application → domain`でも、移行後の業務packageでも、HTTP境界へSQL・金融計算・プロバイダー呼出しを置かず、Application CoreからHTTP・DB・LLM固有型を参照しない。
- A-S02: 外部機能を利用する業務packageが、必要最小限のRepository・Agent・Clock等のPortを所有する。interfaceは差し替え・テストが必要な境界に限定し、全依存へ一律に追加しない。現行のapplicationは用途別RepositoryとAgentのinterfaceを所有し、必要な境界を`Config`から注入する。
- A-S03: 現行の`agentadapter`、移行後の`adviceagent`は、Application Coreと独立AgentのDTO変換、支出評価の接続を担当する。取引情報は許可項目を明示転記し、業務モデルを丸ごとJSON化して渡さない。配置を移す際はAgent moduleの独立性と送信項目の検証を維持する。
- A-S04: `server/agent` は独立Go module・Git submodule。親serverのinternalをimportしない。オーケストレーション・Model境界・検証・fallbackを担当し、金融ルールの正本を移さない。
- A-S05: 現行ではOpenAI固有処理を`infrastructure/openai`、SQLとmigrationを`infrastructure/persistence/postgres`に置く。移行後は`openai`と`postgres`をOutbound Adapterとし、PostgreSQL migrationは`postgres`が所有する。配置を移す際は既存import、埋込みSQL、テスト、DB資料の参照先を同時に更新する。SQLiteは既存テスト用で、本番の切替先にしない。
- A-S06: 現行の`app`は構築、`config`は設定解釈、`auth`はHTTP認証、`identity`は認証済み利用者IDの伝達、`csvimport`はローカル入力の正規化を担当する。移行後は認証ユースケースを`identity`、認証endpoint・middlewareを`httpapi`へ分け、`config`やHTTP型をApplication Coreへ持ち込まない。
- A-S07: API実装は `server/` の一つだけとし、HTTP契約は [OpenAPI](../../server/docs/openapi.yaml) に合わせる。
- A-S08: `server/`は一つのGo binary・実行プロセス・配備単位・PostgreSQLを持つModular Monolithとする。業務モジュールは別サービスを意味せず、公開する型・ユースケース・Portで境界を作る。
- A-S09: 目標の業務境界は`identity`、`profile`、`transaction`、`consultation`、`review`とする。新規・変更コードは技術レイヤーだけでなく業務上の所有者を明確にし、一括移行ではなく機能変更に合わせて段階的に移す。
- A-S10: HTTPをInbound Adapter、PostgreSQL・Agent・OpenAIをOutbound Adapter、`app`をComposition Rootとする。具体Adapterの選択と生成をApplication Coreへ置かない。
- A-S11: Goの外枠は`cmd`・`internal`・`docs`・`scripts`を必要に応じて使う。公開ライブラリがない状態で`pkg`を作らず、`src`やアーキテクチャ用語だけの多重階層を設けない。詳細は [ADR-0001](../adr/0001-server-modular-monolith-hexagonal.md) を参照する。
- A-S12: PostgreSQLは共有してもテーブルの論理的な所有者を業務モジュールごとに定める。他モジュールのデータは公開ユースケース、読取Port、または用途を明示したRead Modelを介して扱い、任意の直接更新を追加しない。
- A-S13: 利用者が直ちに結果を必要とする処理は同期呼出しを基本とする。イベント・非同期ジョブは、即時応答が不要で、再試行・負荷分離・確実な配送の要件がある処理へ限定し、EDAやEvent Sourcingを既定にしない。
- A-S14: 業務package間の依存は一方向に保つ。相互参照が必要な場合は利用側の読取Portと用途別の型を使い、外側のAdapterを`app`から注入する。循環importを避ける目的だけで業務モデルを`shared`へ集めない。

## 採用方針と移行の位置づけ

System ArchitectureにはModular Monolith、server内部のSoftware ArchitectureにはHexagonal Architectureを採用する。Clean Architectureの依存原則は維持し、Goのpackageは技術レイヤー中心の現行構成から、業務機能ごとの所有者が分かる構成へ段階的に移行する。比較、採用理由、見直し条件は[ADR-0001](../adr/0001-server-modular-monolith-hexagonal.md)を参照する。

現在は一つのGo API・実行プロセス・配備単位・PostgreSQLで動作し、配置は採用方針と一致している。一方、内部コードは`handler`、`application`、`domain`、`infrastructure`という技術レイヤー中心であり、業務モジュールへの移行は完了していない。次の責務表と自動検査は、移行前の現行実装を説明・検証する。

## 現行パッケージの責務と所有する境界

| パッケージ | 責務 | 主な公開型・interface | 生成場所 |
| --- | --- | --- | --- |
| `internal/domain` | 金額、相談、取引、レビュー等の業務データと決定論的評価 | `Profile`、`Transaction`、`Consultation`、`EvaluateSpending`等。外部技術のinterfaceは持たない | 値はapplicationとRepository実装が生成 |
| `internal/application` | HTTPに依存しないユースケース、入力検証、処理順序 | `Application`、`Config`、用途別Repository interface、3つのAgent interface | `internal/app`が`application.New`を呼ぶ |
| `internal/handler` | HTTPの入力・出力とapplication errorのHTTP変換 | `New(*application.Application) http.Handler` | `internal/app` |
| `internal/agentadapter` | domainと独立AgentのDTO変換、Agent呼出し前の支出評価 | `New`。具象adapterは非公開 | `internal/app`。applicationは生成しない |
| `internal/app` | production依存の生成と接続、HTTP Serverの所有 | `App`、`New` | `cmd/api` |
| `internal/infrastructure/notification` | 月間相談上限の管理者通知をPagerDuty Events APIへ送信 | `NewPagerDuty` | `internal/app` |
| `internal/infrastructure/openai` | Agentの`Model`を満たすOpenAI Responses API adapter | `NewOpenAIClient` | `internal/app` |
| `internal/infrastructure/persistence/postgres` | production DB、migration、application/auth境界の実装 | `Repository`、`Open` | `internal/app` |
| `internal/infrastructure/persistence/sqlite` | ローカルテスト用Repository実装 | `Repository`、`Open` | テストだけで生成 |
| `internal/auth` | セッション認証とHTTP middleware | `Store`、`Service`、`New` | `internal/app` |
| `internal/config` | 環境変数と設定値の解釈 | `Config`、`Load` | `cmd/api` |
| `internal/identity` | 認証済み利用者IDのcontext伝達 | context helper | authが設定し、applicationとRepositoryが参照 |
| `internal/csvimport` | CSVのdecode、正規化、fingerprint生成 | `Options`、`ParseResult`、`Parse` | applicationから関数として使用 |
| `server/agent` | LLM呼出し、出力検証、上限付き再試行、fallback | `Agent`、`Model`、入出力DTO、`New` | `agentadapter`が生成し、Model実装は`app`が選ぶ |

`Application`は複数ユースケースの入口をまとめるため大きいが、永続化境界はプロフィール、取引、相談、記憶、レビューに分割する。機能別Serviceへの分割は呼出し側とトランザクション境界を変えるため、このIssueでは行わない。新しいRepository操作は、用途が一致する小さいinterfaceへ追加する。

公開識別子は、別パッケージが型を指定する境界、DTO、生成関数、またはinterfaceの実装に必要なメソッドに限定する。今回の棚卸しでは、handlerの具象型、agentadapterの具象型、Agentのorchestratorと修正用interface、OpenAIの具象client、同一パッケージの検証helperを非公開にした。domainの業務型、applicationが所有するinterface、Agentと親server間のDTO・Model、各adapterの生成関数はパッケージ境界を越えるため公開を維持する。OpenAIの利用量は技術DTOの`UsageRecord`でcomposition rootへ通知し、そこで認証済み利用者とdomainの記録型へ結び付けるため、OpenAI実装からdomainとidentityを参照しない。

循環依存はGoコンパイラでも失敗するが、コンパイル前に意図しない逆向き依存を説明できるよう、チェッカーは`internal`パッケージごとの許可先を列挙する。新しいパッケージは依存方針を登録しない限り失敗させ、既存パッケージから未許可の`internal`参照も失敗させる。

## 目標とするpackage境界

```text
server/
├── cmd/{api,passwordhash}/       # 薄いエントリポイント
├── internal/
│   ├── app/                      # Composition Root
│   ├── identity/                 # 認証・セッションのユースケース
│   ├── profile/                  # 家計プロフィール
│   ├── transaction/              # 取引・CSV取込・月次集計
│   ├── consultation/             # 支出相談・支出判定
│   ├── review/                   # 月次レビュー・記憶
│   ├── httpapi/                  # endpoint・認証middlewareを含むInbound Adapter
│   ├── postgres/                 # Outbound Adapter・PostgreSQL migration
│   ├── adviceagent/              # Agent Adapter
│   ├── openai/                   # AgentのModel Adapter
│   ├── config/
│   └── shared/                   # 全機能で共有する最小限の型
├── agent/                        # 独立Go module・Git submodule
├── docs/
└── scripts/
```

各業務packageがモデル、ユースケース、必要最小限のOutbound Portを所有する。HTTP、PostgreSQL、Agent、OpenAIは外側のAdapterとし、具体実装は`app`で組み立てる。interfaceは利用側に置き、I/Oや時刻など差し替え・テストが必要な境界に限定する。

PostgreSQLは一つのまま利用するが、テーブルの論理的な所有者を業務モジュールごとに定める。他モジュールのデータは公開ユースケース、読取Port、または用途を明示したRead Modelを介して扱う。業務package間の依存は一方向に保ち、相互参照が必要な場合は利用側のPortと用途別の型を使う。循環importを避ける目的だけで、業務モデルを`shared`へ集めない。

目標構成への移行は、既存のHTTP契約、金融判定、認証ユーザーの所有者条件、Agentへの送信項目を保ちながら機能単位で行う。ディレクトリだけを先に作る一括移行は行わない。移行時は`tools/server_architecture.py`の許可packageと検査テストを同じ変更で更新する。

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
