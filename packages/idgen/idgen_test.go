package idgen

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUUIDV7ReturnsUniqueVersion7ID(t *testing.T) {
	first, err := NewUUIDV7()
	if err != nil {
		t.Fatalf("first uuid v7: %v", err)
	}
	second, err := NewUUIDV7()
	if err != nil {
		t.Fatalf("second uuid v7: %v", err)
	}
	if first == second {
		t.Fatal("uuid v7 values should be unique")
	}

	parsed, err := uuid.Parse(first)
	if err != nil {
		t.Fatalf("parse uuid v7: %v", err)
	}
	if parsed.Version() != 7 {
		t.Fatalf("uuid version = %d, want 7", parsed.Version())
	}
}
