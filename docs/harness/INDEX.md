# ハーネスIndex

このファイルは参照先の選択専用。ルール本文とコマンドはリンク先に置く。

| 変更対象 | 業務ルール | 実装ルール：アーキテクチャ | 実装ルール：実装時の作法 |
| --- | --- | --- | --- |
| mobile/ | [モバイル業務](business/mobile.md) | [モバイル構成](implementation/mobile/architecture.md) | [モバイル作法](implementation/mobile/conventions.md) |
| server/（Agentを含む） | [サーバー業務](business/server.md) | [サーバー構成](implementation/server/architecture.md) | [サーバー作法](implementation/server/conventions.md) |
| infra/ | [インフラ業務](business/infra.md) | [インフラ構成](implementation/infra/architecture.md) | [インフラ作法](implementation/infra/conventions.md) |

| 横断する変更 | 追加で読む文書 |
| --- | --- |
| mobileの画面・Widget・テーマ | [モバイルのデザインシステム](implementation/mobile/design-system.md) |
| API、DTO、認証、セッション | モバイル・サーバーの3文書と [OpenAPI](../../server/docs/openapi.yaml) |
| 金額、分類、相談、CSV、レビュー | サーバー業務。表示も変える場合はモバイル業務・構成 |
| 設定、DB、コンテナ、配備 | サーバー・インフラの3文書 |
| フォーマット、解析、テスト、生成、検証コマンド | [tool Index](../../tools/INDEX.md) |
| ハーネス自体、例外、正本間の不一致 | [運用・検証範囲](WORKFLOW.md) と tool Index |
| 実装への指摘、レビュー修正、同じ問題の再発 | [自己改善ループ](FEEDBACK.md) と [改善記録](feedback/INDEX.md) |

共通の作業手順は [運用・検証範囲](WORKFLOW.md) を参照する。
