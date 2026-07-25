package sentryx

import "testing"

func TestInitEmptyDSNIsNoOp(t *testing.T) {
	flush, err := Init("", "dev")
	if err != nil {
		t.Fatalf("Init empty: %v", err)
	}
	if flush == nil {
		t.Fatal("flush must be non-nil even when disabled")
	}
	flush() // must not panic
}

func TestInitWhitespaceDSNIsNoOp(t *testing.T) {
	flush, err := Init("  \n\t  ", "staging")
	if err != nil {
		t.Fatalf("Init whitespace: %v", err)
	}
	flush()
}

func TestInitWithDSN(t *testing.T) {
	// Valid-shaped DSN; SDK does not require a live project for Init.
	flush, err := Init("https://publickey@127.0.0.1/1", "dev")
	if err != nil {
		t.Fatalf("Init with DSN: %v", err)
	}
	if flush == nil {
		t.Fatal("flush nil")
	}
	flush()
}
