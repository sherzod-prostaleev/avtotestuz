-- High-water mark of the sequential bilet-unlock chain, frozen by the
-- "Tozalash" control on the tickets screen.
--
-- Bilet #N+1 unlocks once #N has a completed_at, so deleting variant_progress
-- to clear a learner's scores would also re-lock every bilet they had already
-- opened. Tozalash records how far the chain had walked here first, and
-- IsVariantUnlocked treats a bilet at or below this number as having its
-- sequential gate satisfied. VIP entitlement is still required on top, exactly
-- as before -- this column only stands in for "the previous bilet was
-- completed", never for "this profile pays".
--
-- Written as GREATEST(current, new) so repeated clears can only ever raise it,
-- and computed from the completion chain alone (not from VIP state), so a
-- learner who clears while their VIP has lapsed keeps their place when it
-- comes back.
--
-- Default 0 means "never cleared": no bilet number is <= 0, so every existing
-- profile keeps precisely today's unlock behaviour.
ALTER TABLE profile
  ADD COLUMN variant_unlock_ceiling int NOT NULL DEFAULT 0;
