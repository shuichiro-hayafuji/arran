# CSV取込履歴・訂正・取消

Issue #34の契約。取込全体の取消は銀行観測の採否であり、本人の購入取消・未購入・返金ではない。[同一性](csv-identity.md)、[購入との照合](purchase-csv-reconciliation.md)、[相談の受付と期限](consultation-requests.md)を維持する。

## 保存と有効な表示

各確定は`csv_import_batches`と全行の`csv_import_evidence`を同じトランザクションに保存する。全件重複の取込も独立した証拠を持つ。owner・物理行・方向・元観測・canonical transaction IDを保持し、CSV aliases/file rowsと相談状態版も原子的に更新する。同じpreview IDの結果はDBへ保存し、通信切断後・プロセス再起動後の再送でも元の結果を返す。

取消はそのbatchだけをinactiveにする。他のactive evidenceまたは未特定の旧証拠があれば明細は有効なまま。銀行原行、RawData、fingerprint/provider identity、aliases/file rowsは削除しない。金額・日付・カテゴリの訂正はeffective列とrevisionを更新し、原銀行内容を保持する。旧CSVの再取込で訂正値を上書きしない。取消した同じファイルを再確定すると、新batch evidenceで同じ明細IDを復活できる。previewの`reactivation_ids`に対する明示確認が必要で、結果の`reactivated_count`で追加とは区別する。

`effective_transactions`は有効性と最新訂正の読取投影。取引一覧・個別明細・旧MonthlyDashboard・相談triplet・照合候補・統合件数に使う。銀行identity/snapshot検索は原行を使う。旧月次集計のGo計算は継続するが、チャットの残高・自由予算の計算をGoへ追加しない。sameの最新購入実績と訂正履歴は銀行取消・訂正では変更しない。未購入へ自動変更せず、差額Expenseも作らない。

移行はPostgreSQL014 / SQLite008。旧file-rowのowner/file/scopeが証明できる単位だけを履歴化し、取込日時は不明として空欄を保持する。空key・不明単位を時刻で推測して束ねない。元行を`legacy_retained`で保護し、取引詳細から個別訂正できる。元CSVバイト列の保存・復元・download APIは提供しない。保存されるのは既存原行・RawData・全行evidence・必要なfilename/encoding/mappingと監査である。

## 操作と結果照会

すべて既存認証のowner scope。別ownerの対象は404。操作IDはownerごとの論理IDで、同ID同payloadは保存済結果を返し、異payloadは409。最新の対象版・状態版と、batch操作ではdetail snapshotを必要とする。古いモデル結果・reminder・同時操作と競合すると409で拒否し、状態を読み直す。

| API | 内容 |
| --- | --- |
| `GET /imports?before=<id>` | 最新日時/id順で最大20件。日時不明の移行履歴も区別。行・取消影響・snapshotはdetailから取得 |
| `GET /imports/{id}` | 全行の原観測・現在投影、有効性、取消影響/保護件数、版とsnapshot |
| `POST /imports/{id}/cancel-preview` | detailと同じ確認情報。保存しない |
| `POST /imports/{id}/cancel` / `restore` | 明示したbatchの取消・復活。原子保存結果を返す |
| `GET /transactions/{id}` | 原銀行内容・現在投影・有効性・状態版/行revision |
| `PATCH /transactions/{id}` | `mutation_id/expected_version/state_version`付き金額・日付・カテゴリ訂正。旧カテゴリのみリクエストも互換維持 |
| `GET /mutations/{id}` | 保存結果照会。未保存は404 |
| `GET /changes/{target}` | ownerのimmutable変更履歴 |

mobileは変更前に対象と影響を示し、取消/復活/訂正の保存を明示確認する。結果不明なら入力を固定し、同じIDの結果照会→未保存404なら同payload再送に限定する。確定した400/404/409拒否は編集を再開できる。破棄・セッション変更後の応答は公開しない。履歴は銀行操作の保存を示し、金融反映完了を先走らない。

## 金融保留とモデル送信

銀行操作コマンドは現在残高・自由予算Factを加減算しない。変更した有効投影の前後を方向別の`csv_financial_receipts`へ保存し、次の通常チャットで`DataChangeRepository`から限定`data_change_context`を読む。新しいworker/queueやledgerは作らず、既存28秒の論理受付・保存/取消fenceに接続する。

許可DTOはopaque ref、operation（`correct_expense/cancel_expense/restore_expense`とincome版）、前後のtriplet（日付・整数円・カテゴリ）、included/omitted件数、両axis解決状態、関連する本人purchase IDsだけ。原銀行ID・摘要・口座・source・fingerprint・RawData・file keyは出さず、Purchase/Fact経由のコピーも禁止。receipt対象行はowner/ref付きローカルFKで対応し、保留中は通常CSV tripletから除外して別支出に見せない。解決後は通常投影に戻るが、更新済みFactを再計上しないようモデルへ指示する。

最大20操作、各前後20tripletまで保存用contextへ転記する。送信時は全操作を合わせ100tripletまでとし、通常取引入力と合わせても100件以内。カテゴリは通常取引と同じ除去処理と100文字上限。省略数は明示し、partialを全件反映済みと推測しない。忘却より前の取込/購入/操作を外部に復活させない。銀行原記録とCSV再取込のidentityは保持する。

LLM再計算のchangesは`basis=reconciliation`、本人による新しい基準は`basis=user_report`。bank-onlyに`basis=purchase`を使わない。包含が不明なら質問し、金融値を変えず`needs_information`を保存する。`applied`は完全coverageと未解決axisの明示setが必要で、両axis未解決なら同値でも両setを返す。部分coverageでは推測appliedを拒否する。本人が変更後の現在残高と自由予算の両方を明示した場合に限り、両setの`basis=user_report`かつ`kind=fact`と専用`baseline_adopted`で基準を採用できる。片側だけ・推定の基準採用は不可で、送っていない別refを完了させない。期間は空またはJSTの現在月に限定し、現在残高はJSTの現在日も許可する。RecordedAtが新しくても過去期間を現在基準として採用しない。ServiceはInput.Now、両DBの保存境界は保存時server clockで同じ構造判定を行う。月境界をまたいだ古い月の採用は安全側に拒否し、金融値・receipt・監査を原子的に保持する。

LLM失敗・修正上限・期限では旧金融Factとpendingを保持する。金融更新、receipt状態/axis、モードを残す監査、相談結果・論理受付結果は同じSaveChatTurnのtxで確定する。銀行記録の保存成功と金融完了は区別する。

## 検証範囲

両DBの制御回帰は、全重複evidenceの保護、混在入出金、最新訂正/再取込/同ID復活、owner/版fence、原記録/監査、旧known/unknown移行・遅いDDL失敗rollback・並行適用・再起動、同一購入取消保護、pending通常triplet除外・解決後保持、片axis拒否、partial/forgetと両本人基準採用を確認する。HTTPとmobile DTO/MVI/widgetで確認操作・結果照会・拒否/破棄境界を確認する。制御回帰は実LLMの計算品質や実機表示の保証ではない。

2026-10-12の限定実LLM検証では、銀行20,000円の訂正・取消・復元・次相談の5turnを確認した。本人基準の種別・期間を厳格化した後は、その5応答の再生を確認し、21件の部分入力に対する本人の最新基準採用を別の実LLM1turnで確認した。原明細・監査の保持と応答/再取得の一致を確認した範囲であり、実銀行ファイル・実機・本番DBの動作確認ではない。先行の根拠ラベル不正と修正期限切れも[改善記録](../../docs/harness/feedback/FB-20261012-09.md)に保持する。
