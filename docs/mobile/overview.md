# Mobileのアーキテクチャ

Flutter製のiOS / Androidクライアントです。サーバーのHTTP契約を利用し、金融上の最終判断はサーバーへ委譲します。

## 技術構成

- Flutter / Dart。バージョンは `mobile/.fvmrc` で固定し、コマンドはFVM経由で実行する。
- Riverpod: 状態管理と依存性注入。
- `go_router`: 画面遷移。
- `dio`: Go APIとのHTTP通信。
- `freezed`: 状態・Intentなどの不変データ型。
- `mobile/packages/file_picker`: CSV選択用の同梱ローカルプラグイン。

## レイヤーと責務

```text
features/<feature>/
├── screens/       UI、ユーザー操作の受け付け
├── providers/     ViewModel、状態、Repositoryの組み立て
├── repository/    API呼び出しとDTO変換
├── repository/dto JSONとの変換
└── domain/        画面が扱うドメインモデル
```

全機能で共有する通信経路は `mobile/lib/core/network/ApiClient` です。APIのURLは `--dart-define=API_BASE_URL=...` から読み込み、未指定時は `http://127.0.0.1:8080` を使います。Flutter側にOpenAI APIキーは保持しません。

主な機能境界は次のとおりです。

- `profile`: 月収、残高、固定費、自由予算、貯蓄目標、価値観、助言の厳しさ
- `transactions`: CSVプレビュー・確定、明細一覧、カテゴリ変更
- `dashboard`: 月次集計
- `consultation`: 支出相談、追加メッセージ、支出後の結果記録
- `review`: 月次の見直し候補
- `memories`: 相談結果から得た判断傾向の保存・編集・削除

支出可否の正本はサーバーの業務ルールです。モバイルは入力、表示、操作状態、APIレスポンスの表示変換を担い、金融判断を再実装しません。

## サーバーとのデータフロー

```text
CSV選択
  → POST /transactions/import/preview
  → プレビュー表示
  → POST /transactions/import/commit

支出相談入力
  → POST /consultations
  → Goが計算した事実とLLMの説明を表示
```

CSVはプレビューと確定の2段階です。古いプレビューを確定しないこと、保存中の二重操作を防ぐこと、APIエラー時に入力を失わないことを画面側で扱います。

## 実行時の接続先

通常の手順とFVM、生成コード、フォーマット、テストのコマンドは [mobile/README.md](../../mobile/README.md) を参照します。

- iOS Simulator: `http://127.0.0.1:8080`
- Android Emulator: `http://10.0.2.2:8080`
- 物理端末: MacのLAN IPを `API_BASE_URL` に指定

## 関連資料

- [モバイル業務ルール](business.md)
- [モバイル実装ルール](architecture.md)
- [モバイル実装時の作法](conventions.md)
- [モバイルのデザインシステム](design-system.md)
- [OpenAPI定義](../../server/docs/openapi.yaml)
