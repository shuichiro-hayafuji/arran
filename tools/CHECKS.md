# チェックtool

Python 3標準ライブラリの `harness.py` を共通入口とする。ファイル整形を適用せず、依存のインストール・Gitのステージ変更・配備を行わない。各ツール自身のキャッシュやビルド成果物は作成される場合がある。

| scope | 内容 |
| --- | --- |
| docs | AGENTSとハーネス・tool文書のローカルMarkdownリンクの存在確認 |
| mobile-architecture | A-M07の共通HTTP生成境界をPythonで検査（FVM不要） |
| mobile | A-M07検査、FVM Dartのformat確認、Flutter analyze、Flutter test |
| server | A-S01〜A-S05とB-S08のGo依存境界検査、Go format確認、serverとAgentそれぞれのvet・test。テストDB環境変数を除いて実行 |
| infra | npm scriptsのbuild・lint・test |
| postgres | ARRAN_TEST_DATABASE_URLを必須として、PostgreSQL所有者・セッション統合テストを実行 |
| all | docs・mobile・server・infra。postgresを含まない |

```sh
python3 tools/harness.py check docs
python3 tools/harness.py check mobile-architecture
python3 tools/harness.py check mobile --dry-run
python3 tools/harness.py check server
python3 tools/harness.py check infra
python3 tools/harness.py check postgres
python3 tools/harness.py check all
```

- 実行ごとにPASS・FAIL・BLOCKEDを表示する。FAILがあれば終了コード1、FAILなしでBLOCKEDがあれば2、全選択項目成功は0。
- コマンド未導入や専用DB未設定はBLOCKED。環境制約によるコマンド失敗はFAILとして表示されるため、出力に基づき原因を報告する。
- server scopeでDBテストがskipされてもPostgreSQL検証済みとは扱わない。専用postgres scopeの成功が必要。
- `--dry-run` は実行計画であり成功の証拠ではない。docs scopeの実行でも実在確認のみで、文書内容の正しさは保証しない。
- 前提: Python 3、Go/gofmt、FVMと固定SDK・取得済みFlutter依存、Node/npmと取得済みinfra依存。自動導入はしない。
- mobile/packagesは通常mobile scopeの整形対象外。変更時は該当packageをcwdとしてFVMのanalyze/test等を個別に実施し、結果を報告する。
- API互換性と実機E2Eは未実装。import境界検査が扱わない意味上のレビュー事項は各ルールを参照。

CDK synthが必要な変更ではinfraで `npm run cdk -- synth` を別途実行する。DockerやAWS環境の必要条件を確認し、通常checkの結果と分けて報告する。

ハーネスCLI自体を変更した場合は、ルートで `python3 -B -m unittest discover -s tools -p 'test_*.py'` とdocs scopeを実行する。CLIのテストは失敗判定・実行範囲等をモックで検証するもので、アプリのテスト成功を意味しない。

## Goサーバー依存境界の検出範囲

[server_architecture.py](server_architecture.py) はserverと独立Agentのproduction Goファイルを字句検査する。domainの外部技術依存、applicationの実装依存、Agentから親internalへの依存、OpenAI・PostgreSQL実装の不正参照、配置とpackage名の不一致、依存方針が未登録の新しいinternalパッケージを検出する。Agentの`Transaction` DTOはB-S08の許可項目（日付・金額・カテゴリ）との完全一致を確認する。server/allに組み込み済み。

Go ASTや型解決は行わず、`*_test.go`のimportは検査対象外とする。金融計算の処理内容、DTOの既存項目に入る値の意味、interfaceの実装充足、循環依存は差分レビューとGoのvet・testで確認する。例外はimport元・import先・ルールID・例外IDを完全一致で登録し、WORKFLOWにも理由と解消条件を記載する。

## モバイルHTTP境界の検出範囲

[mobile_architecture.py](mobile_architecture.py) はmobile/libのDartを検査し、core/network/api_client.dart以外でDio識別子、ApiClientの直接生成・init/new参照を検出するとファイル・行番号・A-M07を表示してFAILにする。mobile/allにも組み込み済み。mobile/testとmobile/packagesは対象外。sessionの除外はない。

コメント・文字列を除く字句検査であり、Dart ASTや型解決は行わない。文字列補間内の実行コード、型別名・別HTTPライブラリ経由の迂回、Providerの選択・寿命の正しさは保証しない。これらは差分レビューとFlutterテストで確認する。通常のDartコメント・文字列を想定し、入れ子のブロックコメントは非対応。
