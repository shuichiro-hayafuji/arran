# 開発用 hooks

Git の pre-commit と Codex の応答完了通知は別の仕組みです。依存のインストール、環境構築、コミット、push は行いません。全体ビルド・テストは [既存の check](CHECKS.md) で明示的に実行します。調査時点でこのリポジトリに CI workflow はありません（PR テンプレートは CI ではありません）。

## Git pre-commit

既存の [.githooks/pre-commit](../.githooks/pre-commit) から [git_hooks.py](git_hooks.py) を呼び出します。

| 対象 | 処理 |
| --- | --- |
| ステージした mobile/lib・mobile/test の手書き Dart | 固定 SDK の dart format |
| ステージした server の Go | gofmt。Agent submodule 内は親の hook の対象外 |
| ステージ内容全体 | git diff --cached --check（空白・競合マーカー） |
| 変更した Python・JSON・Shell | 構文検査。Python は実行せず compile、Shell は bash -n |
| mobile/lib の変更 | 既存の A-M07 検査をステージ内容の一時コピーに適用 |
| 変更した infra の TS・JS・CJS・MJS | 既存 ESLint とステージした設定を使用。自動修正なし |
| mobile の生成関連入力の変更 | 後述の検証記録とステージ内容の一致確認 |

生成済み Dart と mobile/packages は自動整形しません。品質検査は型検査・テストの代わりにはなりません。各外部コマンドは原則30秒で打ち切り、重い再生成は hook 内で実行しません。

整形するファイルに未ステージの変更がある場合は、全ファイルの整形を始める前に停止します。対象を手動で整形し、意図した差分だけをステージして再実行してください。stash や git add -A は使いません。整形・品質検査は一時コピーで行い、成功後に対象ファイルだけを書き戻し、整形した内容だけを再ステージします。実行中に検知した別の編集・ステージ操作はエラーにします。hook 実行中の同時編集は避けてください。

生成物の検証記録がない場合は、整形結果を反映した後で停止する場合があります。表示された対象と差分を確認し、次の生成検証を実行します。その他のチェック失敗時は整形結果を作業ファイルへ適用しません。

### 有効化と解除

リポジトリルートで実行します。Python 3・Git・必要な既存ツールが前提です。Dart は mobile/.fvmrc に一致する既存 SDK の `.fvm/flutter_sdk` または `FVM_CACHE_PATH`（省略時は `~/fvm/versions`）から直接実行し、FVM/Flutter の自動取得処理を起動しません。ESLint は infra/node_modules の既存インストールを使用します。

```sh
./scripts/install-git-hooks.sh
git config --show-origin --get core.hooksPath
git hook run pre-commit
```

`core.hooksPath=.githooks` がこのリポジトリの `.git/config` に設定されます。同じ Git リポジトリの linked worktree でも共有されます。別の hooksPath や既存の標準 pre-commit がある場合は上書きせず停止します。競合する既存 hook は内容を確認して統合してください。

解除は次のコマンドです。他のリポジトリやグローバル設定を変更しません。解除後は、もともとの継承設定や標準 hooks が再び使われる場合があります。

```sh
./scripts/install-git-hooks.sh --uninstall
```

設定を書き込めない場合も、一回だけ明示的に発火を確認できます。これは有効化状態を保存しません。

```sh
git -c core.hooksPath=.githooks hook run pre-commit
```

### 生成物の更新漏れ確認

対象は既存の Freezed / build_runner の Dart 生成物です。生成の意味上の入力を限定しすぎないよう、mobile 配下の Dart（同梱パッケージを含む）・YAML・lock と .fvmrc を検証記録に含めます。このため、生成結果が変わらない Dart の編集でも再検証を求める場合があります。

1. 変更を確認し、コミットしたいファイルだけをステージします。
2. pre-commit で整形します。検証記録がなければここで停止します。
3. 次のコマンドで、ステージ内容を一時ディレクトリへコピーして再生成・比較します。

```sh
python3 tools/git_hooks.py verify-generated
```

既存の `derry generate` と同じ `build_runner build --delete-conflicting-outputs` を一時コピーで実行します。`dart run` による依存の暗黙取得を避けるため、取得済み build_runner のエントリーポイントを Dart で直接起動します。package_config のリポジトリ内依存は一時コピーへ向け、外部依存は取得済みのキャッシュを使います。作業ディレクトリの生成物や index は書き換えません。比較対象は mobile/lib の `.freezed.dart` と `.g.dart` の追加・変更・削除です。制限時間は180秒です。

成功時だけ、Git 管理ディレクトリ内の `arran-generated-check` に検証対象のハッシュを保存します。pre-commit はハッシュの一致だけを検査するため、再生成を待ちません。別の mobile 入力や出力をステージすると記録は無効になります。記録は開発補助用のキャッシュであり、改ざん防止や CI の代替ではありません。

差分があった場合は対象名を表示して停止します。作業中の差分を保護したうえで、従来の生成コマンドを**明示的に**実行し、結果をレビューしてステージし直してください。

```sh
cd mobile
fvm dart run derry generate
```

その後、ルートで `verify-generated` と `git hook run pre-commit` を再実行します。依存の不足・壊れたキャッシュ・SDK不一致の場合は既存の依存環境を手動で整えてから再実行してください。検証器は依存のインストールも復旧も行いません。package_config は現在の pubspec/lock に対応する取得済み環境を前提とします。将来、別の生成処理やテンプレート入力を追加する場合は検証対象も更新してください。

## Codex の応答完了通知

公式の [Hooks](https://learn.chatgpt.com/docs/hooks) と [Configuration Reference](https://learn.chatgpt.com/docs/config-file/config-reference) を2026-09-21に確認しました。

- 公式形式は `<repo>/.codex/hooks.json` の `hooks.Stop`。コマンドには stdin で JSON が渡されます。
- Stop は応答を終えるタイミングのイベントです。コマンドごとの PostToolUse や、セッションを閉じる SessionEnd とは異なります。
- プロジェクトの .codex 層と hook 定義の信頼確認が必要です。定義変更後は再確認が必要になります。
- プロジェクト内の `notify` は無視される仕様のため使いません。ユーザー全体の設定による代用もしません。

利用環境は ChatGPT/Codex app `26.915.31945`、同梱 Codex `0.155.0-alpha.9.2`。同梱バイナリの `features list` で `hooks stable true` を確認しました。PATH 上の CLI `0.155.1` とは区別しています。

設定の原本は [codex-hooks.json](../scripts/codex-hooks.json)、処理は [codex-notify.py](../scripts/codex-notify.py) です。macOS のローカル通知で「応答が完了しました。検証結果は回答を確認してください。」と表示します。会話内容・パスを通知へ埋め込まず、外部へ送信しません。通知失敗は警告として返し、Codex に作業の継続や再試行を強制しません。

### 導入と解除

```sh
./scripts/install-codex-hooks.sh
```

このスクリプトはリポジトリ内の `.codex/hooks.json` だけを作成し、既存設定があれば上書きしません。原本と導入後のファイルを別々に編集せず、変更時は原本の差分を導入先へ反映してください。

Codex でプロジェクトを開き直し、表示される hook 定義を確認して信頼してください。公式資料で案内されている CLI の確認入口は `/hooks` です。アプリ側で信頼確認を行えない場合は、自動で信頼済みに書き換えず、通知の有効化は保留にしてください。アプリの標準通知は代案ですが、リポジトリ単位であることを確認できていないため、この作業では変更しません。

解除は `.codex/hooks.json` の Arran の Stop ハンドラーを削除します。このファイルが導入した内容だけならファイルを削除できます。他の hook が追加されている場合はファイル全体を削除しないでください。スクリプトだけを残しても通知は発火しません。

macOSへの通知要求だけを試すコマンドです。これは Codex イベントの実発火テストではありません。

```sh
printf '%s\n' '{"hook_event_name":"Stop"}' | python3 scripts/codex-notify.py
```

信頼確認後、短い応答を1回依頼し、応答終了時の通知を確認します。表示されない場合は、macOS の通知許可や集中モードも確認してください。Stop hook は他の hook と同時に実行されるため、別の Stop hook が作業を継続させる環境では通知が最終終了より早く出る場合があります。

## 導入時の確認結果

- Git と Codex の設定保存は、この環境では権限を切り替えても `Operation not permitted`。自動発火の有効化は未完了です。通常のターミナルで上記の導入コマンドを実行してください。
- Git hook は `git -c core.hooksPath=.githooks hook run pre-commit` で実発火を確認しました。現在の実リポジトリはステージ変更なしです。
- 生成比較の正常系・差分検出はテスト用の生成処理で確認します。これは実プロジェクトの build_runner 成功を意味しません。
- 一時リポジトリで、コミットせず `git hook run pre-commit` を発火させ、正常系・失敗系・差分保護を回帰テストします。検証コマンドは `python3 -B -m unittest discover -s tools -p 'test_*.py'` です。
- 実際の生成処理は、既存 package_config が参照する `/private/tmp/arran-login-fvm/versions/3.44.6/packages/flutter/pubspec.yaml` がないため失敗しました。生成物の整合性を確認済みとはしていません。
- 通知スクリプトの直接実行は、権限切り替え後も osascript が構文エラー `-2740` で終了し、警告JSONを返しました。別方式の JXA でも macOS サービスへの接続エラーとなり、通知の表示は確認できていません。Codex Stop の実発火と通知バナーの目視は未確認です。設定の存在や通知コマンドの終了コードだけで有効化済みとは判断しません。
