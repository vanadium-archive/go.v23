package wire

import "testing"

func TestValidateBlessingName(t *testing.T) {
	var (
		valid   = []string{"alice", "alice@google", "alice@google@gmail", "alice.jones"}
		invalid = []string{"", "alice...", "...alice", "alice...bob", "/alice", "alice/", "/alice", "alice/bob"}
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
