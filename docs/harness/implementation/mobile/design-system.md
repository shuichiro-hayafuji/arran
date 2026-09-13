# モバイルのデザインシステムと画面設計

実装ルールとして、画面・Widget・テーマの変更時に必ず読む。上位判断は [DEC-M05](architecture.md)、利用者の操作結果は [モバイル業務](../../business/mobile.md) を参照する。

## D-M01: constantsを正本とする

| 要素 | 正本 | 利用方法 |
| --- | --- | --- |
| 原始色 | [app_color_palette.dart](../../../../mobile/lib/app/constants/app_color_palette.dart) | 意味色を定義する素材。画面で直接色番号を選ばない |
| 意味色 | [color_theme.dart](../../../../mobile/lib/app/constants/color_theme.dart) | AppColors.standardのbrand/background/border/text等を意味で選ぶ |
| 文字 | [text_theme.dart](../../../../mobile/lib/app/constants/text_theme.dart) | AppTextStyles.of(context)またはThemeのtextThemeから役割で選ぶ |
| 余白 | [space_theme.dart](../../../../mobile/lib/app/constants/space_theme.dart) | AppSpaceのpadding・縦横の間隔を使う |
| 全体テーマ | [app_theme.dart](../../../../mobile/lib/app/constants/app_theme.dart) | AppTheme.standard、共通component themeを利用する |

新規・変更する表示にColorリテラル、直接のColors指定、任意のfontSize、同じ目的の余白数値を散在させない。標準Material部品は既存Themeを活用する。paletteは意味色へ割り当て、画面から直接参照する新規コードを避ける。

不足する意味色・余白・角丸・寸法は既存テーマで表現できるかを先に確認する。再利用する設計値ならconstantsへ意味と用途を定義する。画面内に別のテーマ定数群を作らない。存在しないerror/success色のアクセサー等を使用しない。計算上の0やレイアウト比率まで無意味にtoken化する必要はない。

## D-M02: Widgetとの境界

- constantsは値・意味・Themeを持つ。業務処理、API、画面Widgetを置かない。
- エラー表示・見出し等の汎用部品はshared/widgetsを再利用する。新しい共通部品は特定Domain・Providerへ依存せず、表示値とcallbackを受け取る。
- 画面固有部品はscreen配下、feature内だけの共通部品はfeature内。再利用予定だけでsharedへ昇格させない。
- 金額・日付表示は既存shared/utils/formatters.dartを使い、Widgetで支出判定を再計算しない。

## D-M03: 画面構成

- 既存NavigationShell・go_router・Material 3を使う。依頼なしに遷移体系や視覚スタイルを入れ替えない。
- 主情報、入力・詳細、主操作の階層を明確にし、見出し・本文・補助情報へ対応する文字スタイルを使う。
- loading/empty/error/successと保存中を設計する。操作無効化、入力保持、再試行を用意し、色だけでエラーを伝えない。
- 画面表示・Provider再評価で、利用者が実行していない保存・確定を発火させない。

## D-M04: レイアウトとアクセシビリティ

- 実行幅に応じて配置し、画面全体を固定幅・固定高さにしない。SafeArea、スクロール、キーボード表示を考慮する。
- 長い金額・説明・空文字・複数行・文字拡大で確認する。文字拡大を無効化してoverflowを隠さず、重要な金額や判定を無説明に切り捨てない。
- 標準Material部品のタップ領域を維持する。アイコンのみの操作にはTooltip/Semantics等で名称を付ける。
- 安全/注意/エラーは文言やアイコンも併用する。テーマ変更時はコントラストと可読性を確認する。

## D-M05: 適用と確認

使用tokenと共通Widgetを説明でき、対象の状態・幅・文字サイズを確認する。golden/widgetテストは有効な範囲で用い、目視していない状態は未確認と記す。

既存には直接指定の色・余白が残る（GAP-M02）。新規実装へコピーせず、変更部分からtokenを適用する。無関係な全画面リデザインを同時に行わない。この規約の専用自動検出は未実装。
