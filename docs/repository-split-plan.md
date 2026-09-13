# Arran / Agent リポジトリ分割計画

## 完成形

- `arran_agent`: Agentのソースと独立したGit履歴。
- `arran`: アプリ本体。`server/agent`はsubmoduleとして特定のAgentコミットを参照する。
- 初回コミットメッセージは両方とも `first commit`。

## 実施結果

- Agent専用の `.gitignore` を追加した。
- `server/agent` を独立Gitリポジトリ化した。
- Agentの `first commit` は `4d0f2e5`。
- Agentのoriginを `https://github.com/shuichiro-hayafuji/arran_agent.git` に設定した。
- Agent、Serverそれぞれの `go test ./...` が成功した。
- GitHub APIへの接続は権限昇格後も失敗。リモート履歴、権限、pushは未確認。
- 本体のsubmodule登録は `.git/index.lock: Operation not permitted` で失敗。本体のコミットは未作成。

## 残りの操作

通常のターミナルで実施する。途中で失敗したら、その原因を確認してから次へ進む。

### 1. リモート確認

```sh
cd /Users/shuichirohayafuji/Project/arran
gh repo view shuichiro-hayafuji/arran --json isEmpty,defaultBranchRef,viewerPermission
gh repo view shuichiro-hayafuji/arran_agent --json isEmpty,defaultBranchRef,viewerPermission
```

以下の登録手順は、両リポジトリが空で書き込み権限がある場合のもの。
既存履歴がある場合はfetchして内容を確認し、統合方針を決める。force pushは行わない。

### 2. Agentを先にpush

```sh
git -C server/agent push -u origin main
```

### 3. 本体にsubmoduleを登録してコミット

```sh
git submodule add https://github.com/shuichiro-hayafuji/arran_agent.git server/agent
git add --all
git diff --cached --check
git diff --cached --stat
git ls-files --stage -- server/agent
git ls-files -ci --exclude-standard
```

`server/agent`がモード`160000`の1エントリであることを確認する。
除外対象が追跡されていないこと、登録対象に秘密情報・実データがないことを確認してから実行する。

```sh
git commit -m "first commit"
git remote add origin https://github.com/shuichiro-hayafuji/arran.git
git push -u origin main
```

originが既に登録済みの場合はURLを確認し、重複して追加しない。

### 4. 別ディレクトリから再現確認

`git clone --recurse-submodules`で新しいチェックアウトを作り、AgentとServerのテスト、Flutterの解析・テストを実施する。
実際のファイル配置に合わせて起動ドキュメントを確認・修正し、PostgreSQL、API、Flutterを起動する。
モックLLMでプロフィール保存、支出相談、結果保存まで確認する。
READMEの古い`infrastructure/`参照は実行前に修正が必要。

完了条件は、両リモートにコミットが存在し、新規cloneでsubmoduleを取得でき、アプリの基本操作が成功すること。
