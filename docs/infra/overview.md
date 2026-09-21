# Infraのアーキテクチャ

`infra/` は、`server/` の同じGo APIコンテナをAWSへ配備するCDK構成です。ローカル開発用のPostgreSQL Composeは `server/compose.yaml` にあり、InfraはAWS上の実行環境を担当します。

## AWS構成

```text
Internet
  → internet-facing ALB :80
  → ECS Fargate（private subnet, :8080）
       ├─ RDS PostgreSQL 16（private subnet, :5432）
       └─ NAT Gateway → OpenAI Responses API / PagerDuty Events API
```

CDKの `DockerImageAsset` は `server/` のDockerfileをx86_64 Linux向けにビルドします。ECSにはRDSの接続情報とSecrets ManagerのOpenAIキー、PagerDuty routing keyを渡します。OpenAIキーが未設定でもAPIはMock fallbackで動作します。

## 構成

```text
infra/
├── bin/app.ts
├── lib/constructs/{network,database,container,alb}-construct.ts
├── lib/stacks/spendable-today-stack.ts
└── test/spendable-today-stack.test.ts

server/
├── Dockerfile
├── cmd/api/main.go
└── internal/
```

ネットワーク、DB、コンテナ、ALBをCDK constructとして分離し、Stackで組み立てます。APIの業務ルールやDB adapterは `server/` が正本です。

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

デプロイ後はALBの `/health` とコンテナのヘルスチェックを確認し、出力された `ApiBaseUrl` をFlutterの `API_BASE_URL` に設定します。

## Secrets Manager

デプロイ時に `spendable-today-dev-openai` Secretを作成します。実際にOpenAIを使う場合だけ値を設定してください。キーをソースコード、CloudFormation Outputs、ログへ出力してはいけません。

```bash
aws secretsmanager put-secret-value \
  --secret-id spendable-today-dev-openai \
  --secret-string 'sk-REPLACE_ME' \
  --region ap-northeast-1
```

既存Secretを使う場合は `-c openAiSecretArn=...` を指定します。

## 現在の制約

- AWSは未デプロイです。`cdk synth` 成功だけではRDS接続や実コンテナの起動を保証しません。
- dev構成のため、`cdk destroy` ではRDS、ECR、Secretが削除対象です。
- 本番化にはHTTPS / ACM、認証・認可、レート制限、監視、RDSバックアップ、復旧手順、Auto Scalingが必要です。
- RDSを保持する環境ではRemovalPolicy、バックアップ保持、deletion protectionを見直します。

## 関連資料

- [インフラ業務ルール](business.md)
- [インフラ実装ルール](architecture.md)
- [インフラ実装時の作法](conventions.md)
- [AWS配備の残課題](../../infra/REMAINING_TASKS.md)
- [インフラREADME](../../infra/README.md)
