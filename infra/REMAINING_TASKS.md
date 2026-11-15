# 本番公開までのチェックリスト

構成: 東京Lightsail 2GB単一ホスト。手順は [運用手順](operations/README.md)。コードがあるだけの項目を実配備済みとしてチェックしません。

- [ ] 所有ドメイン決定・DNS管理権確認
- [ ] AWSログインと対象アカウント/東京region確認
- [ ] bundle/blueprint/鍵名、旧リソース残存費用確認
- [ ] infra/docs通常検証、Compose契約検証、ローカルDBロール/移行回帰
- [ ] CDK diff確認、productionスタック配備・保持/公開範囲確認
- [ ] static IPへのDNS A設定、SSH管理CIDR確認
- [ ] OS更新、Docker/AWS CLI、サーバー同一イメージ搬入
- [ ] ホスト秘密設定、backup専用鍵設定、初回migrateとAPI起動
- [ ] HTTPS、DB/API port非公開、認証/認可、主要操作/利用制限確認
- [ ] 日次snapshot、S3 backup成功、別DBへのrestoreと所要時間確認
- [ ] timer有効化、失敗/最終成功監視、OS保守担当の確認
- [ ] AWSコスト通知設定・PagerDuty連携（後続Issue）
- [ ] 実機API_BASE_URL設定、公開判断、旧構成の廃止要否確認
