# 運用・検証範囲

## 変更の進め方

1. Indexから対象領域の3文書を選び、該当ルールIDとコードを確認する。
   [改善記録](feedback/INDEX.md) で対象領域・関連ルールIDの未完了項目を確認し、今回の変更に関係するものを検証計画へ含める。
2. 変更で影響するAPI・永続化・利用者の操作・他領域を確認する。
3. 実装と必要な回帰テストを更新し、toolの対象scopeを実行する。
4. 業務・依存方向・データ境界は差分レビューで確認する。toolの成功で代用しない。
5. 実行結果と未検証範囲を報告する。ルールを変えた場合はIDを維持して本文と関連検証を更新する。
6. 実装への指摘があった場合は [自己改善ループ](FEEDBACK.md) を実施し、改善記録の状態・正本への反映先・残る検証を報告する。

## 正本

- 開発上の責務・作法はこのハーネス、HTTP契約は [OpenAPI](../../server/docs/openapi.yaml)、実DBスキーマは [PostgreSQL migration](../../server/internal/infrastructure/persistence/postgres/002_auth.sql) を含むバージョン付きSQLを正本とする。
- [既存設計資料](../architetcure.md) とREADMEは説明資料。コードとの差異を発見した場合、過去の記述だけを根拠にコードを作り替えない。意図を確認し、必要な文書を同じ変更で整合させる。
- 業務ルールは利用者に約束する結果を記述する。配置・依存はアーキテクチャ、エラー処理や変更の進め方は実装作法、機械的なコマンドはtoolへ置く。
- 同じルールの本文を複数ファイルにコピーせず、IDとリンクで参照する。

## 既存例外と未整備事項

| ID | 対象・現状 | 当面の扱い・解消条件 |
| --- | --- | --- |
| EX-M01 | sessionはアプリ寿命のProviderContainer・Secure Storage・ChangeNotifierを使用 | sessionの既存認証経路に限定許可。通信はpublicApiClientProviderから注入し、画面のProviderScope破棄後も失効を完了させる。loginはpublicApiClientProviderと通常のRiverpod + MVIを使う。Session DomainのJSON変換はDTOへ分離済み。層を統一する際は認証・永続化・ユーザー切替の回帰テストを通す |
| EX-M02 | 相談結果→profileのメモリ、CSV確定→dashboardのViewModelが他featureのIntent・Providerを参照 | 既存連携を維持。新しい機能間依存は責務を説明する。公開query等へ移す場合は保存後の表示更新を検証 |
| GAP-M01 | 取引詳細Screenは保存中をsetStateで管理し、既存画面に生の例外表示や二重操作対策が不十分な経路がある | 新規実装の見本にしない。該当操作の変更時にMVIの保存状態・安全なエラー表示・重複操作防止へ寄せ、操作テストで確認する |
| GAP-M02 | dashboard/profile等に直接指定の余白・文字・色が残る | 新規・変更部分はデザインシステムに従う。一括リデザインはしない。対象がtokenへ移行し画面確認できた範囲で解消する |
| EX-S01 | applicationの構築経路がAgent module・agentadapterをimportする | 現行構築を許可。業務処理へプロバイダー固有DTOを持ち込まない。構築を移す場合はapp側とテストを更新 |
| GAP-D01 | READMEのinfrastructure/参照、設計資料のViewModel配置が実装と不一致 | 新規作業は実在する配置と本ハーネスに従う。説明資料の修正は別途可能。存在しない旧ディレクトリを再作成しない |

例外を追加する場合はID、対象、理由、許可範囲、解消条件をこの表に記録する。一律除外や既存違反の無条件な追認はしない。

## この初版で保証する範囲

- 文書の分割と参照経路、既存formatter・analyzer・testへの共通CLIを提供する。
- モバイルの共通HTTP生成境界（A-M07）はtoolで字句検査する。依存方向全体、API互換性、送信データ最小化は差分レビューと既存テストで確認する。
- PostgreSQL統合テストは専用scopeで実行する。通常のserver scopeではDBを使わない。実機E2EとAWS配備はこのCLIに含めない。
- 新規checkerを追加する際は「違反を入れると失敗する」「既存例外を誤検出しない」をテストする。
