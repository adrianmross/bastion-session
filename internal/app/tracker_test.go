package app

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUpsertTracked(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tracked.json")
	first := TrackedBastion{ID: "ocid1.bastion", Name: "b1", LastSeenAt: time.Now().Add(-time.Hour)}
	if err := UpsertTracked(p, first); err != nil {
		t.Fatal(err)
	}
	second := TrackedBastion{ID: "ocid1.bastion", Name: "b1-new", LastSeenAt: time.Now()}
	if err := UpsertTracked(p, second); err != nil {
		t.Fatal(err)
	}
	items, err := LoadTracked(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "b1-new" {
		t.Fatalf("expected updated name, got %s", items[0].Name)
	}
}
