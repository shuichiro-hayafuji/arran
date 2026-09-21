-- psqlで対象月を指定して実行する。
-- 例: psql "$DATABASE_URL" -v report_month=2026-09 -f docs/monthly-cost-report.sql
WITH active_users AS (
  SELECT user_id, consultation_count
  FROM monthly_consultation_usage
  WHERE month = :'report_month' AND consultation_count > 0
),
active_user_count AS (
  SELECT count(*)::numeric AS value FROM active_users
),
api_costs AS (
  SELECT
    usage.user_id,
    count(*) AS api_call_count,
    sum(
      ((usage.input_tokens - usage.cached_input_tokens) * price.input_usd_per_million
       + usage.cached_input_tokens * price.cached_input_usd_per_million
       + usage.output_tokens * price.output_usd_per_million) / 1000000
    ) AS api_cost_usd
  FROM llm_usage AS usage
  JOIN LATERAL (
    SELECT * FROM llm_model_prices
    WHERE model = usage.model AND effective_from <= usage.occurred_at::date
    ORDER BY effective_from DESC LIMIT 1
  ) AS price ON true
  WHERE to_char(usage.occurred_at AT TIME ZONE 'Asia/Tokyo', 'YYYY-MM') = :'report_month'
  GROUP BY usage.user_id
)
SELECT
  :'report_month' AS month,
  active.user_id,
  active.consultation_count,
  COALESCE(api.api_call_count, 0) AS api_call_count,
  round(COALESCE(api.api_cost_usd, 0), 6) AS api_cost_usd,
  round(cost.infrastructure_cost_usd / NULLIF(active_count.value, 0), 6) AS infrastructure_cost_usd_per_active_user,
  round(COALESCE(api.api_cost_usd, 0) + cost.infrastructure_cost_usd / NULLIF(active_count.value, 0), 6) AS measured_cost_usd_per_active_user,
  round(cost.support_minutes::numeric / NULLIF(active_count.value, 0), 2) AS support_minutes_per_active_user,
  cost.support_case_count
FROM active_users AS active
CROSS JOIN active_user_count AS active_count
JOIN monthly_operating_costs AS cost ON cost.month = :'report_month'
LEFT JOIN api_costs AS api ON api.user_id = active.user_id
ORDER BY active.user_id;
