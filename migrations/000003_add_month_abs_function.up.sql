-- Вспомогательная функция: переводит дату формата 'MM-YYYY' в абсолютный номер месяца.
-- Например: '03-2025' → 2025*12 + 3 = 24303
-- IMMUTABLE позволяет PostgreSQL кэшировать результат и использовать индексы.
CREATE OR REPLACE FUNCTION month_abs(date_str VARCHAR(7))
RETURNS INTEGER AS $$
    SELECT (
        EXTRACT(YEAR FROM TO_DATE(date_str, 'MM-YYYY')) * 12 +
        EXTRACT(MONTH FROM TO_DATE(date_str, 'MM-YYYY'))
    )::int
$$ LANGUAGE SQL IMMUTABLE;
