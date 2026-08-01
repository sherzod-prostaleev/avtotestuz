-- Partitions may contain production telemetry. Keep them attached so rollback
-- cannot delete data; remove only the maintenance function.
DROP FUNCTION IF EXISTS ensure_event_partitions(integer);
