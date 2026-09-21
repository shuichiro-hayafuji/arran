# モバイル実装時の作法

- C-M01: 操作はIntentとViewModelの処理にまとめ、表示はStateから作る。通常機能に別の状態管理方式を混在させない。
- C-M02: 非同期処理後は破棄・セッション切替を考慮する。ロード・失敗状態を握り潰さず、共通エラー処理を使う。
- C-M03: API項目はDTOで型・null・欠損を扱い、Domainへの変換を明示する。API変更時は [OpenAPI](../../server/docs/openapi.yaml) とサーバー実装も確認する。
- C-M04: Freezed生成物を直接編集しない。元のState・Intent等を変更してtoolの生成手順を使い、生成差分を確認する。
- C-M05: `mobile/packages/` の同梱コードはアプリ実装と区別する。依存元と実際の採用状況を確認してから変更し、アプリ全体の整形に巻き込まない。変更時にはそのパッケージを明示して検証する。
- C-M06: 接続先は既存Env設定から取得し、資格情報をコードやログへ書かない。セッション永続化は既存Secure Storage経路を使う。
- C-M07: Flutter/Dartの実行はFVM。整形・解析・テスト・生成の具体的手順は [tool](../../tools/INDEX.md) に集約する。

## C-M01の詳細: 操作と状態

- 業務操作はIntent → dispatch → Repository → Stateに統一する。ScreenからViewModelの内部メソッドを直接呼ばない。
- Stateは不変としcopyWith等で更新する。複合画面では取得状態と保存状態を分け、保存失敗で入力済みデータを失わない。意味の重なるbooleanを増やさず、複雑な段階は型で表す。
- WidgetはTextEditingController・FocusNode・アニメーション・送信前の局所入力を所有してよい。API結果・保存状態の正本をsetStateに二重保持しない。画面間で共有する入力や非同期検証対象はViewModelのStateへ置く。
- ViewModelは業務の表示用調停を担う。依存方向・Providerの選択は [アーキテクチャ](architecture.md) に従う。

## C-M02の詳細: 非同期・副作用・寿命

- await後はmountedを確認する。同じViewModelで検索条件・対象が変わる場合はrequest IDや取消し等で古い応答の上書きも防ぐ。mountedだけでは応答順序を保証しない。
- 保存・確定はUI無効化に加えてViewModelで進行中の重複実行を防ぐ。読取の再試行と更新の再送を区別する。
- 遷移・SnackbarはScreenのlisten等で状態変化を検出して一度だけ実行する。buildごとや再入場時の古いsaved状態で再発火させない。
- 初回取得はProvider/ViewModel生成か明示Intentに揃え、buildのたびにロードしない。入力controllerへの初期値反映で編集途中の値を上書きしない。
- controller・focus・購読は所有する側でdisposeする。Providerの寿命は再入場とセッション切替を考慮して選ぶ。
- 保存成功と後続の一覧再取得失敗を混同しない。未awaitのFutureを放置せず、意図的な非同期化にもエラー処理を設ける。

## C-M03の詳細: 入出力・エラー

- 必須・整数・範囲等の入力検証は早期案内として行い、サーバーの検証・金融判断の代わりにしない。
- DTOでAPI契約を吸収し、Screen/ViewModelへJSONキーやResponseを漏らさない。
- 共通例外変換を使い、エラーを握り潰さない。利用者へ生の例外・stack trace・tokenを表示せず、操作可能なメッセージにする。
- 読取のloading/error/empty/dataと保存中・保存失敗を区別し、失敗から再試行できる経路を作る。

## C-M08: 画面デザイン

画面・Widget・テーマ変更時は [デザインシステム](design-system.md) を必ず読み、token、共通Widget、状態別表示、狭い幅・文字拡大・キーボードを確認する。依頼なしにナビゲーション体系や視覚スタイルを変更しない。

## C-M09: 実装前の判断と完了条件

実装前にfeature、Domainの所有者と粒度、DTO/Repository操作、State/Intent、Providerの種類と寿命、利用するtokenを決める。既存で足りる要素は再利用し、空の層・中継だけのクラス・exportだけのViewModelを作らない。

| 変更 | 確認する振る舞い |
| --- | --- |
| DTO/Repository | null・欠損・0円、送信項目、Domain変換、fake HTTPでの契約 |
| ViewModel | fake RepositoryでIntent→State、保存失敗時の入力保持、二重操作、必要時の古い応答破棄 |
| Provider/認証 | familyの対象分離、破棄後の更新防止、ユーザー切替時の状態破棄 |
| Screen | 状態別表示、Intent送信、一回限りの副作用、入力保持 |
| デザイン | token使用、幅・文字拡大・長文・キーボード、操作可能性 |

変更に必要なテストを選び、実装を写しただけのテストは追加しない。完了時は層の漏れ、循環依存、正本の二重化、非同期とエラー、token使用を差分で確認する。例外は対象と理由を [WORKFLOW](../harness/WORKFLOW.md) に残す。未実行の画面確認を完了としない。
