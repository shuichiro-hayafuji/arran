# 外部サービス確認手順

OpenAI Responses API、PagerDuty Events API、AWS配備を実サービスで確認するための手順。APIキー、routing key、AWS認証情報が必要なため、認証情報を管理する実施者が行う。値をIssue、ログ、画面共有、シェル履歴へ残さない。

確認にはテスト用ユーザーと検証用DBを使う。本番データを使う場合は、相談内容へ実在する氏名、連絡先、口座、店舗名などを入力しない。

## OpenAI Responses API

1. PostgreSQLを起動し、検証用ユーザーがログインできることを確認する。
2. APIキーを画面へ表示せず、環境変数へ設定する。

```sh
read -rs 'OPENAI_API_KEY?OpenAI API key: '
export OPENAI_API_KEY
export USE_MOCK_LLM=false
export OPENAI_MODEL=gpt-5.6-terra
export OPENAI_REASONING_EFFORT=medium
```

3. `server/`で`go run ./cmd/api`を実行する。
4. 別のターミナルでログインし、テスト用の新規相談を1件作成する。`USERNAME`と`PASSWORD`は検証用ユーザーの値へ置き換える。

```sh
export API_BASE_URL=http://127.0.0.1:8080
read -r 'USERNAME?Test username: '
read -rs 'PASSWORD?Test password: '
ACCESS_TOKEN="$(curl -fsS \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
  "$API_BASE_URL/auth/login" | jq -r .access_token)"
curl -fsS \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"message":"検証用の相談です。500円使ってよいですか？","planned_amount":500}' \
  "$API_BASE_URL/consultations"
unset ACCESS_TOKEN PASSWORD OPENAI_API_KEY
```

5. 応答に助言とverdictが含まれることを確認する。
6. DBで`llm_usage`の最新行を確認する。`model`が`gpt-5.6-terra`で、token数が0以上であることを確認する。相談文や回答本文が保存されていないことも列定義で確認する。

```sh
psql "$DATABASE_URL" -c '
SELECT operation, model, input_tokens, cached_input_tokens,
       output_tokens, reasoning_tokens, total_tokens, occurred_at
FROM llm_usage
ORDER BY occurred_at DESC
LIMIT 5;'
```

## PagerDuty Events API

この確認ではPagerDutyへ実際のインシデントが1件作成される。事前に検証用Service、または検証通知を受けてもよいServiceを選ぶ。

1. PagerDutyでEvents API v2 Integrationを作成し、routing keyを取得する。
2. 検証用ユーザーの月間上限を0へ一時変更する。`USER_ID`は対象ユーザーの内部IDへ置き換える。

```sh
psql "$DATABASE_URL" -v user_id=USER_ID -c '
INSERT INTO user_consultation_limits(user_id, monthly_limit)
VALUES (:user_id, 0)
ON CONFLICT (user_id) DO UPDATE
SET monthly_limit = EXCLUDED.monthly_limit,
    updated_at = CURRENT_TIMESTAMP;'
```

3. routing keyを画面へ表示せずに設定し、APIを起動する。

```sh
read -rs 'PAGERDUTY_ROUTING_KEY?PagerDuty routing key: '
export PAGERDUTY_ROUTING_KEY
export APP_ENVIRONMENT=verification
go run ./cmd/api
```

4. OpenAI確認と同じ方法でログインし、新規相談を1件作成する。
5. PagerDutyにインシデントが1件作成され、本文に相談内容、家計情報、氏名、メールアドレスが含まれないことを確認する。
6. 同じユーザーでもう1件相談し、同じ月の通知が増えないことを確認する。
7. DBで通知が`delivered`になり、`attempts`が不要に増えていないことを確認する。

```sh
psql "$DATABASE_URL" -v user_id=USER_ID -c '
SELECT user_id, month, notification_type, consultation_count,
       consultation_limit, status, attempts, last_error
FROM admin_notifications
WHERE user_id = :user_id
ORDER BY month DESC;'
```

8. 検証後、利用者別の上書きを削除して既定値へ戻し、PagerDutyの検証インシデントをresolveする。

```sh
psql "$DATABASE_URL" -v user_id=USER_ID \
  -c 'DELETE FROM user_consultation_limits WHERE user_id = :user_id;'
unset PAGERDUTY_ROUTING_KEY
```

## AWS配備

1. `aws sts get-caller-identity`で、配備先アカウントと想定アカウントが一致することを確認する。
2. [infra/README.md](../../infra/README.md)に従い、`npm run build`、`npm test`、`npx cdk synth`を実行する。
3. OpenAI APIキーとPagerDuty routing keyをSecrets Managerへ登録する。コマンド履歴へ秘密値を残さない方法を使う。
4. `npx cdk deploy`を実行し、出力された`ApiBaseUrl`の`/health`がHTTP 200を返すことを確認する。
5. ECSログで起動失敗がないこと、RDS migrationが完了していることを確認する。
6. 配備先APIでOpenAIとPagerDutyの確認を各1回行い、`llm_usage`と`admin_notifications`へ結果が保存されることを確認する。

AWSの実請求額と利用者1人あたりの原価は、1か月分の実運用データがそろった後にIssue #97で測定する。配備確認時の概算値だけで実測完了とは扱わない。

## 記録する結果

Issueへは秘密値を含めず、次だけを記録する。

- 実施日、環境、対象commit
- OpenAIのモデルとreasoning effort、相談成功の可否、`llm_usage`保存の可否
- PagerDutyの通知成功、個人・家計情報が含まれないこと、月内重複がないこと
- AWSの配備先リージョン、health check、ECS/RDSの確認結果
- 失敗した項目、エラーの要約、未確認の範囲
