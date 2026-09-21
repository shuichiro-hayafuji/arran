# Arran ドキュメント

Arran のドキュメントは、システムの責務ごとに分けています。全体像を確認したあと、変更対象の領域とハーネスのルールを参照してください。

## システム構成

| 領域 | 説明 |
| --- | --- |
| [mobile](mobile/overview.md) | Flutterクライアントの構成、画面・状態・API通信の境界 |
| [server](server/overview.md) | Go API、業務ルール、Agent、データ永続化の構成 |
| [infra](infra/overview.md) | AWS CDK、ECS、RDS、Secrets Managerの配備構成 |

```text
mobile/ Flutter
    │ HTTP JSON / multipart
    ▼
server/ Go REST API ─── Agent / OpenAI Responses API（任意）
    │ PostgreSQL wire protocol
    ▼
infra/ Docker / ECS Fargate / RDS PostgreSQL
```

## 正本と運用ルール

領域別のルール本文は各領域ディレクトリに置き、ハーネスIndexは変更対象から参照先を選ぶために使います。

| 領域 | 業務ルール | アーキテクチャ | 実装時の作法 | 補助資料 |
| --- | --- | --- | --- | --- |
| mobile | [business](mobile/business.md) | [architecture](mobile/architecture.md) | [conventions](mobile/conventions.md) | [design-system](mobile/design-system.md) |
| server | [business](server/business.md) | [architecture](server/architecture.md) | [conventions](server/conventions.md) | — |
| infra | [business](infra/business.md) | [architecture](infra/architecture.md) | [conventions](infra/conventions.md) | — |

- [ハーネスIndex](harness/INDEX.md): 変更対象に応じた業務ルール、アーキテクチャ、実装作法の入口
- [運用・検証範囲](harness/WORKFLOW.md): 文書・コード・検証の責務と、未検証範囲の扱い
- [改善記録](harness/feedback/INDEX.md): 実装上の指摘と再発防止の履歴
- [OpenAPI定義](../server/docs/openapi.yaml): HTTPリクエスト・レスポンスの正本
- [ER図](ER_DIAGRAM.md): データ構造の説明資料
- [PostgreSQL migration](../server/internal/infrastructure/persistence/postgres/002_auth.sql): 実行時スキーマの正本

## 製品・業務

- [対象者と支出相談の体験](product/target-users.md)
- [初版の提供範囲](product/initial-release-scope.md)

## 補助資料

- [Issue棚卸し・反映原稿](issue-audit-2026-09-13.md)
- [Agentリポジトリ分割計画](repository-split-plan.md)
- [Agent submoduleへの移行](agent-submodule-migration.md)
