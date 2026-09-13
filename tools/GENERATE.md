# 変更を伴うtool

整形結果の正本はformatter。インデントや改行を文章ルールで重複定義しない。

| 処理 | cwd | コマンド |
| --- | --- | --- |
| Flutterアプリ整形 | mobile | `fvm dart run derry format` |
| Freezed等の生成 | mobile | `fvm dart run derry generate` |
| Go本体整形 | server | `go fmt ./...` |
| Agent整形 | server/agent | `go fmt ./...` |

生成前後に差分を確認する。derry generateは競合する生成出力を上書きするため、元ファイルの変更と生成結果をセットで確認する。infraには専用formatterを追加せず、現行のESLint設定を使う。

既存Dart pre-commitは [.githooks/pre-commit](../.githooks/pre-commit)、導入入口は [install-git-hooks.sh](../scripts/install-git-hooks.sh)。ステージ済みファイルだけを整形し、部分ステージを保護する既存動作を維持する。共通checkはこのステージ操作を行わない。ローカル導入済みかは `git config --get core.hooksPath` で確認する。

IDE保存時のformatは [mobile README](../mobile/README.md) を参照する。IDE設定・hook導入済みであることを、文書やファイルの存在だけで断定しない。
