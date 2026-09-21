# 公式資料と採用範囲

2026-09-21に公式ページの本文を確認した。GoogleのCLはレビュー対象の変更単位を指す。本Skillでは、その分割と説明の原則をコミット単位に応用する。

| 資料 | 採用した指針 |
| --- | --- |
| [Google: Small CLs](https://google.github.io/eng-practices/review/developer/small-cls.html) | 一つの関心ごとで完結する小さな変更、関連テストの同梱、大きなリファクタリングの分離、各段階でビルドを壊さない分割 |
| [Google: Writing good CL descriptions](https://google.github.io/eng-practices/review/developer/cl-descriptions.html) | 単独で意味が伝わる短い件名、空行で分けた本文、何を変えるかと変更理由、最終差分に合う説明 |
| [Pro Git: Contributing to a Project](https://git-scm.com/book/en/v2/Distributed-Git-Contributing-to-a-Project) | 論理的に独立したコミット、部分ステージ、空白エラー確認、件名と補足本文、トピックブランチ |
| [Git: git-push](https://git-scm.com/docs/git-push) | remoteとrefspec、upstream設定、non-fast-forward拒否による履歴保護、期待SHAを明示したforce-with-lease |

## このリポジトリでの適用

- `feature/<Issue番号>`、日本語の件名、必要に応じた日本語の本文はユーザー指定。公式資料の英語の命令形や50文字・72桁の目安を、日本語の厳密な文字数制限にはしない。
- 小ささを行数の上限で判定しない。関心ごととビルド可能性を優先する。
- 検証はArranのハーネスを使う。文書だけの変更にアプリのビルドや新規テストを一律に追加しない。
- ステージ済みスナップショットの検証、公開済み履歴の書き換え範囲、push後のSHA照合は、上記の原則をCodexの作業手順へ落とし込んだ運用上のルール。
- `--force-with-lease`の期待SHA省略形は、バックグラウンドfetchがリモート追跡情報を更新すると保護の前提が変わる。履歴の書き換えを許可された場合も、確認済みSHAを明示する。
