-- arena_join.lua: atomic look-for-opponent-else-enqueue (spec §3.2)
-- KEYS: candidate bucket ZSET keys, closest first (own, -1, +1, -2, +2, …)
-- ARGV[1]=selfProfileID  ARGV[2]=nowMs  ARGV[3]=ownBucketKey  ARGV[4]=markerTTLSec
--
-- The first key holding a live waiter wins, and within a key the longest
-- waiter wins: rating closeness first, then fairness. A member whose
-- arena:queued:<id> marker is gone (queue timeout, crash, restart) is a
-- ghost; it is removed here so it can never be paired or block a bucket.
local self = ARGV[1]
local now = tonumber(ARGV[2])
local ownKey = ARGV[3]
local ttl = tonumber(ARGV[4])

for i = 1, #KEYS do
  local key = KEYS[i]
  local rows = redis.call('ZRANGE', key, 0, 49)
  for j = 1, #rows do
    local member = rows[j]
    if member ~= self then
      if redis.call('EXISTS', 'arena:queued:' .. member) == 1 then
        redis.call('ZREM', key, member)
        redis.call('DEL', 'arena:queued:' .. member)
        return {'paired', member}
      end
      redis.call('ZREM', key, member)
    end
  end
end

-- Marker value "<bucket>:<queuedAtMs>" lets the timeout watcher tell this
-- queue entry from a later one by the same player.
local bucket = string.match(ownKey, 'arena:q:(%-?%d+)$') or '0'
local marker = bucket .. ':' .. ARGV[2]
redis.call('ZADD', ownKey, now, self)
redis.call('SET', 'arena:queued:' .. self, marker, 'EX', ttl)
return {'queued', marker}
