package app

import "testing"

func TestApplyContextScopeAuthMethod(t *testing.T) {
	cfg := Config{AuthMethod: "", Profile: DefaultProfile, Region: DefaultRegion}
	cfg.ApplyContextScope(&ContextRef{Profile: "P", Region: "us-phoenix-1", AuthMethod: "security_token"})
	if cfg.AuthMethod != "security_token" {
		t.Fatalf("expected auth method from context, got %q", cfg.AuthMethod)
	}
}

func TestApplyContextScopeDoesNotOverrideExplicitAuthMethod(t *testing.T) {
	cfg := Config{AuthMethod: "api_key", Profile: DefaultProfile, Region: DefaultRegion}
	cfg.ApplyContextScope(&ContextRef{Profile: "P", Region: "us-phoenix-1", AuthMethod: "security_token"})
	if cfg.AuthMethod != "api_key" {
		t.Fatalf("expected explicit auth method to be preserved, got %q", cfg.AuthMethod)
	}
}
