-- Составной индекс для оптимизации запроса CalculateTotalCost.
-- Покрывает фильтрацию по user_id и date-полям.
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_dates
    ON subscriptions(user_id, start_date, end_date);
