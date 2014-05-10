package vom

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// matchHexPat compares the given target and pat hex codes, and returns true if
// they match.  We allow special syntax in the pat code; in addition to regular
// string matching, we allow sequences that may appear in any order.
// E.g. "1122[33,44]55" means that 33 and 44 are a sequence that may appear in
// any order, so either "1122334455" or "1122443355" are accepted.
//
// We allow this special syntax since parts of the encoding aren't
// deterministic; e.g. Go maps are unordered.
func matchHexPat(target, pat string) (bool, error) {
	orig := pat
	for pat != "" {
		start := strings.IndexRune(pat, '[')
		// If there isn't a start token, just compare the full strings.
		if start == -1 {
			return target == pat, nil
		}
		// Compare everything up to the start token.
		if !strings.HasPrefix(target, pat[:start]) {
			return false, nil
		}
		// Now compare all permutations of the sequence until we find a match.
		pat = pat[start+1:] // remove '[' too
		target = target[start:]
		end := strings.IndexRune(pat, ']')
		if end == -1 {
			return false, fmt.Errorf("Malformed hex pattern, no closing ] in %q", orig)
		}
		seqs := strings.Split(pat[:end], ",")
		if !matchPrefixSeq(target, seqs) {
			return false, nil
		}
		// Found a match, move past this sequence.  An example of our state:
		//    pat="11,22]3344" target="22113344" end_seq=5
		// We need to remove everything up to and including "]" from pat, and
		// remove the matched sequence length from target, so that we get:
		//    pat="3344" target="3344"
		pat = pat[end+1:]
		target = target[end-len(seqs)+1:]
	}
	return target == "", nil
}

// tryCall tries to invoke a call specified by reflect.Value objects
func tryCall(meth reflect.Value, args []reflect.Value) (rv []reflect.Value, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%v", rec)
		}
	}()
	return meth.Call(args), nil
}

// isExportedName returns true if the name is exposed (i.e. begins with a capital letter)
func isExportedName(name string) bool {
	if len(name) == 0 {
		panic("Invalid name")
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

// matchPrefixSeq is a recursive function that returns true iff a prefix of the
// target string matches any permutation of the seqs strings.
func matchPrefixSeq(target string, seqs []string) bool {
	if len(seqs) == 0 {
		return true
	}
	for ix, seq := range seqs {
		if strings.HasPrefix(target, seq) {
			if matchPrefixSeq(target[len(seq):], append(seqs[:ix], seqs[ix+1:]...)) {
				return true
			}
		}
	}
	return false
}

// binFromHexPat returns a binary string based on the given hex pattern.
// Allowed hex patterns are the same as for matchHexPat.
func binFromHexPat(pat string) (string, error) {
	// TODO(toddw): We could also choose to randomly re-order the sequences.
	hex := strings.NewReplacer("[", "", "]", "", ",", "").Replace(pat)
	var bin []byte
	_, err := fmt.Sscanf(hex, "%x", &bin)
	return string(bin), err
}

// matchIndexedErrorRE is a helper that calls matchErrorRE with the errRE
// extracted from regexps at index.  If the index it out of range it uses an
// empty errRE.
func matchIndexedErrorRE(err error, regexps []string, index int) (bool, error) {
	var errRE string
	if len(regexps) >= index+1 {
		errRE = regexps[index]
	}
	return matchErrorRE(err, errRE)
}

// matchErrorRE matches the err against the errRE regexp.  It returns a boolean
// indicating whether there was a non-empty regexp, and a non-nil error if there
// was a mismatch.
func matchErrorRE(err error, errRE string) (bool, error) {
	if errRE != "" {
		if err == nil {
			return true, fmt.Errorf("succeeded, expected error regexp %q", errRE)
		}
		match, merr := regexp.MatchString(errRE, err.Error())
		if merr != nil {
			return true, fmt.Errorf("regexp match %q failed: %v", errRE, merr)
		}
		if !match {
			return true, fmt.Errorf("actual error %q, expected error regexp %q", err, errRE)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed: %v", err)
	}
	return false, nil
}

func TestMatchPrefixSeq(t *testing.T) {
	tests := []struct {
		target string
		seqs   []string
		expect bool
	}{
		{"112233", []string{"11"}, true},
		{"112233", []string{"1122"}, true},
		{"112233", []string{"11", "22"}, true},
		{"112233", []string{"22", "11"}, true},
		{"112233", []string{"112233"}, true},
		{"112233", []string{"11", "2233"}, true},
		{"112233", []string{"2233", "11"}, true},
		{"112233", []string{"112", "233"}, true},
		{"112233", []string{"233", "112"}, true},
		{"112233", []string{"1122", "33"}, true},
		{"112233", []string{"33", "1122"}, true},
		{"112233", []string{"1", "1223", "3"}, true},
		{"112233", []string{"3", "1223", "1"}, true},
		{"112233", []string{"11", "22", "33"}, true},
		{"112233", []string{"11", "33", "22"}, true},
		{"112233", []string{"22", "11", "33"}, true},
		{"112233", []string{"22", "33", "11"}, true},
		{"112233", []string{"33", "11", "22"}, true},
		{"112233", []string{"33", "22", "11"}, true},
		{"112233", []string{"1", "1", "2", "2", "3", "3"}, true},
		{"112233", []string{"1", "2", "3", "1", "2", "3"}, true},
		{"112233", []string{"332211"}, false},
		{"112233", []string{"1122333"}, false},
		{"112233", []string{"11", "22333"}, false},
		{"112233", []string{"11", "22", "333"}, false},
		{"112233", []string{"11", "11", "11"}, false},
	}
	for _, test := range tests {
		if matchPrefixSeq(test.target, test.seqs) != test.expect {
			t.Errorf("matchPrefixSeq(%q, %v) != %v", test.target, test.seqs, test.expect)
		}
	}
}

func TestMatchHexPat(t *testing.T) {
	tests := []struct {
		target, pat string
		expect      bool
	}{
		{"112233", "112233", true},
		{"112233", "[112233]", true},
		{"112233", "11[2233]", true},
		{"112233", "1122[33]", true},
		{"112233", "11[22]33", true},
		{"112233", "[11,22]33", true},
		{"112233", "[22,11]33", true},
		{"112233", "11[22,33]", true},
		{"112233", "11[33,22]", true},
		{"112233", "1[12,23]3", true},
		{"112233", "1[23,12]3", true},
		{"112233", "[11,22,33]", true},
		{"112233", "[11,33,22]", true},
		{"112233", "[22,11,33]", true},
		{"112233", "[22,33,11]", true},
		{"112233", "[33,11,22]", true},
		{"112233", "[33,22,11]", true},

		{"112233", "11223", false},
		{"112233", "1122333", false},
		{"112233", "[11223]", false},
		{"112233", "[1122333]", false},
		{"112233", "11[223]", false},
		{"112233", "11[22333]", false},
		{"112233", "11[22,3]", false},
		{"112233", "11[223,33]", false},
		{"112233", "[11,2]33", false},
		{"112233", "[22,1]33", false},
		{"112233", "11[2,3]33", false},
		{"112233", "[11,2,33]", false},
	}
	for _, test := range tests {
		actual, err := matchHexPat(test.target, test.pat)
		if err != nil {
			t.Error(err)
		}
		if actual != test.expect {
			t.Errorf("matchHexPat(%q, %q) != %v", test.target, test.pat, test.expect)
		}
	}
}
