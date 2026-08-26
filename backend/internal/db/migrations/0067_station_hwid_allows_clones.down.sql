-- Restoring the old index requires that no two active stations share an
-- hwid_hash. A school imaged from one master disk violates that by design, so
-- revoke the newer duplicates before recreating it -- keeping, for each
-- hwid_hash, the station that was seen most recently.
UPDATE b2b_station s SET status = 'revoked'
WHERE s.status = 'active'
  AND EXISTS (
    SELECT 1 FROM b2b_station peer
    WHERE peer.hwid_hash = s.hwid_hash
      AND peer.status = 'active'
      AND (peer.last_seen_at, peer.id) > (s.last_seen_at, s.id)
  );

DROP INDEX b2b_station_active_hwid_key_uidx;

CREATE UNIQUE INDEX b2b_station_active_hwid_uidx
  ON b2b_station (hwid_hash) WHERE status = 'active';
