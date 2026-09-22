-- psqlで対象月を指定して実行する。
-- 例: psql "$DATABASE_URL" -v report_month=2026-09 -f docs/monthly-cost-report.sql
--
-- 月次サマリーは利用者別の各行へ同じ値を載せる。対象月の実利用者が0人でも、
-- user_idがNULLのサマリー行を1行返す。
WITH params AS (
  SELECT
    :'report_month'::text AS report_month,
    to_date(:'report_month' || '-01', 'YYYY-MM-DD')::timestamp
      AT TIME ZONE 'Asia/Tokyo' AS month_start,
    (to_date(:'report_month' || '-01', 'YYYY-MM-DD') + INTERVAL '1 month')::timestamp
      AT TIME ZONE 'Asia/Tokyo' AS month_end
),
active_users AS (
  SELECT usage.user_id, usage.consultation_count
  FROM monthly_consultation_usage AS usage
  CROSS JOIN params
  WHERE usage.month = params.report_month
    AND usage.consultation_count > 0
),
active_user_totals AS (
  SELECT
    count(*) AS active_user_count,
    COALESCE(sum(consultation_count), 0) AS monthly_consultation_count
  FROM active_users
),
monthly_usage AS (
  SELECT
    usage.user_id,
    price.effective_from IS NOT NULL AS price_available,
    CASE WHEN price.effective_from IS NOT NULL THEN
      ((usage.input_tokens - usage.cached_input_tokens) * price.input_usd_per_million
       + usage.cached_input_tokens * price.cached_input_usd_per_million
       + usage.output_tokens * price.output_usd_per_million) / 1000000
    END AS api_cost_usd
  FROM llm_usage AS usage
  CROSS JOIN params
  LEFT JOIN LATERAL (
    SELECT
      effective_from,
      input_usd_per_million,
      cached_input_usd_per_million,
      output_usd_per_million
    FROM llm_model_prices
    WHERE model = usage.model
      AND effective_from <= (usage.occurred_at AT TIME ZONE 'Asia/Tokyo')::date
    ORDER BY effective_from DESC
    LIMIT 1
  ) AS price ON true
  WHERE usage.occurred_at >= params.month_start
    AND usage.occurred_at < params.month_end
),
api_usage_by_user AS (
  SELECT
    user_id,
    count(*) AS api_call_count,
    count(*) FILTER (WHERE price_available) AS priced_api_call_count,
    count(*) FILTER (WHERE NOT price_available) AS unpriced_api_call_count,
    COALESCE(sum(api_cost_usd), 0) AS priced_api_cost_usd
  FROM monthly_usage
  GROUP BY user_id
),
monthly_api_totals AS (
  SELECT
    count(*) AS monthly_api_call_count,
    count(*) FILTER (WHERE price_available) AS monthly_priced_api_call_count,
    count(*) FILTER (WHERE NOT price_available) AS monthly_unpriced_api_call_count,
    COALESCE(sum(api_cost_usd), 0) AS monthly_priced_api_cost_usd
  FROM monthly_usage
),
operating_cost AS (
  SELECT
    cost.month IS NOT NULL AS operating_cost_recorded,
    cost.infrastructure_cost_usd,
    cost.support_case_count,
    cost.support_minutes
  FROM params
  LEFT JOIN monthly_operating_costs AS cost ON cost.month = params.report_month
),
monthly_summary AS (
  SELECT
    params.report_month AS month,
    costs.operating_cost_recorded,
    api.monthly_unpriced_api_call_count = 0 AS api_price_coverage_complete,
    active.active_user_count,
    active.monthly_consultation_count,
    api.monthly_api_call_count,
    api.monthly_priced_api_call_count,
    api.monthly_unpriced_api_call_count,
    round(api.monthly_priced_api_cost_usd, 6) AS monthly_priced_api_cost_usd,
    round(costs.infrastructure_cost_usd, 6) AS monthly_infrastructure_cost_usd,
    CASE
      WHEN costs.operating_cost_recorded AND api.monthly_unpriced_api_call_count = 0
      THEN round(api.monthly_priced_api_cost_usd + costs.infrastructure_cost_usd, 6)
    END AS monthly_measured_cash_cost_usd,
    CASE
      WHEN costs.operating_cost_recorded AND api.monthly_unpriced_api_call_count = 0
      THEN round(
        (api.monthly_priced_api_cost_usd + costs.infrastructure_cost_usd)
        / NULLIF(active.active_user_count, 0),
        6
      )
    END AS average_measured_cash_cost_usd_per_active_user,
    costs.support_case_count AS monthly_support_case_count,
    costs.support_minutes AS monthly_support_minutes,
    round(
      costs.support_minutes::numeric / NULLIF(active.active_user_count, 0),
      2
    ) AS support_minutes_per_active_user
  FROM params
  CROSS JOIN active_user_totals AS active
  CROSS JOIN monthly_api_totals AS api
  CROSS JOIN operating_cost AS costs
),
user_details AS (
  SELECT
    active.user_id,
    active.consultation_count,
    COALESCE(api.api_call_count, 0) AS api_call_count,
    COALESCE(api.priced_api_call_count, 0) AS priced_api_call_count,
    COALESCE(api.unpriced_api_call_count, 0) AS unpriced_api_call_count,
    COALESCE(api.unpriced_api_call_count, 0) = 0 AS api_price_coverage_complete,
    round(COALESCE(api.priced_api_cost_usd, 0), 6) AS priced_api_cost_usd,
    round(
      costs.infrastructure_cost_usd / NULLIF(active_count.active_user_count, 0),
      6
    ) AS infrastructure_cost_usd_share,
    CASE
      WHEN costs.operating_cost_recorded
        AND COALESCE(api.unpriced_api_call_count, 0) = 0
      THEN round(
        COALESCE(api.priced_api_cost_usd, 0)
        + costs.infrastructure_cost_usd / NULLIF(active_count.active_user_count, 0),
        6
      )
    END AS measured_cash_cost_usd,
    round(
      costs.support_minutes::numeric / NULLIF(active_count.active_user_count, 0),
      2
    ) AS support_minutes_share
  FROM active_users AS active
  CROSS JOIN active_user_totals AS active_count
  CROSS JOIN operating_cost AS costs
  LEFT JOIN api_usage_by_user AS api ON api.user_id = active.user_id
)
SELECT
  summary.month,
  summary.operating_cost_recorded,
  summary.api_price_coverage_complete,
  summary.active_user_count,
  summary.monthly_consultation_count,
  summary.monthly_api_call_count,
  summary.monthly_priced_api_call_count,
  summary.monthly_unpriced_api_call_count,
  summary.monthly_priced_api_cost_usd,
  summary.monthly_infrastructure_cost_usd,
  summary.monthly_measured_cash_cost_usd,
  summary.average_measured_cash_cost_usd_per_active_user,
  summary.monthly_support_case_count,
  summary.monthly_support_minutes,
  summary.support_minutes_per_active_user,
  details.user_id,
  details.consultation_count,
  details.api_call_count,
  details.priced_api_call_count,
  details.unpriced_api_call_count,
  details.api_price_coverage_complete AS user_api_price_coverage_complete,
  details.priced_api_cost_usd,
  details.infrastructure_cost_usd_share,
  details.measured_cash_cost_usd,
  details.support_minutes_share
FROM monthly_summary AS summary
LEFT JOIN user_details AS details ON true
ORDER BY details.user_id NULLS FIRST;
