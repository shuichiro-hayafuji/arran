# 東京の本番環境

月5,000円程度を目安に、Lightsail 2GBの単一ホストへ既存の `server/Dockerfile` から作るAPIとPostgreSQL 16を配置します。検証用AWS環境は作りません。標準CDKは旧ECS・ALB・RDS・NAT構成を作成しません。既存AWSリソースを自動削除する移行ではありません。旧スタックが残っている場合は、データと請求を確認して別途廃止します。

```text
Internet → static IPv4 → Lightsail firewall (80/443, SSHは管理CIDRのみ)
                         Caddy :443 (公開DNS名・自動HTTPS)
                           → API :8080 (ホスト非公開、通常DBロール)
                             → PostgreSQL 16 :5432 (内部networkのみ)
                           → OpenAI (外向き)
                         one-off migrate (移行DBロール)
                         daily pg_dump → private S3 (7日で期限切れ)
                         daily Lightsail snapshot
```

実装と運用の正本は [運用手順](operations/README.md)、配備前のチェックリストは [残課題](REMAINING_TASKS.md) です。API認証・利用上限は既存サーバーに従います。HTTPSだけでAPIの認可や利用料金が保証されるわけではありません。

初回の権限設定は [限定CDK bootstrap](bootstrap/README.md) を参照してください。管理者操作と通常配備の資格情報を分離します。

## 検証

```bash
python3 tools/harness.py check infra
python3 tools/harness.py check docs
python3 infra/operations/verify-compose.py
cd infra
npx cdk synth -c domainName=YOUR_OWNED_DNS_NAME -c adminCidr=YOUR_PUBLIC_IP/32 -c keyPairName=YOUR_LIGHTSAIL_KEY
```

最初の2つは通常チェック、Compose契約確認はDocker Compose CLIが必要な読み取り検証です。synthは配備ではありません。CDKはドメイン・管理CIDR・既存SSH鍵名の省略を拒否し、東京以外のregion指定を拒否します。Docker実起動・DBのロール権限・AWS接続・TLS・バックアップ復元・実負荷は別途確認します。

## 費用と容量

IPv4 `small_3_0` は2GB RAM、2vCPU、60GB SSD、転送枠3TB、月額$12のプランです。東京での提供状態・価格は配備前に `aws lightsail get-bundles --region ap-northeast-1` で再確認します。CPUはバースト型で、2vCPUを継続して占有できる保証はありません。

バックアップは別料金です。S3のDB backupは日次取得・現行版7日で期限切れ、旧版1日で削除対象とし、実保存量・リクエスト・非同期削除までの時間に応じて課金されます。スナップショットは保存量に対し$0.05/GB・月。例としてホスト$12にスナップショット60GB・月相当$3を加え、1ドル150円・税10%なら約2,475円です。差分の増加で保存量は変動します。この例にS3の保存・API要求・復元時転送、ドメイン代、転送超過、OpenAI料金は含みません。自動スナップショットは無料ではありません。停止中もホスト料金が発生し、保持したリソースはスタック削除後も請求されます。

これは5,000円の強制上限ではありません。AWS Budgetsの通知は後続手順で設定し、毎月の請求と日々のバックアップ量を確認します。大きいCSVや同時相談の上限は既存の利用制限を使い、実負荷試験後に運用人数を決めます。2GB上でAPI512MB・DB768MB・Caddy128MBに制限し、残りをOSとbackupへ確保しますが、バックアップや更新時のピーク/OOMは未保証です。増強は自動化せず予算を再確認します。

単一ホスト故障では全体が停止します。毎日の成功したバックアップがある場合の目標RPOは24時間、RTOは復旧訓練で測定します。自動フェイルオーバー・ゼロダウンタイム更新は提供しません。

公式資料: [料金](https://aws.amazon.com/lightsail/pricing/)、[bundle API](https://docs.aws.amazon.com/cli/latest/reference/lightsail/get-bundles.html)、[CloudFormation Instance](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-lightsail-instance.html)。
