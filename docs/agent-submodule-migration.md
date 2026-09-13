# Agent submodule への移行

`server/agent` は、独立 Go モジュール
`github.com/shuichiro-hayafuji/arran_agent` として準備済みです。以下の手順で、
依存境界を変えずにローカルの準備済みディレクトリを GitHub へ接続します。

## 事前条件

- GitHub リポジトリは `shuichiro-hayafuji/arran` と
  `shuichiro-hayafuji/arran_agent` です。
- GitHub 側に既存ファイルや履歴がある場合は、先に内容を確認・統合してください。
  以下は公開ブランチが `main` であり、リモート履歴を上書きしない前提です。
- 各コミットの前に、末尾のテストを実行してください。

## 1. 準備済み Agent モジュールを公開する

Arran のチェックアウトから実行します。

```sh
cd server/agent
git init
git branch -M main
git remote add origin https://github.com/shuichiro-hayafuji/arran_agent.git
git add .
git commit -m "Extract bounded agent module"
git push -u origin main
cd ../..
```

Agent リポジトリの `go.mod` の module path は、必ず
`github.com/shuichiro-hayafuji/arran_agent` にしてください。

## 2. Arran 本体へ submodule として登録する

このチェックアウトの親リポジトリには、まだ remote が登録されていません。未登録の
場合だけ、一度追加します。

```sh
git remote add origin https://github.com/shuichiro-hayafuji/arran.git
```

手順 1 の後、`server/agent` はすでに Git の作業ツリーです。その既存チェックアウトを
submodule として登録します。

```sh
git submodule add --force -b main \
  https://github.com/shuichiro-hayafuji/arran_agent.git server/agent
git add .gitmodules server/agent server/go.mod server/go.sum \
  server/internal/agentadapter server/internal/application \
  server/internal/app server/internal/handler \
  server/internal/infrastructure/openai docs README.md server/README.md
git commit -m "Use arran_agent as server submodule"
git push -u origin main
```

Arran の初回コミットであれば、残りのアプリケーションファイルも意図どおり同じ
初回コミットへ追加してください。履歴がすでにあるリモートを統合するために、広範な
force push を使わないでください。

## 3. 以後の clone と更新

新しいチェックアウトでは、submodule も初期化します。

```sh
git clone --recurse-submodules https://github.com/shuichiro-hayafuji/arran.git
```

既存チェックアウトでは次を実行します。

```sh
git submodule update --init --recursive
```

Agent コードを変更するときは、最初に `server/agent` 内でコミット・push します。次に
Arran のルートで、更新された gitlink（`git add server/agent`）を別の親コミットとして
記録します。submodule のチェックアウト外で Agent を通常の Go モジュールとして使う
場合は、`server/go.mod` のバージョンを更新してください。ローカルの
`replace ... => ./agent` は submodule 開発のため意図的に残します。

## 確認

```sh
(cd server/agent && GOCACHE=/private/tmp/arran-go-cache GOTELEMETRY=off GOPROXY=off go test ./...)
(cd server && GOCACHE=/private/tmp/arran-go-cache GOTELEMETRY=off GOPROXY=off go test ./...)
git submodule status
```
