-- name: SalesReportDaily :many
-- Daily sales aggregation for a date range.
SELECT
  DATE(created_at) as report_date,
  COUNT(*) as order_count,
  COALESCE(SUM(subtotal), 0) as net_revenue,
  COALESCE(SUM(vat_total), 0) as vat_collected,
  COALESCE(SUM(total), 0) as gross_revenue,
  COALESCE(SUM(discount_amount), 0) as total_discounts
FROM orders
WHERE payment_status = 'paid'
  AND created_at >= @from_date
  AND created_at < @to_date
GROUP BY DATE(created_at)
ORDER BY report_date;

-- name: SalesReportSummary :one
-- Summary metrics for a period.
SELECT
  COUNT(*) as order_count,
  COALESCE(SUM(subtotal), 0) as net_revenue,
  COALESCE(SUM(vat_total), 0) as vat_collected,
  COALESCE(SUM(total), 0) as gross_revenue,
  COALESCE(SUM(discount_amount), 0) as total_discounts,
  CASE WHEN COUNT(*) > 0 THEN COALESCE(SUM(total), 0) / COUNT(*) ELSE 0 END as average_order_value
FROM orders
WHERE payment_status = 'paid'
  AND created_at >= @from_date
  AND created_at < @to_date;

-- name: VATReportByCountry :many
-- VAT collected per country per rate type for a period.
SELECT
  o.vat_country_code as country_code,
  ec.name as country_name,
  oi.vat_rate_type as rate_type,
  oi.vat_rate as rate,
  COALESCE(SUM(oi.net_unit_price * oi.quantity), 0) as net_sales,
  COALESCE(SUM(oi.vat_amount), 0) as vat_collected,
  COALESCE(SUM(oi.gross_unit_price * oi.quantity), 0) as gross_sales,
  COUNT(DISTINCT o.id) as order_count
FROM orders o
JOIN order_items oi ON oi.order_id = o.id
LEFT JOIN eu_countries ec ON ec.country_code = o.vat_country_code
WHERE o.payment_status = 'paid'
  AND o.vat_reverse_charge = false
  AND o.created_at >= @from_date
  AND o.created_at < @to_date
GROUP BY o.vat_country_code, ec.name, oi.vat_rate_type, oi.vat_rate
ORDER BY o.vat_country_code, oi.vat_rate_type;

-- name: VATReverseChargeReport :many
-- B2B reverse charge orders for a period.
SELECT
  o.id,
  o.order_number,
  o.email,
  o.vat_number,
  o.vat_company_name,
  o.vat_country_code,
  o.subtotal as net_total,
  o.created_at
FROM orders o
WHERE o.payment_status = 'paid'
  AND o.vat_reverse_charge = true
  AND o.created_at >= @from_date
  AND o.created_at < @to_date
ORDER BY o.created_at DESC;

-- name: VATReverseChargeSummary :one
-- Summary of B2B reverse charge for a period.
SELECT
  COUNT(*) as order_count,
  COALESCE(SUM(subtotal), 0) as total_net
FROM orders
WHERE payment_status = 'paid'
  AND vat_reverse_charge = true
  AND created_at >= @from_date
  AND created_at < @to_date;

-- name: TopProductsByRevenue :many
-- Top selling products by revenue for a period.
SELECT
  oi.product_name,
  SUM(oi.quantity)::bigint as total_quantity,
  COALESCE(SUM(oi.total_price), 0) as total_revenue
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
WHERE o.payment_status = 'paid'
  AND o.created_at >= @from_date
  AND o.created_at < @to_date
GROUP BY oi.product_name
ORDER BY total_revenue DESC
LIMIT @max_results;
