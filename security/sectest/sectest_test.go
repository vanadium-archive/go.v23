package sectest

import (
	"testing"

	"veyron.io/veyron/veyron2/security"
)

func TestNewKey(t *testing.T) {
	pub, priv, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	if pub == nil {
		t.Fatalf("nil PublicKey")
	}
	if priv == nil {
		t.Fatalf("nil PrivateKey")
	}

	data := []byte("the dark knight rises")
	purpose := []byte("testing")
	sig, err := security.NewInMemoryECDSASigner(priv).Sign(purpose, data)
	if err != nil {
		t.Fatal(err)
	}
	if !sig.Verify(pub, data) {
		t.Fatalf("Signature verification failed")
	}
}

func TestNewBlessingRoots(t *testing.T) {
	var (
		r          = NewBlessingRoots()
		key1, _, _ = NewKey()
		key2, _, _ = NewKey()
	)
	r.Add(key1, security.BlessingPattern("dccomics/batman/..."))
	r.Add(key2, security.BlessingPattern("dccomics/batman/personal/..."))

	tests := []struct {
		key        security.PublicKey
		blessing   string
		recognized bool
	}{
		{key1, "marvel", false},
		{key1, "dccomics/superman", false},
		{key1, "dccomics/batman", true},
		{key1, "dccomics/batman/car/gps", true},

		{key2, "dccomics/superman", false},
		// TODO(ashankar,ataly): This is a relic of the "prefix" matching in
		// BlessingPattern.MatchedBy. The plan is to change BlessingRoots so that
		// it stores root certificates not (key, pattern) pairs.
		{key2, "dccomics/batman", true},
		{key2, "dccomics/batman/public", false},
		{key2, "dccomics/batman/personal/phone", true},
	}
	for _, test := range tests {
		err := r.Recognized(test.key, test.blessing)
		if got, want := (err == nil), test.recognized; got != want {
			t.Errorf("Recognized(%v, %v) returned %v, wanted recognition: %v", test.key, test.blessing, err, want)
		}
	}
}
