# CSV形式の対応範囲（Issue #5の限定工程）

今回の実装は列名・年先頭日付、合成CP932 bytesの回帰、手動マッピングUIの金額方式選択に限る。Issue #5全体の完了、代表的な実銀行エクスポートへの互換性は未確認。[合成fixtureの出自](../sample_data/README.md)、[同一性契約](csv-identity.md)、[取込履歴と原記録](csv-import-management.md)を参照。

## 確認した範囲

`csvimport`がローカルでdecode・列特定・正規化する。既存候補の優先順を維持し、`年月日`、`お取り扱い内容`、`お引出し`、`お預入れ`を追加した。残高を自動でamount候補にはしない。残高しかないCSVはNeedsMapping、正しい金額/入出金列があれば残高を金額へ使わずRawDataに保持する。

既存のISO・slash・dot・日本語年月日・月/日/年・英語月表記は維持する。年先頭のslash/dash/dotでは月日ゼロ埋めなしと混在にも対応し、ISO日付へ正規化する。新たな日/月推定や和暦・年の補完は追加しない。不正日付は既存の行Warningで示し、その行を正常な0円取引へ変えず、正常行を残す。

単一金額列は負数がincome、入出金別列は従来のtypeと絶対額を保持する。Goによる新しい金融計算・現在残高計算は追加しない。136 bytesの合成CP932 fixtureから入出金・拡張文字・残高を確認し、UTF-8対照と同じdecoded recordsのfile key/fingerprintになることも検証する。既存identity/alias、原記録保持、外部モデルtriplet3項目の境界は変更しない。API、DB、Agent、依存・SDKは変更しない。mobileは送信前の列選択だけを変更し、金額の解釈はサーバーに維持する。

## 読取調査と未完了

| 項目 | 現状 | 今回の扱い |
| --- | --- | --- |
| 代表的な実銀行CSVのfixture | authenticな出自証拠なし。既存sampleも証明されていない | 未完了。出自・使用許可・匿名化方針と対応形式の証拠が必要 |
| 複数行明細・銀行独自形式 | encoding/csvの通常recordを読む。引用符内改行と、別recordに分かれる銀行の継続明細は同じものではない | 独自ヘッダー/継続行/和暦等の形式別方針は未決、実装しない |
| 両入出金が非0の行 | 現行normalizerはdebit優先。単一金額列があればそちらを使う | 意味変更はしない。銀行形式の証拠と曖昧入力方針の検討が必要 |
| 手動マッピングUI | 「金額1列」または「出金・入金の別列」を選び、その方式だけを送信する。残高ではなく取引額の列を選ぶ案内と、無効な旧列名の除外を追加。明細ID列は独立して任意指定 | 限定改善。必要列がなければプレビュー不可。実機での操作確認は未実施 |
| プレビューの再起動耐性 | 所有者付きプロセス内cache、30分。再起動で未確定previewは失われる。確定結果のDB照会とは別 | 方式検討は未完了、既存B-S05を維持 |
| 旧SQLite→PostgreSQL | SQLiteファイルを自動コピーしない。別途一回限り移行が必要とREADMEに記載 | 具体的移行/所有者/照合/rollback手順は未完了。DB操作なし |
| subscriptionのカテゴリ分類 | ownerの加盟店規則とローカル語句でカテゴリ分類 | 意味変更なし。実データでの分類評価は未完了 |
| subscriptionの月次候補検出 | applicationのDetectSubscriptionCandidatesが同一加盟店・25〜35日間隔・同額支出を候補化。TestDetectSubscriptionCandidatesあり | 検出条件は変更しない。実データ評価・精度改善は未完了 |

制御回帰と静的検証は、実銀行・実機・DB移行・実LLM・AWSの動作確認を代替しない。列名が同じでも全銀行形式に対応するとは断定しない。

## 調査の根拠

- [CSV normalizer](../internal/csvimport/importer.go): columnCandidates/dateLayouts、ParseのCSV record・Warnings、normalizeRecordの金額列優先/debit優先、Categorizeのローカル語句。
- [手動マッピング画面](../../mobile/lib/features/transactions/screens/import/import_screen.dart): _MappingFormと_ColumnDropdownによる手動列指定。金額方式を明示選択し、選択した方式の有効な列だけを送信する。
- [月次サブスク候補検出](../internal/application/application.go)のDetectSubscriptionCandidatesと[既存回帰](../internal/application/application_test.go)のTestDetectSubscriptionCandidates: カテゴリ分類とは別の月次候補検出。今回未変更。
- [プレビュー保存](../internal/application/application.go): previews map/cachedPreviewと30分期限。未確定preview永続化は未実装。
- [server README](../README.md)のSQLite開発データ移行記述と[DB資料](database-schema.md): 自動移行・具体移行手順の完了証拠はない。
- [fixture出自](../sample_data/README.md): 新規合成fixtureの出自・hash・生成手順。authentic銀行fixtureは未提供。

CP932回帰の初回は髙のbyteをFBFCと想定した検証側の誤りでFAILした。Python標準cp932の実encodeを確認しEEE0への正しいbyte断言に直した。製品decoderは変更せず、UTF-8対照への独立decode一致と入出金・原文字・残高・identity一致を残している。
