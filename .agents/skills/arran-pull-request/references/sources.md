# 公式資料と採用範囲

2026-09-21に以下の公式ページの本文を確認した。GoogleのCL（レビュー対象の変更単位）とGitHubのPRは同一の仕組みではないが、変更範囲・説明・レビューの原則を応用する。

| 資料 | 採用した指針 |
| --- | --- |
| [Google: Small CLs](https://google.github.io/eng-practices/review/developer/small-cls.html) | 一つの目的で完結する変更、関連テストの同梱、大きな整形・リファクタリングの分離、依存する変更の順序とビルド可能性 |
| [Google: Writing good CL descriptions](https://google.github.io/eng-practices/review/developer/cl-descriptions.html) | 何を・なぜ変えるか、単独で意味が分かる短いタイトル、重要な背景や判断、最終差分と説明の一致 |
| [Google: What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html) | 設計・動作・複雑さ・テスト・文書を対象に応じて確認する。テストは成功だけでなく妥当性を見る。個人的な好みは必須修正と分ける |
| [Google: How to handle reviewer comments](https://google.github.io/eng-practices/review/developer/handling-comments.html) | 指摘の意図を理解し、コードや文書を改善する。意見の相違には技術的根拠とトレードオフで回答する |
| [Pro Git: Contributing to a Project](https://git-scm.com/book/en/v2/Distributed-Git-Contributing-to-a-Project) | トピックブランチ、論理的に独立した変更、説明可能な履歴。コミットの具体的な運用はarran-commitへ集約する |

## リポジトリ固有のルール

- 日本語、テンプレート、レビュアーのチェックボックス、関連Issueの`Closes`指定はユーザー指定。GoogleやGitが必須にしている形式ではない。
- Draftの使い分け、コメントの重複防止、投稿後の再取得は、レビュー準備と記録の正確さを保つための運用上のルール。
- Googleの全観点を全PRの必須チェックリストにはしない。変更の性質に合う観点を選ぶ。テスト・フォーマット・構成の正本はArranのハーネスとする。
- 自己レビューやCI成功を他者の承認として扱わない。このSkillはマージ権限やブランチ保護設定を変更しない。
