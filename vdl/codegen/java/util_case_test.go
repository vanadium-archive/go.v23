package java

import (
	"testing"
)

func TestConstCase(t *testing.T) {
	testcases := []struct {
		name, want string
	}{
		{"TestFunction", "TEST_FUNCTION"},
		{"BIGNumber", "BIG_NUMBER"},
		{"SHA256Hash", "SHA_256_HASH"},
		{"Sha256Hash", "SHA_256_HASH"},
		{"Sha256hash", "SHA_256_HASH"},
		{"THISIsAHugeVarname", "THIS_IS_A_HUGE_VARNAME"},
		{"Sha256MD5Function", "SHA_256_MD_5_FUNCTION"},
		{"IfIHadADollar4EachTest", "IF_I_HAD_A_DOLLAR_4_EACH_TEST"},
	}

	for _, testcase := range testcases {
		if want, got := testcase.want, toConstCase(testcase.name); want != got {
			t.Errorf("toConstCase(%q) error, want %q, got %q", testcase.name, want, got)
		}
	}
}
