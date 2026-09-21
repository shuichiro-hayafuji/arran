# 変更を伴うtool

整形結果の正本はformatter。インデントや改行を文章ルールで重複定義しない。

| 処理 | cwd | コマンド |
| --- | --- | --- |
| Flutterアプリ整形 | mobile | `fvm dart run derry format` |
| Freezed等の生成 | mobile | `fvm dart run derry generate` |
| Go本体整形 | server | `go fmt ./...` |
| Agent整形 | server/agent | `go fmt ./...` |

生成前後に差分を確認する。derry generateは競合する生成出力を上書きするため、元ファイルの変更と生成結果をセットで確認する。infraには専用formatterを追加せず、現行のESLint設定を使う。

既存の [.githooks/pre-commit](../.githooks/pre-commit) は、コミット対象のDart・Go整形と軽量チェックを行います。部分ステージを保護し、共通check自体はステージ操作を行いません。Git hook・Codex応答完了通知の導入、有効化、解除、生成物の検証は [開発用hooks](HOOKS.md) を参照してください。

IDE保存時のformatは [mobile README](../mobile/README.md) を参照する。IDE設定・hook導入済みであることを、文書やファイルの存在だけで断定しない。
