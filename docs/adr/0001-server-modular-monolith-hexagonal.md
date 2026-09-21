# ADR-0001: Go serverをModular MonolithとHexagonal Architectureで構成する

- 状態: 採用
- 決定日: 2026-09-21
- 対象: `server/`

## 背景

ArranのGo serverは、認証、プロフィール、取引・CSV取込、支出相談、月次レビュー・記憶を一つのAPIとして提供しています。これらの機能は同じ利用者のデータを参照し、相談時にはプロフィール、月次集計、過去の相談、記憶、関連明細を組み合わせます。現時点では、機能ごとに独立した配備、DB、運用体制を持たせる要件はありません。

現在のコードは`handler`、`application`、`domain`、`infrastructure`という技術レイヤーを中心に分かれています。依存方向と用途別のPortは整理されていますが、複数の業務機能が一つの`Application`、`handler`、Repository実装へ集まり、機能単位の変更範囲とデータ所有者が見えにくくなっています。

また、PostgreSQL、HTTP、CSV、独立Agent、OpenAIという複数の外部境界があります。特に、金額、予算超過、支出可否の最終判断はGoに残し、LLMは検証済みの事実を説明する役割に限定する必要があります。

## 決定

### システム構成

`server/`は、一つのGo API、一つの実行プロセス、一つの配備単位を持つModular Monolithとします。PostgreSQLも一つのインスタンスを共有します。

当面の業務モジュールは、次の単位とします。

- `identity`: 認証、セッション、認証済み利用者の識別
- `profile`: 家計プロフィール
- `transaction`: 取引、CSV取込、月次集計
- `consultation`: 支出相談、決定論的な支出判定、相談結果
- `review`: 月次レビュー、記憶

モジュールは別プロセスや別サービスを意味しません。同じGo binaryの中で、公開する型・ユースケース・Portを境界として責務を分けます。

### Software Architecture

server内部にはHexagonal Architectureを採用します。Clean Architectureの「業務処理を外部技術へ依存させない」という原則は、依存方向の判断として継続します。

- Application Coreは業務機能ごとのpackageで構成する
- HTTPはInbound Adapterとする
- PostgreSQL、Agent、OpenAIはOutbound Adapterとする
- 外部機能を必要とする業務packageが、利用する最小限のinterfaceをPortとして所有する
- `internal/app`をComposition Rootとし、具体的なAdapterを選択して組み立てる
- `cmd/api`は設定読込、起動、終了制御に限定する

依存方向は次を基本とします。

```text
cmd/api → app
           ├→ business packages
           ├→ httpapi ─────────────→ business packages
           ├→ postgres / adviceagent ─→ business packages
           └→ openai ─────────────────→ agent.Model

business packages -X→ httpapi / postgres / adviceagent / openai / config
```

Application Coreから、HTTP、SQL、`pgx`、OpenAIのリクエスト・レスポンス型を参照しません。Agentとの変換では送信項目を明示し、業務モデルをそのまま外部DTOとして使用しません。

### Goのpackage構成

[`golang-standards/project-layout`](https://github.com/golang-standards/project-layout)はGo公式標準ではなく、一般的なレイアウトパターンの集約として参照します。掲載されたディレクトリを一律に導入せず、Arranでは主に次を維持します。

- `cmd/`: 実行可能アプリケーションごとの薄いエントリポイント
- `internal/`: 外部リポジトリへ公開しないアプリケーションコード
- `docs/`: OpenAPI、DB資料、設計・運用資料
- `scripts/`: 開発・検証用スクリプト

外部利用を保証するGoライブラリがないため、`pkg/`は設けません。`src/`、機械的な`domain/application/infrastructure`の多重階層、全依存に対するinterfaceは導入しません。interfaceは利用側に置き、I/O、時刻、外部モデルなど、差し替えまたはテストが必要な境界に限定します。PostgreSQL固有のmigrationは`postgres` Adapterが所有し、Go binaryへ埋め込める配置にします。

目標とするpackage構成は次のとおりです。

```text
server/
├── cmd/{api,passwordhash}/
├── internal/
│   ├── app/             # Composition Root
│   ├── identity/        # Application Core
│   ├── profile/         # Application Core
│   ├── transaction/     # Application Core
│   ├── consultation/    # Application Core
│   ├── review/          # Application Core
│   ├── httpapi/         # Inbound Adapter
│   ├── postgres/        # Outbound Adapter・PostgreSQL migration
│   ├── adviceagent/     # Outbound Adapter
│   ├── openai/          # AgentのModel Adapter
│   ├── config/
│   └── shared/          # 全機能で共有する最小限の型だけ
├── agent/               # 独立Go module・Git submodule
├── docs/
└── scripts/
```

この構成は目標であり、ディレクトリだけを先に作りません。既存機能を変更・分割するときに、振る舞いとHTTP契約を維持しながら段階的に移行します。

### データ所有とモジュール間連携

PostgreSQLは共有しますが、テーブルの論理的な所有者を業務モジュールごとに定めます。あるモジュールの業務処理から、別モジュールが所有するテーブルを任意に更新しません。横断参照が必要な場合は、公開ユースケース、利用側が定義する読取Port、または用途を明示したRead Modelを使います。

業務package間の依存は一方向に保ちます。相互参照が必要な場合は、利用側に読取Portと用途別の型を定義し、外側のAdapterを`app`から注入します。循環importを避ける目的だけで、業務モデルを`shared`へ集めません。

利用者が直ちに結果を必要とするログイン、プロフィール更新、CSVプレビュー・確定、支出相談は同期処理を基本とします。イベントや非同期ジョブは、即時応答が不要で、再試行、負荷分離、確実な配送が必要になった処理へ限定します。EDAやEvent Sourcingを既定の連携方式にはしません。

### 独立Agentの扱い

`server/agent`は独立Go module・Git submoduleですが、現在は同じGo APIプロセスへライブラリとして組み込みます。別サービスやMicroserviceとして扱いません。親serverとの変換は`adviceagent` Adapterへ隔離し、Agentから親serverの`internal`をimportしません。

## 比較した選択肢

### 技術レイヤー中心のMonolithを維持する

現在の依存方向を保ちやすく、移行コストもありません。一方、業務機能が増えるほど`application`、`handler`、Repository実装が肥大化し、変更範囲と所有者が分かりにくくなるため、長期の目標にはしません。

### Clean Architectureの同心円構成を厳密に導入する

業務処理を外部技術から守る原則は適合します。一方、Entity、Use Case、Gateway、Presenterなどを機械的に分割すると、現在の規模では中継だけの型やpackageが増えます。依存原則は採用し、具体的な境界表現にはPorts and Adaptersを使います。

### Microservicesへ分割する

機能ごとの独立配備、障害分離、個別スケールが可能になります。しかし現時点では、複数チーム、異なるリリース周期、独立スケール、サービスごとの可用性要件がありません。サービス間認証、分散トランザクション、監視、API互換性、ローカル開発の負担が先に増えるため採用しません。

### EDAをコンポーネント間連携の既定とする

非同期処理や再試行には有効ですが、主要な操作はその場で結果を返す必要があります。状態遷移、重複配送、結果取得、監視の複雑性に見合う要件がないため、同期呼び出しを既定とします。これは将来、必要な処理だけへイベントを導入することを妨げません。

## 結果

### 利点

- 一つの配備単位とDBトランザクションを維持できる
- 業務機能ごとに変更範囲とデータ所有を説明できる
- PostgreSQLやOpenAIをApplication Coreから分離できる
- 金融判断をGoに残し、Agentを交換可能なAdapterとして扱える
- 将来サービス分割が必要になった場合の境界を先に育てられる

### コストと注意点

- 同じプロセスを共有するため、障害とスケールは完全には分離されない
- package境界だけでは不正なimportを防ぎきれないため、レビューと静的検査が必要になる
- 共有DBでは、SQLを通じたモジュール間結合が生まれやすい
- 既存の技術レイヤー構成から段階的に移行する間、現行構成と目標構成が併存する
- interfaceやDTOを増やすこと自体を目的にしない判断が必要になる

## 移行方針

1. 既存の`application.go`と`handler.go`を、振る舞いを変えず機能別ファイルへ分ける
2. 変更頻度と依存の多い相談機能から、業務packageとPortを抽出する
3. HTTP、PostgreSQL、Agentの実装をAdapterとして分離する
4. import方向と外部送信項目のチェッカーを、新しいpackage境界と同じ変更で更新する
5. 移行済みの範囲から旧`domain`、`application`、`handler`の集約を縮小する

一括移行は行いません。各段階でOpenAPI互換性、決定論的な金融判定、認証ユーザーの所有者条件、Agentへの送信項目、既存テストを維持します。

## 見直し条件

次のいずれかが実測された場合は、サービス分割または非同期処理を別のADRで検討します。

- 複数チームが独立したリリース周期を必要とする
- 特定モジュールだけを独立してスケールする必要がある
- 障害、セキュリティ、可用性をプロセス単位で隔離する必要がある
- 同期処理では応答時間や再試行要件を満たせない
- 単一DBまたは単一デプロイが測定可能なボトルネックになる

## 関連資料

- [Serverの全体像](../server/overview.md)
- [サーバーのアーキテクチャルール](../server/architecture.md)
- [運用・検証範囲](../harness/WORKFLOW.md)
- [Go server README](../../server/README.md)
