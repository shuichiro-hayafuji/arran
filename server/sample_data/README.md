# CSV fixtureの出自

`synthetic_bank_cp932.csv`はIssue #5の限定回帰用に作成した合成データ。実銀行のエクスポートではなく、口座・取引・人物の実情報を含まない。代表的な実銀行CSVの検証完了を意味しない。

- 対照: [同じ合成内容のUTF-8ファイル](synthetic_bank_cp932_source_utf8.csv)
- CP932ファイル: 136 bytes、SHA-256 `3183ef2cf427ec3ee43005e9b18de6a8af1656d1a547bcb10d23b401a6a117cc`
- 内容: 年月日・お取り扱い内容・お引出し・お預入れ・残高。出金1,200円、入金3,500円の2行と、CP932拡張文字`①`（8740）、`髙`（EEE0）、`㈱`（878A）を含む。残高は取引額とは別の観測として保持する。
- 生成日: 2026-10-12。ソースはこの作業で作成し、Python標準`cp932` codecで変換した。回帰はコミットされたbytesを直接読み、実行時にGo encoderから作り直さない。

再現コマンド（リポジトリルート）:

```sh
python3 - <<'PY'
from pathlib import Path
base = Path('server/sample_data')
source = (base / 'synthetic_bank_cp932_source_utf8.csv').read_bytes().decode('utf-8')
(base / 'synthetic_bank_cp932.csv').write_bytes(source.encode('cp932'))
PY
```

既存の他のsample CSVについてもauthenticな銀行出力であることを裏付ける出自資料は、このリポジトリでは確認できていない。ファイル名や文字コードから実銀行検証済みと判断しない。[対応範囲と未完了事項](../docs/csv-format-coverage.md)を参照。
