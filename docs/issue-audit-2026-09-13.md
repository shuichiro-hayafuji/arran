# Issue棚卸し・反映原稿（2026-09-13）

## 反映状況

**GitHub未反映。新規作成・本文更新・コメント投稿・Closeはいずれも未実行。**

GitHubの読み取り接続からOpen 12件、Closed 0件とリモートのコミットを確認した。CLIは接続失敗、ブラウザは管理者ポリシーで拒否された。書き込み可能な許可済み接続が必要。本書は再開時の原稿であり、Issue登録済みの代わりにはならない。再開時はIssueの最新本文・コメントを取得して重複と競合を確認する。

評価対象: `e4f7d03e13659f77e0f12c5d4377b38591745a48`。ローカルHEADとGitHubの最新コミットが一致。アプリコードは変更していない。

## 判定方針と結果

- Issue本文の完了条件全体を満たした証拠があるものだけcompletedでCloseする。
- 一部実装済みは、実装の根拠と未検証範囲を追記してOpenを維持する。
- コードの存在、テストの存在、テスト成功、実機・DB・AWSの検証成功を区別する。
- **今回、全完了を確認できた既存Issueは0件。Close対象はない。**
- 初回コミット以前の実装経緯は履歴から復元できない。`ada3b97`は「現履歴で最初に収録を確認できるコミット」であり、実装した日・変更の細分化まで証明するものではない。

## コミットの根拠

| コミット | 確認できた変更・状態 |
| --- | --- |
| [ada3b97 — first commit](https://github.com/shuichiro-hayafuji/arran/commit/ada3b971c13df5e82e048d57d627b3ea68f56d43) | feature構成、認証・所有者分離、PostgreSQL統合テスト、Agent submodule登録等を初回収録 |
| [32d3056 — harnessを拡充](https://github.com/shuichiro-hayafuji/arran/commit/32d305666d0a5612c85da3f7aab064eeab0441f4) | ハーネスのルール・検証入口を整備。CIや実機検証の完了を意味しない |
| [ccdc84e — 不要なproviderをrepository/viewModelに集約](https://github.com/shuichiro-hayafuji/arran/commit/ccdc84ef98b6102dd497cbc75f6dcd1990610b4e) | DIとViewModel公開Providerを集約。相談履歴の状態管理と古い応答・破棄後応答の回帰テストを追加 |
| [e4f7d03 — sessionとloginを分離する](https://github.com/shuichiro-hayafuji/arran/commit/e4f7d03e13659f77e0f12c5d4377b38591745a48) | loginのRepository/DTO/MVIを分離し、SessionのJSONをDTOへ移動。login/sessionの回帰テストを追加・更新 |

## 既存12件の判定・追記原稿

以下は各Issueへ追記する内容。既存タスクの削除や、未検証項目への完了チェックは行わない。

### [#1 本番向けビルド・環境設定](https://github.com/shuichiro-hayafuji/arran/issues/1)

Open継続。現HEADでも`mobile/lib/config/env.dart`の既定値はHTTP localhost、`mobile/android/app/build.gradle.kts`のrelease署名はdebug設定。productionでlocalhost・HTTP・debug署名を排除する完了条件は未達。配布用ビルド・実機検証も今回行っていない。

### [#2 プライバシー・法務・データ管理](https://github.com/shuichiro-hayafuji/arran/issues/2)

Open継続。現設定画面はプロフィール・メモリ・CSV・ログアウトが中心。全データ削除、エクスポート、保持期間・AI送信説明を設定から確認・操作できる完了条件は満たしていない。メモリ個別削除を全データ削除の完了とは扱わない。

### [#3 共通UX・アクセシビリティ・アプリ情報](https://github.com/shuichiro-hayafuji/arran/issues/3)

Open継続。`ada3b97`収録の`main.dart`に共通例外ハンドラの登録はあるが、`global_exception_handler.dart`の処理は実質未実装の分岐があり、FlutterError経路でrootContextを強制アンラップしている。未捕捉例外処理全体の完成とは判定しない。ローカライズ設定・ダークテーマの接続・アクセシビリティの検証も未完了。

残作業へ追加する原稿:

- [ ] 共通例外処理でNavigator未生成時のnullを安全に扱い、表示・記録する責務を明示する。
- [ ] GAP-M02の色・文字・余白の直接指定を対象画面の改修時にデザイントークンへ寄せ、文字拡大・狭い画面で確認する。

### [#4 日時契約とテスト・CI](https://github.com/shuichiro-hayafuji/arran/issues/4)

Open継続。一部実装済み: `ada3b97`にserverの固定UTC+9月次処理やsessionテストがあり、`ccdc84e`で相談ViewModelの履歴・応答順序・破棄後応答テスト、`e4f7d03`でloginテストが追加された。「Flutterのwidget/DTO以外のテスト」は存在する段階まで進んでいる。ただし今回FlutterテストはSDKキャッシュの権限制約により実行できず、主要フローのCI再現性まで確認できていない。共通の画面表示用JST変換は`formatters.dart`に存在せず、CI・実機・専用PostgreSQL検証は残る。

### [#5 CSV取込・データ移行・分析精度](https://github.com/shuichiro-hayafuji/arran/issues/5)

Open継続。`ada3b97`収録の`importer_test.go`にはUTF-8と生成したShift_JISデータの解析テストがある。一方、指定された銀行列名の拡充や実銀行CP932 fixtureでの検証はそれだけでは完了とならない。CSVプレビューはapplicationのプロセス内mapに保持され、再起動で失われる制約も残る。下記N1はmobileで旧プレビューを誤確定し得る問題であり、本Issueの銀行CSV互換性とは分けて管理する。

### [#6 LLM運用・プロバイダー評価・メモリ承認](https://github.com/shuichiro-hayafuji/arran/issues/6)

Open継続。Go側の決定論的判断、Agent境界、検証・fallbackの既存実装は維持されている。直前のチェックではAgentのvet/testが成功したが、fallback率・費用等の計測、実プロバイダー比較、承認フロー改善の完了証拠ではない。50〜100件での実測比較を実施済みとは判定しない。

### [#7 API認証・通信・運用保護](https://github.com/shuichiro-hayafuji/arran/issues/7)

Open継続。`ada3b97`でBearerセッション認証、所有者条件、ログイン試行制限と認証リクエストのサイズ制限、`auth_integration_test.go`を収録済み。`e4f7d03`ではmobile側のlogin/session境界を整理した。認証・認可の実装とデータ分離方針はコード・ハーネス上で確認できるが、PostgreSQLテストの存在と実DB検証成功は別。認証経路の制限を全APIの利用量制限とみなさず、TLS・監視・復旧・公開構成の検証を残す。

### [#8 PostgreSQLとAWS配備E2E](https://github.com/shuichiro-hayafuji/arran/issues/8)

Open継続。PostgreSQL・migration・インフラコードの存在だけでは、Docker・RDS・ALB・HTTPS・AWS deployの成功を証明できない。今回これらの環境を起動・変更しておらず、実環境の完了証拠なし。

### [#9 LAN・Tailscale・実機接続](https://github.com/shuichiro-hayafuji/arran/issues/9)

Open継続。今回Tailnet認証、VPN経由のiPhone/Android疎通、iOS署名・実機E2Eを検証していない。接続設定の存在だけでは「新しい端末でも文書だけで接続できる」完了条件を満たしたと判断できない。

### [#10 Agent submoduleとリモート運用](https://github.com/shuichiro-hayafuji/arran/issues/10)

Open継続。一部実装済み: `ada3b97`の`.gitmodules`に`server/agent`と`arran_agent.git`を登録済み。親HEADはmode 160000でAgentの`ef16c219e76ffb037387e7704982e4d2038312c6`を参照し、親originは`https://github.com/shuichiro-hayafuji/arran`。これにより親remote確認・submodule登録・親gitlink記録は確認できる。直前のAgent vet/testは成功したが、親testはキャッシュ権限で失敗。新規recursive cloneと変更→push→親gitlink更新の再現検証は未実行のため閉じない。

### [#11 1週間の価値検証](https://github.com/shuichiro-hayafuji/arran/issues/11)

Open継続。実測の相談起動回数・減額率・翌日満足度等と継続判断を今回取得できていない。コード整備や自動テストで代替できない完了条件であり、実測記録が必要。

### [#12 MVP対象外機能の優先順位](https://github.com/shuichiro-hayafuji/arran/issues/12)

Open継続。全候補の価値・リスク・依存・コスト比較と着手／保留／却下の意思決定は未確認。ただし「複数ユーザー対応」は完全な未着手候補ではない。`ada3b97`でAPI認証・所有者分離を収録し、`e4f7d03`でmobileのlogin/sessionを整理済み。候補の注記を「認証・所有者分離は実装済み。実DB検証、運用面の拡張は#7で管理」と更新し、機能全体の本番対応済みとはしない。

## 新規Issue原稿

N1〜N7は未採番のローカル識別子。各Issueに「確認基準HEAD: e4f7d03」「未修正・静的確認」「既存#4と連携し必要な回帰テストを追加」を記載して登録する。

### N1: [Mobile][P1] CSV再選択・再解析後に古いプレビューを確定できないようにする

**現状と影響**: `mobile/lib/features/transactions/screens/import/import_view_model.dart`の`selectAndPreview`はpreview/commitResultを消さない。Aをプレビュー後にBを選んでBの解析が失敗すると、Bのファイル名にAのプレビューが残り、Aを確定できる経路がある。過去の保存成功表示も残る。非同期応答の世代管理もない。

**完了条件**:

- [ ] ファイル選択・mapping変更時に旧preview/commitResultを無効化する。
- [ ] resetや新しい要求の開始後に古い応答で状態を復元・上書きしない。
- [ ] 確定できるのは現在の入力に対応する有効なプレビューだけにする。
- [ ] 確定成功後は同じpreviewの確定操作を終了し、別ファイルへ成功表示を持ち越さない。
- [ ] A成功→B失敗、応答順逆転、reset中の応答、成功後の再選択をfake RepositoryとWidgetテストで確認する。

**関連**: #5のCSV改善、#4の回帰テスト。新しいキャッシュ基盤の導入は本Issueの前提にしない。

### N2: [Mobile][P1] プロフィール保存・CSV確定の二重実行をViewModelで防止する

**現状と影響**: `profile_view_model.dart`の`_save`と`import_view_model.dart`の`commit`に進行中ガードがない。UIの無効化だけに依存し、同じIntentの重複dispatchで複数要求が発生する。CSVでは成功と期限切れエラーが競合し得る。DBの重複排除があっても操作結果の整合性は別問題。

**完了条件**:

- [ ] 保存・確定中の同一操作をViewModelで抑止する。
- [ ] エラー後に再試行でき、入力と直前の正常データを保持する。
- [ ] 同時dispatchでRepository呼出しが1回であることを確認する。
- [ ] CSV確定のDB失敗時は、serverがプレビューを消費する現行仕様に合わせ再プレビューへ案内する。

**関連**: N1、#4。サーバー側の冪等性・プレビュー消費仕様変更は別途検討する。

### N3: [Mobile][P1] 取引カテゴリ保存の失敗時にデータと入力を保持する

**現状と影響**: `transactions_view_model.dart`の`_updateCategory`失敗時に一覧のdata全体をAsyncErrorへ置き換える。詳細画面は同じdataを参照するため、保存失敗で取引内容とフォームが表示されなくなる。`transaction_detail_screen.dart`は保存状態をsetStateで管理し、例外をそのまま表示する。

**完了条件**:

- [ ] 保存中・保存失敗を読取dataから分離してMVIで管理する。
- [ ] 失敗時も取引内容・選択カテゴリ・加盟店適用設定を保持して再試行できる。
- [ ] 重複保存を防ぎ、安全な利用者向けエラーを表示する。
- [ ] 成功時に一覧・dashboardを更新し、画面遷移は一度だけ行う。
- [ ] 失敗→再試行→成功をViewModel/Widgetテストで検証する。

**関連**: GAP-M01、#3、#4。

### N4: [Server][P2] 業務エラーとHTTP応答の対応を型で定義する

**現状と影響**: `server/internal/handler/handler.go`の`isUserError`は日本語キーワードをerr.Error()から検索し400を選ぶ。文言変更でステータスが変わり、内部エラーが誤って入力エラー扱いになる可能性がある。

**完了条件**:

- [ ] application/domainの業務エラーをerrors.Is/As等で識別できる契約にする。
- [ ] handlerは型から既存HTTPステータス・エラーコードへ変換し、文言で分類しない。
- [ ] 未知の内部エラーは安全な500にし、内部情報を応答へ出さない。
- [ ] 文言変更・ラップされたエラー・内部エラーを回帰テストする。
- [ ] OpenAPIとの互換性を確認し、意図的変更が必要なら契約を同時更新する。

### N5: [Server][P2] Agentの生成とfallback構築をappへ集約する

**現状と影響**: `application/application.go`のNewでAgent MockClientとagentadapterを直接生成している。`app/app.go`にも構築経路があり、applicationの抽象境界に構築上の依存が残る（EX-S01）。

**完了条件**:

- [ ] Agent/adapter/fallbackの構築はappで行い、applicationへ抽象契約を注入する。
- [ ] 未指定依存の扱いを明示し、テスト側も必要なfakeを明示注入する。
- [ ] Goの金融判断、LLM送信DTO、fallback動作、API互換性を保つ。
- [ ] 親serverとAgentの検証を行い、EX-S01と関連設計文書を更新する。

### N6: [Server][P3] applicationをユースケース単位のファイルに整理する

**現状と影響**: `application/application.go`の835行にプロフィール、CSV、相談、レビュー、メモリ、構築・共通契約が集まり、変更箇所と責務を追いにくい。

**完了条件**:

- [ ] 同じapplication package内でユースケース単位にファイルを分ける。
- [ ] 共通のRepository/Agent契約、構築、時刻等の置き場を明示する。
- [ ] API・DB・金融判断の挙動を変えず既存回帰テストを通す。
- [ ] 機械的なinterface細分化や中継だけの層を増やさない。

**順序**: N4/N5の後を推奨。ファイル長だけを理由にサービスを分割しない。

### N7: [Quality][P2] 現行チェックの整形差分を解消し検証結果を確定する

**現状**: 直前の`python3 tools/harness.py check server`は`server/agent/agent.go`、`server/internal/agentadapter/adapter.go`、`server/internal/identity/context.go`の整形差分を検出。親serverのvet/testとmobileのformat/analyze/testはキャッシュ権限制約で成功を確認できていない。これはコード不具合と環境制約を分けて扱う。

**完了条件**:

- [ ] 変更を伴うtoolの手順に従って整形し、差分をレビューする。
- [ ] Agentへの変更は独立リポジトリ側で記録し、親gitlinkとの差を明示する。
- [ ] 許可された実行環境でmobile/server scopeを実行し、結果と基準コミットを記録する。
- [ ] PostgreSQL・実機・AWSの未検証を通常scopeの成功で完了扱いしない。

**関連**: #4のCI整備、#10のsubmodule運用。CI化自体は#4へ残す。

## 再開時の手順

1. 最新Issueとコメント、default branchのHEADを再取得し、本書との変更を確認する。
2. N1〜N7について同等のIssueが未作成なら登録し、対応する既存Issueから参照する。
3. 既存#1〜#12へ上記監査結果を反映する。#3に追加課題、#12に複数ユーザー機能の現状を追記する。
4. 新たに完了証拠を得たIssueだけ、元の問題・解決経緯・コミットURL・検証証拠をコメントし、completedで閉じる。残作業を削って見かけ上完了にしない。
5. GitHubを再取得し、作成・更新・Closeの実結果とURLを報告する。
