package app

import "testing"

func TestParseSessionJSONIncludesTargetDetails(t *testing.T) {
	out := []byte(`{"id":"ocid1.session.oc1..s1","bastionId":"ocid1.bastion.oc1..b1","targetResourceId":"ocid1.instance.oc1..i1","targetResourceDetails":{"privateIpAddress":"10.0.0.44"},"lifecycleState":"ACTIVE","timeCreated":"2026-03-19T12:00:00Z","timeExpires":"2026-03-19T13:00:00Z"}`)
	s, err := parseSessionJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if s.BastionID != "ocid1.bastion.oc1..b1" || s.TargetResourceID != "ocid1.instance.oc1..i1" || s.TargetPrivateIP != "10.0.0.44" {
		t.Fatalf("unexpected parsed session: %#v", s)
	}
}

func TestParseSessionJSONIncludesTargetDetailsKebabCase(t *testing.T) {
	out := []byte(`{"id":"ocid1.session.oc1..s2","bastion-id":"ocid1.bastion.oc1..b2","target-resource-details":{"target-resource-id":"ocid1.instance.oc1..i2","target-resource-private-ip-address":"10.0.0.77"},"lifecycle-state":"ACTIVE","time-created":"2026-03-19T12:00:00Z","time-expires":"2026-03-19T13:00:00Z"}`)
	s, err := parseSessionJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if s.BastionID != "ocid1.bastion.oc1..b2" || s.TargetResourceID != "ocid1.instance.oc1..i2" || s.TargetPrivateIP != "10.0.0.77" {
		t.Fatalf("unexpected parsed session (kebab-case): %#v", s)
	}
}

func TestAsNestedString(t *testing.T) {
	row := map[string]any{
		"targetResourceDetails": map[string]any{
			"privateIpAddress": "10.0.0.55",
		},
	}
	got := asNestedString(row, "targetResourceDetails.privateIpAddress")
	if got != "10.0.0.55" {
		t.Fatalf("expected nested private IP, got %q", got)
	}
}

func TestAsNestedStringKebabCase(t *testing.T) {
	row := map[string]any{
		"target-resource-details": map[string]any{
			"target-resource-private-ip-address": "10.0.0.88",
		},
	}
	got := asNestedString(row, "target-resource-details.target-resource-private-ip-address")
	if got != "10.0.0.88" {
		t.Fatalf("expected nested kebab private IP, got %q", got)
	}
}
