package selfinstall

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSamePath exercises samePath directly (it lives in package selfinstall,
// not selfinstall_test, because the function is unexported). It matters on
// its own: TestEnsureSkipsWhenAlreadyRunningFromTarget in selfinstall_test.go
// stages the binary at the target path *before* Ensure ever runs, so
// os.Stat(target) already succeeds by the time samePath would be consulted
// -- the generic "already installed" check produces the same result, and
// that test would keep passing even if the samePath guard were deleted from
// Ensure entirely. samePath is the guard that matters on Windows: a running
// image cannot be copied over itself, and without it every boot after the
// first fails with a sharing violation. These cases pin its actual
// behaviour instead.
func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("same bytes"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("same bytes"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	t.Run("identical path", func(t *testing.T) {
		if !samePath(a, a) {
			t.Fatal("samePath(a, a) = false, want true")
		}
	})

	t.Run("same file via a differently-spelled path", func(t *testing.T) {
		spelled := filepath.Join(dir, ".", "a")
		if !samePath(a, spelled) {
			t.Fatal("samePath(a, dir/./a) = false, want true: cleans to the same path")
		}
	})

	t.Run("different existing files", func(t *testing.T) {
		if samePath(a, b) {
			t.Fatal("samePath(a, b) = true, want false: distinct files, even with identical content")
		}
	})

	t.Run("stat failure on one side", func(t *testing.T) {
		missing := filepath.Join(dir, "does-not-exist")
		if samePath(a, missing) {
			t.Fatal("samePath(existing, missing) = true, want false")
		}
		if samePath(missing, a) {
			t.Fatal("samePath(missing, existing) = true, want false")
		}
	})

	t.Run("stat failure on both sides", func(t *testing.T) {
		missingA := filepath.Join(dir, "missing-a")
		missingB := filepath.Join(dir, "missing-b")
		if samePath(missingA, missingB) {
			t.Fatal("samePath(missing, missing) with different names = true, want false")
		}
	})
}
