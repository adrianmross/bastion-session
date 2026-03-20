package app

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadCurrent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "current.json")
	in := CurrentBastion{
		ID:            "ocid1.bastion",
		Name:          "dev-bastion",
		CompartmentID: "ocid1.compartment",
		Region:        "us-chicago-1",
		Profile:       "oabcs1-terraform",
		SSHPublicKey:  "/Users/test/.ssh/id_ed25519.pub",
		ContextName:   "dev",
		Source:        "tracked",
	}
	if err := SaveCurrent(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadCurrent(p)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.ID != in.ID || out.Profile != in.Profile || out.Source != in.Source || out.SSHPublicKey != in.SSHPublicKey {
		t.Fatalf("unexpected current selection: %#v", out)
	}
}
