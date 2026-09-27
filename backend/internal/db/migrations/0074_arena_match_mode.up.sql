-- Arena match mode. Only 'ranked' (matchmaking) moves ELO: a 'friend' duel is
-- arranged by invite code, so two accounts of one person could farm rating
-- through it, and a 'bot' practice duel has no second human at all. A bot
-- duel persists a single arena_match_player row (the bot has no profile).
ALTER TABLE arena_match
  ADD COLUMN mode text NOT NULL DEFAULT 'ranked'
  CHECK (mode IN ('ranked', 'friend', 'bot'));
