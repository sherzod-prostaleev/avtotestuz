package session

import "testing"

func TestClampMySessionsLimit(t *testing.T) {
	for in, want := range map[int]int32{
		-1: 20, 0: 20, 1: 1, 100: 100, 101: 100, 1 << 40: 100,
	} {
		if got := clampMySessionsLimit(in); got != want {
			t.Errorf("clampMySessionsLimit(%d) = %d, want %d", in, got, want)
		}
	}
}
