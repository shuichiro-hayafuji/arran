# Spendable Today AWS infrastructure

`server/` を唯一のGo API実装としてAWSへ配備するためのCDK構成です。旧 `backend/` の最小ヘルスチェック実装は廃止しました。

## アーキテクチャ

```text
Internet → ALB:80 → ECS Fargate (private subnet, :8080)
                         ├─ RDS PostgreSQL 16 (private subnet, :5432)
                         └─ NAT Gateway → OpenAI Responses API / PagerDuty Events API
```

CDKの`DockerImageAsset`は`server/`のDockerfileをx86_64 Linux向けにビルドします。ローカルDocker Composeと同じPostgreSQL adapterを使い、ECSにはRDSの接続情報を環境変数とSecrets Manager経由で渡します。

## ディレクトリ

```text
infra/
  bin/app.ts
  lib/constructs/{network,database,container,alb}-construct.ts
  lib/stacks/spendable-today-stack.ts
  test/spendable-today-stack.test.ts
server/
  Dockerfile
  cmd/api/main.go
  internal/
```

## デプロイ前の確認

```bash
cd infra
npm install
npm run build
npm test
npx cdk synth
```

初回だけ、認証済みAWSアカウントに対してbootstrapします。

```bash
export CDK_DEFAULT_REGION=ap-northeast-1
cdk bootstrap aws://ACCOUNT_ID/ap-northeast-1
```

## OpenAI APIキー

デプロイ時に`spendable-today-dev-openai` Secretが作成されます。キー未設定でもAPIはMock fallbackで動作します。実際にOpenAIを使う場合だけ値を設定してください。

```bash
aws secretsmanager put-secret-value \
  --secret-id spendable-today-dev-openai \
  --secret-string 'sk-REPLACE_ME' \
  --region ap-northeast-1
```

既存Secretを使う場合は`-c openAiSecretArn=...`を指定します。キーをソースコード、CloudFormation Outputs、ログへ出力してはいけません。

## PagerDuty routing key

デプロイ時に`spendable-today-dev-pagerduty` Secretが作成されます。PagerDuty Events API v2のIntegrationを作成し、routing keyを設定してください。

```bash
aws secretsmanager put-secret-value \
  --secret-id spendable-today-dev-pagerduty \
  --secret-string 'REPLACE_WITH_ROUTING_KEY' \
  --region ap-northeast-1
```

既存Secretを使う場合は`-c pagerDutySecretArn=...`を指定します。未設定時は通知失敗をDBへ記録し、次の上限超過相談で再試行します。

## デプロイと確認

```bash
cd infra
npx cdk deploy
curl -i "http://ALB_DNS_NAME/health"
```

出力された`ApiBaseUrl`をFlutterの`API_BASE_URL`へ設定します。ALBのヘルスチェックは`/health`、コンテナのヘルスチェックは`/health`です。

## 削除と制約

未デプロイ環境向けのdev構成です。RDS、ECR、Secretは`cdk destroy`で削除対象です。データを保持する環境ではRDSのRemovalPolicy、バックアップ保持、deletion protectionを見直してください。

本番化にはHTTPS/ACM、認証・認可、レート制限、監視、RDSバックアップ、復旧手順、Auto Scalingが必要です。
