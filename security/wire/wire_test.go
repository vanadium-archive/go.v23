package wire

import (
	"testing"

	"veyron2/security"
)

func TestValidateBlessingName(t *testing.T) {
	var (
		valid   = []string{"alice", "alice@google", "alice@google@gmail"}
		invalid = []string{"", "alice*", "*alice", "alice*bob", "/alice", "alice/", "/alice", "alice/bob"}
	)
	for _, n := range valid {
		if err := ValidateBlessingName(n); err != nil {
			t.Errorf("ValidateBlessingName(%q) failed unexpectedly", n)
		}
	}
	for _, n := range invalid {
		if err := ValidateBlessingName(n); err == nil {
			t.Errorf("ValidateBlessingName(%q) passed unexpectedly", n)
		}
	}
}

func TestValidatePrincipalPattern(t *testing.T) {
	var (
		valid   = []security.PrincipalPattern{"*", "alice", "alice@google", "veyron/alice@google", "veyron/alice@google/bob", "alice/*", "alice/bob/*"}
		invalid = []security.PrincipalPattern{"", "alice*", "*alice", "alice*bob", "/alice", "alice/", "/alice", "*alice/bob", "alice*/bob", "alice/*/bob"}
	)
	for _, p := range valid {
		if err := ValidatePrincipalPattern(p); err != nil {
			t.Errorf("ValidatePrincipalPattern(%q) failed unexpectedly", p)
		}
	}
	for _, p := range invalid {
		if err := ValidatePrincipalPattern(p); err == nil {
			t.Errorf("ValidatePrincipalPattern(%q) passed unexpectedly", p)
		}
	}
}
