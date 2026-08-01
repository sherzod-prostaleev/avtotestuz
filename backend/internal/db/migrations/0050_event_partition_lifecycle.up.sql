-- Keep telemetry writes out of the unbounded default partition without ever
-- deleting historical events. The current month remains in event_default;
-- future monthly partitions are prepared before data reaches them.
CREATE OR REPLACE FUNCTION ensure_event_partitions(months_ahead integer DEFAULT 18)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
  month_offset integer;
  range_start timestamptz;
  range_end timestamptz;
  partition_name text;
  created_count integer := 0;
BEGIN
  IF months_ahead < 1 OR months_ahead > 60 THEN
    RAISE EXCEPTION 'months_ahead must be between 1 and 60';
  END IF;

  FOR month_offset IN 1..months_ahead LOOP
    range_start := (date_trunc('month', CURRENT_TIMESTAMP AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')
      + make_interval(months => month_offset);
    range_end := range_start + interval '1 month';
    partition_name := 'event_' || to_char(range_start AT TIME ZONE 'UTC', 'YYYYMM');

    IF to_regclass('public.' || partition_name) IS NOT NULL THEN
      CONTINUE;
    END IF;

    -- Attaching a partition whose range already exists in the default
    -- partition would fail. Skip it instead of moving/deleting any row.
    IF EXISTS (
      SELECT 1 FROM event_default
      WHERE ts >= range_start AND ts < range_end
      LIMIT 1
    ) THEN
      RAISE WARNING 'event partition % skipped: matching rows already exist in event_default', partition_name;
      CONTINUE;
    END IF;

    EXECUTE format(
      'CREATE TABLE %I PARTITION OF event FOR VALUES FROM (%L) TO (%L)',
      partition_name, range_start, range_end
    );
    created_count := created_count + 1;
  END LOOP;

  RETURN created_count;
END;
$$;

SELECT ensure_event_partitions(18);
