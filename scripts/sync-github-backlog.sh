#!/usr/bin/env bash

set -euo pipefail

OWNER="shuichiro-hayafuji"
REPO="shuichiro-hayafuji/arran"
PROJECT_NUMBER="1"

titles=(
  "[Mobile] 本番向けビルド・環境設定を整備する"
  "[Mobile] プライバシー・法務・データ管理機能を整備する"
  "[Mobile] 共通UX・アクセシビリティ・アプリ情報を整備する"
  "[Quality] 日時契約とテスト・CIを強化する"
  "[Data] CSV取込・データ移行・分析精度を改善する"
  "[AI] LLM運用・プロバイダー評価・メモリ承認を強化する"
  "[Security] API認証・通信・運用保護を実装する"
  "[Infra] PostgreSQLとAWS配備をE2E検証する"
  "[Ops] LAN・Tailscale・実機接続を検証する"
  "[Repository] Agent submoduleとリモート運用を完成させる"
  "[Product] 1週間の価値検証を実施する"
  "[Future] MVP対象外機能の優先順位を決める"
)

bodies=(
  $'## 目的\n\nモバイルアプリを開発用MVPから安全な配布ビルドへ移行する。\n\n## タスク\n\n- [ ] dev / staging / productionを分離する\n- [ ] 本番ビルドでlocalhostへ接続できないようにする\n- [ ] API通信をHTTPSへ限定する\n- [ ] Androidのcleartext通信をdebug限定にする\n- [ ] AndroidのApplication IDを正式な値へ変更する\n- [ ] Android release署名とkeystoreの安全な注入を設定する\n- [ ] R8 / ProGuardを設定する\n- [ ] iOS / Android / Flutter / 画面内のアプリ名を統一する\n- [ ] 正式なアイコン・起動画面・ストア素材を整備する\n\n## 完了条件\n\n- production設定でlocalhost・HTTP・debug署名が使用されない\n- iOS / Androidのreleaseビルド手順が再現できる'
  $'## 目的\n\n金融情報を扱うアプリとして、利用者がデータの扱いを理解・制御できるようにする。\n\n## タスク\n\n- [ ] プライバシーポリシーを追加する\n- [ ] 利用規約を追加する\n- [ ] 全データ削除機能を追加する\n- [ ] データエクスポート機能を追加する\n- [ ] データ保持期間を定義・表示する\n- [ ] 外部AIへ送信する情報を説明する\n- [ ] アプリロック／生体認証を検討・実装する\n- [ ] ログから個人情報・金融情報を除外する\n\n## 完了条件\n\n- 設定画面からデータの利用、保持、削除、出力を確認・操作できる'
  $'## 目的\n\nアプリ全体の品質と一貫性を整える。\n\n## タスク\n\n- [ ] 未捕捉Flutter例外を共通処理する\n- [ ] 個人情報を含まないクラッシュレポートを導入する\n- [ ] supportedLocales / localizationsDelegatesを設定する\n- [ ] 画面文言をリソース化する\n- [ ] ダークモードへ対応する\n- [ ] アプリ状態を復元できるようにする\n- [ ] About、バージョン、ビルド番号を表示する\n- [ ] 文字サイズ・画面サイズ・VoiceOver/TalkBackを検証する\n\n## 完了条件\n\n- 共通エラー処理、テーマ、ローカライズ、アクセシビリティのテストがある'
  $'## 目的\n\n日時境界と主要ユーザーフローを自動テストし、継続的に品質を確認する。\n\n## タスク\n\n- [ ] 家計上の今日・今月をAsia/Tokyoで扱う\n- [ ] API保存・通信時刻をUTC RFC3339に統一する\n- [ ] 画面表示用の共通JST変換を実装する\n- [ ] 取引日をdate-onlyとして扱いUTC変換しない\n- [ ] Goの固定UTC+9をtime.LoadLocationへ変更するか判断する\n- [ ] 月末・年末・UTC/JST境界テストを追加する\n- [ ] プロフィール保存をAPI・PostgreSQL・再読込までテストする\n- [ ] Flutterのwidget/DTO以外のテストを拡充する\n- [ ] Android/iOSの主要フローを検証する\n- [ ] format / analyze / test / vetをCI化する\n\n## 完了条件\n\n- 境界日時と主要フローがCIで再現可能に検証される'
  $'## 目的\n\n実在する銀行CSVの互換性と、保存データの継続性・分析精度を改善する。\n\n## タスク\n\n- [ ] 対応する列名・日付形式を拡充する\n- [ ] 年月日、お取り扱い内容、お引出し、お預入れを自動認識する\n- [ ] 残高列を取引金額として扱わないテストを追加する\n- [ ] CP932実ファイルで入出金符号を検証する\n- [ ] 複数行明細・銀行独自形式への対応方針を決める\n- [ ] 手動マッピングUIを改善する\n- [ ] CSVプレビューを再起動で失わない方式を検討する\n- [ ] 旧SQLiteデータからPostgreSQLへの移行手順を作る\n- [ ] サブスクリプション検出の精度を評価・改善する\n\n## 完了条件\n\n- 代表的な実銀行CSVのfixtureと回帰テストがある'
  $'## 目的\n\nGoの決定権と最小データ送信を維持しながら、AI機能を運用可能にする。\n\n## タスク\n\n- [ ] モデルが金額・カテゴリ・verdictを変更できない回帰テストを維持する\n- [ ] Structured Output検証とbounded revisionを監視する\n- [ ] fallback率・schema成功率・latency・costを計測する\n- [ ] トークン上限、利用量上限、レート制限を設定する\n- [ ] 重複リクエストを防止する\n- [ ] APIキーのローテーション手順を作る\n- [ ] AI送信データを監査する\n- [ ] provider / base URL / model / timeout / fallbackを明示設定にする\n- [ ] Bedrockのendpoint・schema互換性・retentionを検証する\n- [ ] 50〜100件の匿名化ケースで品質・速度・費用を比較する\n- [ ] メモリ候補の承認・却下フローを改善する\n\n## 完了条件\n\n- Goの決定論的境界を壊さず、品質・費用・データ送信を測定できる'
  $'## 目的\n\n信頼できるLAN内だけを前提としたAPIを、本番運用可能な境界へ移行する。\n\n## タスク\n\n- [ ] 認証・認可を実装する\n- [ ] TLS / HTTPSを有効にする\n- [ ] レート制限を実装する\n- [ ] リクエストサイズ・利用量制限を設定する\n- [ ] セキュリティ監視とアラームを設定する\n- [ ] 複数ユーザー化時のデータ分離方針を決める\n- [ ] バックアップ・復旧・削除方針を定義する\n- [ ] 認証・TLSなしのポートをインターネット公開しない構成を検証する\n\n## 完了条件\n\n- 認証、暗号化、制限、監視、データ分離の脅威モデルとテストがある'
  $'## 目的\n\nローカルPostgreSQLからAWSまで、同じserver実装を実環境で検証する。\n\n## タスク\n\n- [ ] Docker ComposeでPostgreSQLを起動する\n- [ ] Go APIの接続・migration・主要APIを確認する\n- [ ] CDKのnpm install / build / test / synthを実行する\n- [ ] serverのLinux/x86_64コンテナをビルドする\n- [ ] RDS接続環境でmigrationを確認する\n- [ ] ALBとコンテナのhealth checkを確認する\n- [ ] CDK bootstrap / deployを実行する\n- [ ] OpenAI Secretを必要時だけ登録する\n- [ ] ApiBaseUrlをFlutterへ設定する\n- [ ] HTTPS / ACMを設定する\n- [ ] RDS保持期間・削除保護・復旧訓練を整備する\n- [ ] Auto Scaling、監視、アラーム、CI/CDを整備する\n\n## 完了条件\n\n- ローカルとAWSでmigration・health・主要APIがE2E成功する'
  $'## 目的\n\nLAN、Tailnet、物理端末からの接続手順を再現可能にする。\n\n## タスク\n\n- [ ] tailscaledの起動状態を確認する\n- [ ] Tailnet認証を確認する\n- [ ] MagicDNS / Tailscale Serveを確認する\n- [ ] ローカルAPIの疎通を確認する\n- [ ] iPhone / AndroidからVPN経由で疎通確認する\n- [ ] Wi-Fi変更時のLAN IP更新手順を確認する\n- [ ] 物理端末用API_BASE_URLを確認する\n- [ ] iOSの信頼・Developer Mode・署名・API接続をE2E確認する\n\n## 完了条件\n\n- 新しい端末でもドキュメントだけで安全に接続できる'
  $'## 目的\n\n分離済みAgentをGitHub submoduleとして正式運用する。\n\n## タスク\n\n- [ ] arran_agentのリモート内容とローカル内容を照合する\n- [ ] Arran本体のremoteを確認する\n- [ ] server/agentをGit submoduleとして登録する\n- [ ] 親リポジトリへgitlinkを記録する\n- [ ] clone --recurse-submodulesを検証する\n- [ ] Agent変更→push→親gitlink更新の手順を検証する\n- [ ] 親とAgentのテストをsubmodule状態で実行する\n\n## 完了条件\n\n- 新規cloneから親・Agent双方のテストを再現できる'
  $'## 目的\n\nSpendable Todayが実際の支出判断を改善するか、1週間の利用で評価する。\n\n## 記録する指標\n\n- [ ] 相談起動回数\n- [ ] 助言後に見送った割合\n- [ ] 助言後に減額した割合\n- [ ] 翌日の満足度\n- [ ] 翌日の後悔度\n- [ ] 過去結果を参照した助言が役立った具体例\n\n## 検証シナリオ\n\n- [ ] 6,000円程度の飲み会\n- [ ] 20,000円の仕事兼用イヤホン\n- [ ] 1,490円の未利用サブスクリプション\n\n## 完了条件\n\n- 継続・改善・停止の判断と、その根拠となる実測値を記録する'
  $'## 目的\n\n現MVPで意図的に対象外とした機能を、価値検証後に改めて優先順位付けする。\n\n## 候補\n\n- [ ] 通知・リマインダー\n- [ ] 銀行API／カードAPIとの自動連携\n- [ ] 決済制御\n- [ ] ベクトルDB\n- [ ] 複数ユーザー対応\n- [ ] 本格的なクラウド運用\n- [ ] より厳密なメモリ承認管理\n\n## 完了条件\n\n- 各候補について価値、リスク、依存関係、実装コストを比較し、着手／保留／却下を決める'
)

if [[ ${#titles[@]} -ne ${#bodies[@]} ]]; then
  echo "internal error: title/body count mismatch" >&2
  exit 1
fi

gh auth status --hostname github.com >/dev/null
gh project view "$PROJECT_NUMBER" --owner "$OWNER" >/dev/null

project_id="$(gh project view "$PROJECT_NUMBER" --owner "$OWNER" --format json --jq '.id')"
status_field_id="$(gh project field-list "$PROJECT_NUMBER" --owner "$OWNER" --format json --jq '.fields[] | select(.name == "Status") | .id' | head -n 1)"
todo_option_id="$(gh project field-list "$PROJECT_NUMBER" --owner "$OWNER" --format json --jq '.fields[] | select(.name == "Status") | .options[]? | select(.name == "Todo") | .id' | head -n 1)"

for index in "${!titles[@]}"; do
  title="${titles[$index]}"
  body="${bodies[$index]}"

  issue_url="$(
    gh issue list --repo "$REPO" --state all --limit 500 \
      --json title,url --jq ".[] | select(.title == \"$title\") | .url" \
      | head -n 1
  )"

  if [[ -z "$issue_url" ]]; then
    echo "Creating: $title"
    issue_url="$(gh issue create --repo "$REPO" --title "$title" --body "$body")"
  else
    echo "Reusing:  $title"
  fi

  item_id="$(
    gh project item-list "$PROJECT_NUMBER" --owner "$OWNER" --limit 500 \
      --format json --jq ".items[] | select(.content.url == \"$issue_url\") | .id" \
      | head -n 1
  )"

  if [[ -z "$item_id" ]]; then
    echo "Adding to project: $issue_url"
    gh project item-add "$PROJECT_NUMBER" --owner "$OWNER" --url "$issue_url" >/dev/null
    item_id="$(
      gh project item-list "$PROJECT_NUMBER" --owner "$OWNER" --limit 500 \
        --format json --jq ".items[] | select(.content.url == \"$issue_url\") | .id" \
        | head -n 1
    )"
  fi

  if [[ -n "$item_id" && -n "$status_field_id" && -n "$todo_option_id" ]]; then
    gh project item-edit \
      --id "$item_id" \
      --project-id "$project_id" \
      --field-id "$status_field_id" \
      --single-select-option-id "$todo_option_id" >/dev/null
  fi
done

echo
echo "Backlog sync complete."
gh project item-list "$PROJECT_NUMBER" --owner "$OWNER" --limit 500 \
  --format json --jq '.items[] | [.status, .content.title, .content.url] | @tsv'
