# AWS 配備の残課題

`server/` を唯一のAPI実装として、ECS Fargate + RDS PostgreSQLへ配備する構成です。ローカルDocker Composeも同じPostgreSQL adapterを使用します。

## デプロイ前

- `cd infra && npm install && npm run build && npm test && npx cdk synth`
- Docker Desktopで`server/`のLinux/x86_64イメージをビルドする
- RDSへ接続したコンテナでmigrationが適用できることを確認する
- ALBとコンテナの`/health`ヘルスチェックを確認する

## 初回デプロイ後

- `spendable-today-dev-openai`へ必要時のみOpenAI APIキーを設定する
- ALBの`ApiBaseUrl`をFlutterの`API_BASE_URL`に設定する
- RDSのバックアップ・復旧手順を定義する

## 本番化時

- HTTPS/ACM、認証・認可、レート制限、監視、アラーム、CI/CD
- RDSのバックアップ保持期間、削除保護、復旧訓練
- OpenAI送信データの監査、利用量上限、キーのローテーション
