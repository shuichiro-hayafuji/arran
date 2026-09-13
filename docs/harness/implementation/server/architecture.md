# サーバーのアーキテクチャ

- A-S01: `handler → application → domain` を基本依存とする。handlerへSQL・金融計算・プロバイダー呼出しを置かない。domainはHTTP・DB・LLMに依存しない。
- A-S02: applicationはユースケースとRepository等の境界を所有する。PostgreSQLやOpenAI固有の実装に直接依存しない。Agent構築の既存依存は [EX-S01](../../WORKFLOW.md) として扱う。
- A-S03: `agentadapter` はDomainと独立AgentのDTO変換、支出評価の接続を担当する。取引情報は許可項目を明示転記し、Domainを丸ごとJSON化して渡さない。
- A-S04: `server/agent` は独立Go module・Git submodule。親serverのinternalをimportしない。オーケストレーション・Model境界・検証・fallbackを担当し、金融ルールの正本を移さない。
- A-S05: OpenAI固有処理は `infrastructure/openai`、SQLとmigrationは `infrastructure/persistence/postgres`。SQLiteは既存テスト用で、本番の切替先にしない。
- A-S06: `app` は構築、`config` は設定解釈、`auth` は認証、`identity` はユーザー識別の伝達、`csvimport` はローカル入力の正規化を担当する。
- A-S07: API実装は `server/` の一つだけとし、HTTP契約は [OpenAPI](../../../../server/docs/openapi.yaml) に合わせる。
