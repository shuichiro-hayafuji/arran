# Spendable Today

支出の直前に、自分の予算・価値観・過去の判断を踏まえた助言を受けるための個人用MVPです。家計簿を完成させることではなく、「相談することで実際の意思決定が変わるか」を1週間で検証します。

一般の利用者に向けた初期方針は、[対象者と支出相談の体験](docs/product/target-users.md)に記録しています。お金の管理が苦手な人が、黒字化や期限付きの貯金目標に沿って、支出を見送る助言を受け取る体験を目指します。本人への聞き取りに基づく方針であり、他の対象者への調査と実際の利用による検証は未実施です。

領域別の構成・責務は[ドキュメントIndex](docs/README.md)から参照できます。

## ディレクトリ

データ構造は[ER図](docs/ER_DIAGRAM.md)を参照してください。

```text
server/          Go REST API、PostgreSQL、CSV取込、Agent境界、OpenAI Adapter
infra/           AWS CDK、ECS Fargate、RDS PostgreSQLの配備構成
mobile/          Flutter iOS/Androidアプリ（Android Studioで開く）
```

サーバーはGo、スマホはFlutter/Dartです。FlutterとDartのコマンドはすべてFVM経由で実行します。

## 実装済みの体験

1. 月収、残高、固定費、自由予算、貯蓄目標、価値観、助言の厳しさを保存
2. UTF-8、UTF-8 BOM、Shift_JIS/CP932のCSVを選択
3. 列の自動判定または手動マッピング後、10件プレビューを確認
4. 確定時だけPostgreSQLへ保存し、日付・金額・摘要・取込元によるfingerprintで重複排除
5. 今月の支出、自由予算残額、酒・飲み会、サブスク、直近5件を表示
6. 自然文で支出相談し、明確な結論・理由・現状・代替案・問いかけを表示
7. 支出した、見送った、減らした、保留と、満足度・後悔度を保存
8. 結果から再利用可能な行動傾向をメモリ候補として保存
9. 支出パターンを最大5件レビュー

LLMは初期状態では決定論的なモックです。OpenAIが未設定、タイムアウト、不正JSON、APIエラーの場合もモック回答へフォールバックします。

## 必要な環境

- macOS
- Go 1.26以降
- Docker Desktop（PostgreSQL用）
- FVM
- Flutter 3.44.6（`mobile/.fvmrc`で固定）
- Xcode / iOS Simulator
- Android Studio / Android SDK（Androidを動かす場合）

主要ライブラリは、Riverpod（状態管理）、go_router（画面遷移）、Dio（REST/CSV送信）です。CSV選択はオフラインでビルド可能にするため、iOS/Androidの最小ローカルプラグインを `mobile/packages/file_picker` に同梱しています。

## 起動

### 1. Go API

```bash
cd server
docker compose up -d postgres
export API_ADDR=127.0.0.1:8080
export DATABASE_URL='postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable'
export USE_MOCK_LLM=true
go run ./cmd/api
```

ヘルスチェック:

```bash
curl -sS http://127.0.0.1:8080/health
```

設定例は `server/config.example` にあります。Go APIは起動時に`server/.env`を読み込み、存在しなければリポジトリ直下の`.env`を読み込みます。OSの環境変数が指定済みの場合はそちらを優先します。`.env`はGit管理対象外です。

従来の`server/data/*.db`は削除しませんが、新しいPostgreSQLコンテナへ自動移行はしません。既存のSQLite開発データを使い続ける必要がある場合は、移行対象を確認してから一回限りの移行を別途実施してください。

### 物理端末から同じWi-Fiで接続する

MacとiPhone/Androidを同じWi-Fiへ接続し、Go APIをLAN向けに起動します。

```bash
make -C infrastructure lan-info
make -C infrastructure server-run-lan
```

別ターミナルから物理iPhoneへ起動します。MacのWi-Fi IPは自動検出されます。

```bash
make -C infrastructure mobile-run-ios-device
```

IPを自動検出できない場合は明示できます。

```bash
make -C infrastructure mobile-run-ios-device LAN_IP=192.168.1.20
```

iPhoneでローカルネットワーク利用の確認が表示されたら許可してください。Safariから `http://<MacのLAN IP>:8080/health` を開き、`{"status":"ok"}` が返れば接続可能です。macOSファイアウォールの確認が表示された場合はGoへの受信接続を許可します。

このHTTP構成はローカル開発向けです。実運用ではログイン資格情報を保護するためHTTPSを使用してください。8080番ポートをそのままインターネットに公開しないでください。

### 2. Flutter

```bash
cd mobile
fvm flutter pub get
fvm flutter run
```

iOS Simulatorは `127.0.0.1:8080` を使います。物理端末は前節のMac LAN IP、Android EmulatorではホストMacを `10.0.2.2` で参照します。

```bash
fvm flutter run \
  --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

Android Studioでは `mobile` を開き、Flutter SDKをFVMのSDKへ設定してください。インフラとGoの開発は `infrastructure/spendable-today.code-workspace` をVS Codeで開けます。

## OpenAIを使う

`server/.env`（またはリポジトリ直下の`.env`）に次を設定します。

```bash
USE_MOCK_LLM=false
OPENAI_API_KEY=your-key
OPENAI_MODEL=gpt-5.6-terra
OPENAI_REASONING_EFFORT=medium
```

その後、LAN用サーバーを起動します。

```bash
make -C infrastructure server-run-lan
```

任意の場所の設定ファイルを使う場合は、`ENV_FILE=/absolute/path/to/settings`を指定できます。起動ログに`LLM mode: OpenAI Responses API`と表示され、相談結果のチップが「AI回答」になればOpenAI経由です。キーの値はログへ出力しません。

FlutterにAPIキーは持たせません。OpenAIにはプロフィール、月次集計、カテゴリ別集計、関連する少数の取引の日付・金額・カテゴリ、相談と結果の要約だけを送ります。CSVのrawData、加盟店名、口座名、明細本文は送りません。詳細な思考過程は保存せず、ユーザー向けの短い理由のみ保存します。

## 相談のagentic loop

支出相談はLLMを1回呼んでそのまま採用するのではなく、次のbounded loopで処理します。

1. Goが残額、支出後残額、カテゴリ、予算超過、過去の後悔パターン、暫定verdictを決定
2. LLMがその事実を説明するStructured Outputを生成
3. Goが必須項目、カテゴリ、verdict、金額不足時の追加質問を検証
4. 不備があれば指摘内容と前回回答を渡して最大2回まで修正生成
5. 直らなければ従来の決定論的モックへフォールバック

したがって、金額やverdictの最終権限はGoにあり、agentic loopは説明の整合性と不足情報の補完に使います。各相談のOpenAI呼び出し回数には上限があり、無限ループにはなりません。

## サンプルCSV

`server/sample_data` に以下を用意しています。

- `utf8_general.csv`
- `shift_jis.csv`
- `column_variants.csv`
- `currency_formats.csv`

モバイルの「CSVを追加する」から選び、プレビュー後に確定してください。同じファイルを再度確定すると、既存fingerprintは重複として除外されます。

## 検証コマンド

```bash
cd server
go test ./...
go vet ./...

cd ../mobile
fvm dart format --set-exit-if-changed lib test
fvm flutter analyze
fvm flutter test
```

または:

```bash
make -C infrastructure check
```

## API

業務APIはBearer認証が必要です。`POST /auth/login`でログイン、`GET /auth/me`でユーザー確認、`POST /auth/logout`でセッションを失効します。[DBへのユーザー登録と既存データ移行](server/docs/authentication.md)を先に実施してください。

```text
GET    /health
GET    /profile
PUT    /profile
POST   /transactions/import/preview
POST   /transactions/import/commit
GET    /transactions
PATCH  /transactions/{id}
GET    /dashboard/monthly
POST   /consultations
POST   /consultations/{id}/messages
PATCH  /consultations/{id}/result
GET    /consultations
GET    /consultations/{id}
POST   /reviews/monthly
GET    /reviews/latest
GET    /memories
POST   /memories
PATCH  /memories/{id}
DELETE /memories/{id}
```

## 主な設計判断

- 金額は整数の円で保存し、浮動小数点を使わない。
- DB登録ユーザーのBearer認証と所有者認可を行う。[認証設計・登録手順](server/docs/authentication.md)を参照。CSVプレビューは単一プロセス内で保持する。
- CSVプレビューはメモリ上で30分保持し、確定した時だけDBへ保存する。
- カテゴリ分類は、保存済み加盟店ルールとローカルルールで行う。加盟店名を保護するため、OpenAIによる分類は無効にしている。
- 月次レビューはローカル検出を正とし、LLMは説明補助に限定する。
- 日本時間で月を決め、保存時刻はRFC 3339で保持する。
- OpenAI Responses APIは`server/internal/infrastructure/openai`へ隔離し、不正JSONや失敗時はAgentがモックへ戻す。Goの`domain.EvaluateSpending`がverdictを決定し、モデルはその説明だけを担う。

サーバーの責務分離と依存方向は[server/README.md](./server/README.md#アーキテクチャ)に記載しています。FlutterのHTTP契約は[OpenAPI定義](./server/docs/openapi.yaml)を正本として維持しています。

## MVPの制限

- CSV列名と日付形式は代表的な形式のみ。複雑な複数明細行や独自フォーマットは手動マッピングが必要。
- サブスク検出は同じ加盟店の約月次の支出を候補化するヒューリスティック。
- メモリ候補は構造化データだが、ユーザーによる承認ワークフローは簡易。
- 通知、銀行API、決済制御、ベクトルDB、クラウド配備は対象外。
- Androidの通常動作を考慮しているが、主な実行確認対象はiOS Simulator。

## 1週間の価値検証

毎回の支出前に相談し、少なくとも次を記録します。

- 相談を起動した回数
- 助言後に「見送った／減らした」割合
- 翌日の満足度と後悔度
- 過去結果を参照した助言が役立った具体例

最初に試す相談:

1. 「今から6,000円くらいで飲みに行っていい？」
2. 「2万円のイヤホンを買うか迷っている。仕事にも使う」
3. 「ほとんど使っていない1,490円のサブスクを継続していい？」
