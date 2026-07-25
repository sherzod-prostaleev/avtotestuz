-- arena_join.lua: atomic look-for-opponent-else-enqueue (spec §3.2)
-- KEYS: candidate bucket ZSET keys (own first, then widened)
-- ARGV[1]=selfProfileID  ARGV[2]=nowMs  ARGV[3]=ownBucketKey
local self = ARGV[1]
local now = tonumber(ARGV[2])
local ownKey = ARGV[3]

local bestKey = nil
local bestMember = nil
local bestScore = nil

for i = 1, #KEYS do
  local key = KEYS[i]
  local rows = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  if #rows >= 2 then
    local member = rows[1]
    local score = tonumber(rows[2])
    if member ~= self then
      if bestScore == nil or score < bestScore then
        bestKey = key
        bestMember = member
        bestScore = score
      end
    end
  end
end

if bestMember ~= nil then
  redis.call('ZREM', bestKey, bestMember)
  redis.call('DEL', 'arena:queued:' .. bestMember)
  return {'paired', bestMember}
end

-- Extract bucket index from own key "arena:q:<n>"
local bucket = string.match(ownKey, 'arena:q:(%d+)$') or '0'
redis.call('ZADD', ownKey, now, self)
redis.call('SET', 'arena:queued:' .. self, bucket, 'EX', 120)
return {'queued'}
