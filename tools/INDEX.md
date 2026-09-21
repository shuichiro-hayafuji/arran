# Tool Index

決定論的な処理の入口。ルール本文は [ハーネスIndex](../docs/harness/INDEX.md) を参照。

| 目的 | 定義・手順 |
| --- | --- |
| ドキュメント参照確認、整形チェック、静的解析、テスト | [チェックtool](CHECKS.md) / [CLI](harness.py) |
| Git/Codex hooksの導入・解除、ステージ内容の検査 | [開発用hooks](HOOKS.md) |
| 整形を適用、コード生成、既存hook | [変更を伴うtool](GENERATE.md) |

通常はリポジトリルートで `python3 tools/harness.py check <scope>` を実行する。対象と実コマンドの確認だけなら `--dry-run` を付ける。
