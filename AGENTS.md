# Arranの変更作業

作業開始時に [ハーネスIndex](docs/harness/INDEX.md) を読み、変更対象に対応する業務ルール・アーキテクチャ・実装作法と共通toolの文書を選ぶ。全領域の文書を無条件に読む必要はない。

- ユーザーの明示的な指示を優先する。通常の実装で都度承認を求めない。
- 着手時に [改善記録](docs/harness/feedback/INDEX.md) の対象領域の未完了項目を確認する。実装への指摘を受けたら [自己改善ループ](docs/harness/FEEDBACK.md) に従い、修正・再発防止・検証結果を同じ作業で記録する。
- 現在の責務分担を維持する。構成変更が必要な場合は理由と影響を示し、関連ルールと検証も同じ変更で更新する。
- 決定論的な検証は [tool Index](tools/INDEX.md) の入口を使う。フォーマットを独自の文章ルールで再定義しない。
- 作業中の既存差分を保持する。生成物・Agent submodule・同梱パッケージの扱いは該当文書に従う。
- 完了時は変更内容、実行した検証、未実行・失敗・残る例外を報告する。静的検証だけで実機・DB・AWSの動作確認済みとしない。

## PRの作成・更新

PRの作成・本文更新・レビュー対応では [arran-pull-request Skill](.agents/skills/arran-pull-request/SKILL.md) を読み、[PRテンプレート](.github/pull_request_template.md) を使う。本文・コメントの作成と投稿範囲はSkillに従う。

## ブランチ・コミット・push

ブランチ作成・コミットの分割・commit・pushでは [arran-commit Skill](.agents/skills/arran-commit/SKILL.md) を読む。関連Issueがある場合のブランチ名、日本語のコミットメッセージ、関心ごととビルド可能性を基準にした分割はSkillに従う。
