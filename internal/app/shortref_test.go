package app

import "testing"

func TestBuildShortRefs_UniqueAndShort(t *testing.T) {
	ids := []string{"ocid...abcd1", "ocid...abcd2", "ocid...xyzz9"}
	refs := BuildShortRefs(ids, 2)
	seen := map[string]bool{}
	for _, id := range ids {
		ref := refs[id]
		if ref == "" {
			t.Fatalf("missing ref for %s", id)
		}
		if seen[ref] {
			t.Fatalf("duplicate ref %s", ref)
		}
		seen[ref] = true
	}
	if len(refs[ids[0]]) > 5 {
		t.Fatalf("expected short ref, got %q", refs[ids[0]])
	}
}

func TestBuildShortRefs_ExpandsOnCollision(t *testing.T) {
	ids := []string{"zzabc", "yyabc"}
	refs := BuildShortRefs(ids, 2)
	if refs["zzabc"] == refs["yyabc"] {
		t.Fatalf("expected distinct refs, got same: %q", refs["zzabc"])
	}
	if len(refs["zzabc"]) < 3 || len(refs["yyabc"]) < 3 {
		t.Fatalf("expected refs to expand beyond 2 chars on collision, got %q and %q", refs["zzabc"], refs["yyabc"])
	}
}
