# Mobile

Flutter製のiOS/Androidクライアントです。Android Studioでは、この `mobile` ディレクトリをFlutterプロジェクトとして開いてください。

Flutter/DartはFVM経由で実行します。最初に依存パッケージを取得してください。

```bash
cd /Users/shuichirohayafuji/Project/arran/mobile
fvm flutter pub get
```

日常の操作は `pubspec.yaml` のderryコマンドに登録しています。以下はすべて `mobile` ディレクトリで実行します。

| コマンド | 内容 |
| --- | --- |
| `fvm dart run derry run` | アプリ起動（必要に応じて端末を選択） |
| `fvm dart run derry run-android` | Android Emulator用API接続先で起動 |
| `fvm dart run derry devices` | 接続端末一覧 |
| `fvm dart run derry get` | 依存パッケージ取得 |
| `fvm dart run derry analyze` | 静的解析 |
| `fvm dart run derry test` | テスト |
| `fvm dart run derry format` | lib/testの整形 |
| `fvm dart run derry generate` | Freezedなどのコード生成 |
| `fvm dart run derry watch` | ファイル変更を監視してコード生成 |
| `fvm dart run derry clean` | Flutterビルド成果物のクリア |

`run` と `watch` は `Ctrl+C` で停止します。`generate` と `watch` は競合する生成出力を上書きするため、生成ファイルは直接編集しないでください。Goサーバーは別ターミナルで起動しておきます。

さらに短く `derry run` などで実行したい場合は、一度だけグローバルCLIを導入します。

```bash
fvm dart pub global activate derry
```

`derry` が見つからない場合は、`~/.zshrc` に `export PATH="$HOME/.pub-cache/bin:$PATH"` を追加してターミナルを開き直してください。グローバル導入をしなくても、上記の `fvm dart run derry ...` は使用できます。

Android EmulatorからMac上のAPIへ接続する場合:

```bash
fvm flutter run \
  --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

iOS Simulatorは既定の `http://127.0.0.1:8080` を使用します。

物理iPhoneを同じWi-Fiで使う場合は、MacのWi-Fi IPを指定します。

```bash
fvm flutter run \
  --dart-define=API_BASE_URL=http://192.168.1.20:8080
```

iOSのローカルネットワーク利用確認は許可してください。APIは `0.0.0.0:8080` で待受させますが、ルーターでポート開放は行わず、同一Wi-Fi内だけで利用します。

## Dartの自動フォーマット

Dart SDK標準の `dart format` を使用します（追加パッケージ不要）。
手動でまとめて整形する場合は、`mobile` で `fvm dart run derry format` を実行します。

### Android StudioでCommand+S

Settings → Languages & Frameworks → Flutter の **Format code on save** を有効にして Apply / OK を押してください。
Dartファイルを変更して `Command+S` で保存すると標準フォーマッターで整形します。
整形だけを実行するショートカットは、macOS標準キーマップで `Option+Command+L`（Code → Reformat Code）です。
この設定は各自のIDEで一度有効にします。

### pre-commit

リポジトリのルートで一度実行します（cloneした各環境で必要）。

```bash
./scripts/install-git-hooks.sh
```

コミット時、ステージ済みの `mobile/**/*.dart` を `fvm dart format` で整形し、
そのファイルだけを再ステージします。FVMがGit実行環境のPATHに必要です。
一部だけステージされたDartファイルがある場合は、未ステージの変更を混ぜないため
コミットを停止します。先に整形し、コミットしたい変更を選び直してください。
構文エラーやSDK実行エラーでもコミットを停止します。
整形途中でエラーになった場合は作業ツリーの差分を確認してください。
既存のGitフック設定がある場合、インストーラーは上書きせず停止します。

## ログインの利用

[サーバーの認証設計・DB登録手順](../server/docs/authentication.md)に従ってアカウントを登録してください。ログイン画面でユーザー名とパスワードを入力し、設定画面からログアウトできます。セッションは7日で失効し、再ログインが必要です。自己登録画面はありません。

初回導入時は`flutter pub get`とネイティブアプリの再ビルドが必要です（secure storageプラグインを追加）。実運用の`API_BASE_URL`にはHTTPSを設定してください。

### iOSで認証成功後に保存エラーが出る場合

ネイティブプラグインを追加する前のアプリを動かしたままHot Reload／Hot Restartしても、保存機能は組み込まれません。実行を停止し、`mobile/`で`fvm flutter pub get`、`ios/`で`pod install`を実行したうえで、以前と同じ接続先設定のRunから再ビルドしてください。ユーザーの再登録やDBの再起動は不要です。

`ios/Podfile.lock`と`ios/Pods/Manifest.lock`に`flutter_secure_storage`が存在することが依存解決の確認になります。端末に再ビルドしたアプリを入れてログイン・再起動後のセッション復元まで確認してください。
