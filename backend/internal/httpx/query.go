package httpx

import "strconv"

// ParseInt32 parses a base-10 integer that must fit in int32. Plain
// strconv.Atoi followed by int32(n) wraps silently on 64-bit hosts
// (4294967297 becomes 1), so every query/path value headed for an int32
// column or sqlc parameter goes through here instead.
func ParseInt32(raw string) (int32, error) {
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}
