# ハーネスIndex

このファイルは参照先の選択専用。ルール本文とコマンドはリンク先に置く。

| 変更対象 | 業務ルール | 実装ルール：アーキテクチャ | 実装ルール：実装時の作法 |
| --- | --- | --- | --- |
| mobile/ | [モバイル業務](../mobile/business.md) | [モバイル構成](../mobile/architecture.md) | [モバイル作法](../mobile/conventions.md) |
| server/（Agentを含む） | [サーバー業務](../server/business.md) | [サーバー構成](../server/architecture.md) | [サーバー作法](../server/conventions.md) |
| infra/ | [インフラ業務](../infra/business.md) | [インフラ構成](../infra/architecture.md) | [インフラ作法](../infra/conventions.md) |

| 横断する変更 | 追加で読む文書 |
| --- | --- |
| 対象者・相談体験・製品方針 | [対象者と支出相談の体験](../product/target-users.md)（初期方針と未検証事項） |
| 初版の機能範囲・公開先 | [初版の提供範囲](../product/initial-release-scope.md)（7機能の採否と後続の検討） |
| mobileの画面・Widget・テーマ | [モバイルのデザインシステム](../mobile/design-system.md) |
| API、DTO、認証、セッション | モバイル・サーバーの3文書と [OpenAPI](../../server/docs/openapi.yaml) |
| 金額、分類、相談、CSV、レビュー | サーバー業務。表示も変える場合はモバイル業務・構成 |
| 設定、DB、コンテナ、配備 | サーバー・インフラの3文書 |
| フォーマット、解析、テスト、生成、検証コマンド | [tool Index](../../tools/INDEX.md) |
| ハーネス自体、例外、正本間の不一致 | [運用・検証範囲](WORKFLOW.md) と tool Index |
| Issue・エピック・業務決定の文章を作成・更新 | [業務方針を決めるIssue・エピックの記述](WORKFLOW.md#業務方針を決めるissueエピックの記述) |
| Codexオーケストレーション・サブエージェント・モデル選択 | [Codexオーケストレーション](ORCHESTRATION.md) |
| 実装への指摘、レビュー修正、同じ問題の再発 | [自己改善ループ](FEEDBACK.md) と [改善記録](feedback/INDEX.md) |

共通の作業手順は [運用・検証範囲](WORKFLOW.md) を参照する。
