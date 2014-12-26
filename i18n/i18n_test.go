// Binary i18n_test tests message catalogues.

package i18n

import "strings"
import "time"
import "testing"
import "v.io/core/veyron2/context"

// testLookupSetAndRemove tests Lookup, Set, and Set-to-empty on *cat.
func testLookupSetAndRemove(t *testing.T, cat *Catalogue, catName string) {
	want := "expected format"
	expectLookup(t, cat, "", "en", "bar", catName+" 1")
	expectLookup(t, cat, "", "en-US", "bar", catName+" 2")
	expectLookup(t, cat, "", "en", "foo", catName+" 3")
	expectLookup(t, cat, "", "en-US", "foo", catName+" 4")
	if cat.Set(LangID("en-US"), MsgID("bar"), want) != "" {
		t.Errorf("Set found en-US/bar in %s catalogue", catName)
	}
	if cat.SetWithBase(LangID("en-US"), MsgID("foo"), want) != "" {
		t.Errorf("Set found en-US/foo in %s catalogue", catName)
	}
	expectLookup(t, cat, "", "en", "bar", catName+" 5")
	expectLookup(t, cat, want, "en-US", "bar", catName+" 6")
	expectLookup(t, cat, want, "en", "foo", catName+" 7")
	expectLookup(t, cat, want, "en-US", "foo", catName+" 8")
	// Check that Set(..., "") doesn't delete the base entry.
	if cat.Set(LangID("en"), MsgID("bar"), "other format") != "" {
		t.Errorf("Set found en/bar in %s catalogue", catName)
	}
	if cat.Set(LangID("en-US"), MsgID("bar"), "") != want {
		t.Errorf("Set didn't find en-US/bar in %s catalogue", catName)
	}
	if cat.SetWithBase(LangID("en-US"), MsgID("foo"), "") != want {
		t.Errorf("Set didn't find en-US/foo in %s catalogue", catName)
	}
	// The previous SetWithBase will not have removed the base entry.
	if cat.Set(LangID("en"), MsgID("foo"), "") != want {
		t.Errorf("Set didn't find en/foo in %s catalogue", catName)
	}
	expectLookup(t, cat, "other format", "en", "bar", catName+" 9")
	// Test that a lookup of en-US finds the "en" entry.
	expectLookup(t, cat, "other format", "en-US", "bar", catName+" 10")
	if cat.Set(LangID("en"), MsgID("bar"), "") != "other format" {
		t.Errorf("Set didn't find en/bar in %s catalogue", catName)
	}
	expectLookup(t, cat, "", "en", "bar", catName+" 11")
	expectLookup(t, cat, "", "en-US", "bar", catName+" 12")
	expectLookup(t, cat, "", "en", "foo", catName+" 13")
	expectLookup(t, cat, "", "en-US", "foo", catName+" 14")
}

// testLookupSetAndRemove tests Lookup, Set, and Set-to-empty on
// a newly created catalogue.
func TestLookupSetAndRemove(t *testing.T) {
	testLookupSetAndRemove(t, new(Catalogue), "new")
}

// TestDefaultCatalogue verifies that the default Catalogue behaves like a
// catalogue and is the same whenever it's invoked.
func TestDefaultCatalogue(t *testing.T) {
	cat := Cat()
	if cat != Cat() {
		t.Errorf("got different default Catalogue")
	}
	testLookupSetAndRemove(t, cat, "default")
	if cat != Cat() {
		t.Errorf("got different default Catalogue")
	}
}

// expectFormatParams verifies that FormatParams(format, v...) generates
// want.
func expectFormatParams(t *testing.T, want string, format string, v ...interface{}) {
	got := FormatParams(format, v...)
	if want != got {
		t.Errorf("FormatParams(%q, %v): got %q, want %q", format, v, got, want)
	}
}

// TestFormatParams tests the FormatParams() call with various arguments
func TestFormatParams(t *testing.T) {
	expectFormatParams(t, "", "", "1st")
	expectFormatParams(t, "", "{_}")
	expectFormatParams(t, "? ? ? ? ? ?", "{0} {1} {2} {3} {4} {5}")
	expectFormatParams(t, "{ foo }?", "{ foo }{2}")
	expectFormatParams(t, "3rd: foo 2nd bar 1st 4th (3rd)",
		"{3}: foo {2} bar {_} ({3})", "1st", "2nd", "3rd", "4th")
	expectFormatParams(t, "?: foo 4th ?",
		"{0}: foo {4} {5}", "1st", "2nd", "3rd", "4th")
	expectFormatParams(t, " foo 1st 2nd 3rd 4th{-1}",
		"{_} foo {_}{-1}", "1st", "2nd", "3rd", "4th")
	expectFormatParams(t, "{ foo }2nd",
		"{ foo }{2}", "1st", "2nd", "3rd", "4th")

	// Test the formatting of colon-formats.
	expectFormatParams(t, "", "{:_}")
	expectFormatParams(t, "", "{_:}")
	expectFormatParams(t, "", "{:_:}")

	expectFormatParams(t, ": 1st 2nd", "{:_}", "1st", "2nd")
	expectFormatParams(t, "1st 2nd:", "{_:}", "1st", "2nd")
	expectFormatParams(t, ": 1st 2nd:", "{:_:}", "1st", "2nd")

	expectFormatParams(t, "", "{:_}", "")
	expectFormatParams(t, "", "{_:}", "")
	expectFormatParams(t, "", "{:_:}", "")

	expectFormatParams(t, ": 1st", "{:1}", "1st")
	expectFormatParams(t, "1st:", "{1:}", "1st")
	expectFormatParams(t, ": 1st:", "{:1:}", "1st")

	expectFormatParams(t, "", "{:1}", "")
	expectFormatParams(t, "", "{1:}", "")
	expectFormatParams(t, "", "{:1:}", "")

	expectFormatParams(t, "?: ? ?: ?: ?: ?", "{0}{:1} {2:} {3}{:4:} {5}")

	expectFormatParams(t, "{: foo }?", "{: foo }{2}")
	expectFormatParams(t, "{ foo :}?", "{ foo :}{2}")
	expectFormatParams(t, "{: foo :}?", "{: foo :}{2}")

	expectFormatParams(t, "3rd: foo 2nd bar: 1st 4th (3rd)",
		"{3:} foo {2} bar{:_} ({3})", "1st", "2nd", "3rd", "4th")

	expectFormatParams(t, "?: foo: 4th ?",
		"{0:} foo{:4} {5}", "1st", "2nd", "3rd", "4th")

	expectFormatParams(t, " foo: 1st 2nd 3rd 4th{-1}",
		"{_:} foo{:_}{-1}", "1st", "2nd", "3rd", "4th")

	expectFormatParams(t, "{ foo }: 2nd",
		"{4:}{ foo }{:2}", "1st", "2nd", "3rd", "")

	expectFormatParams(t, "1st foo 2nd: bar: 3rd wombat: 4th: numbat",
		"{1} foo {2:} bar{:3} wombat{:4:} numbat",
		"1st", "2nd", "3rd", "4th")

	expectFormatParams(t, " foo  bar wombat numbat",
		"{1} foo {2:} bar{:3} wombat{:4:} numbat",
		"", "", "", "")
}

var mergeData string = `# In what follows we use the "languages" "fwd" and "back".
fwd foo "{1} foo to {2}"
# Next line has a missing trailing double quote, so will be ignored.
 back   foo   "{2} from foo {1}

# Comment "quote"

# The following two lines are ignored, since each has fewer than three tokens.
one
one two

fwd 	bar "{1} bar to {2}"
back bar "{2} from bar {1}" extraneous word

back funny.msg.id "{2} from funny msg id {1}"
odd.lang.id funny.msg.id "odd and\b \"funny\""
`

// expectLookup verifies that cat.Lookup(lang, msg)==want
func expectLookup(t *testing.T, cat *Catalogue, want string,
	lang string, msg string, tag string) {

	got := cat.Lookup(LangID(lang), MsgID(msg))
	if want != got {
		t.Errorf("%s: cat.Lookup(%s, %s): got %q, want %q",
			tag, lang, msg, got, want)
	}
}

// expectInMap verifies that m[s]==true.
func expectInMap(t *testing.T, m map[string]bool, s string) {
	if !m[s] {
		t.Errorf("m[%#v]==false; want true", s)
	}
}

// A writableString can be appended to via the Writer interface.
type writeableString struct {
	s string
}

// Write writes p to the the target string.
func (ws *writeableString) Write(p []byte) (n int, err error) {
	ws.s += string(p)
	return len(p), nil
}

// TestMerge tests the Merge() call.
func TestMergeAndOutput(t *testing.T) {
	cat := new(Catalogue)

	// Check that Merge() works.
	cat.Merge(strings.NewReader(mergeData))
	expectLookup(t, cat, "{1} foo to {2}", "fwd", "foo", "1")
	expectLookup(t, cat, "", "back", "foo", "2")
	expectLookup(t, cat, "{1} bar to {2}", "fwd", "bar", "3")
	expectLookup(t, cat, "{2} from bar {1}", "back", "bar", "4")
	expectLookup(t, cat, "{2} from funny msg id {1}", "back", "funny.msg.id", "5")
	expectLookup(t, cat, "odd and\b \"funny\"", "odd.lang.id", "funny.msg.id", "6")

	// Verify that the result of Output is as expected.
	var ws writeableString
	err := cat.Output(&ws)
	if err != nil {
		t.Errorf("error from cat.Output(): %v", err)
	}

	m := make(map[string]bool)
	for _, line := range strings.Split(ws.s, "\n") {
		if line != "" {
			m[line] = true
		}
	}
	expectInMap(t, m, "fwd foo \"{1} foo to {2}\"")
	expectInMap(t, m, "fwd bar \"{1} bar to {2}\"")
	expectInMap(t, m, "back bar \"{2} from bar {1}\"")
	expectInMap(t, m, "back funny.msg.id \"{2} from funny msg id {1}\"")
	expectInMap(t, m, "odd.lang.id funny.msg.id \"odd and\\b \\\"funny\\\"\"")
	if want := 5; len(m) != want {
		t.Errorf("wrong number of lines in <%s>; got %d, want %d ", ws.s, len(m), want)

	}
}

// expectNormalizeLangID verifies that NormalizeLangID(input)==want.
func expectNormalizeLangID(t *testing.T, input string, want LangID) {
	got := NormalizeLangID(input)
	if got != want {
		t.Errorf("NormalizeLangID(%s) got %q, want %q", input, got, want)
	}
}

// TestNormalizeLangID tests NormalizeLangID().
func TestNormalizeLangID(t *testing.T) {
	expectNormalizeLangID(t, "en", "en")
	expectNormalizeLangID(t, "en-US", "en-US")
	expectNormalizeLangID(t, "en_US", "en-US")
}

// expectBaseLangID verifies that BaseLangID(input)==want.
func expectBaseLangID(t *testing.T, input LangID, want LangID) {
	got := BaseLangID(input)
	if got != want {
		t.Errorf("BaseLangID(%s) got %q, want %q", input, got, want)
	}
}

// TestBaseLangID tests BaseLangID().
func TestBaseLangID(t *testing.T) {
	expectBaseLangID(t, "en", "en")
	expectBaseLangID(t, "en-US", "en")
}

// A dummyContext is a basic implementation of context.T for testing.
type dummyContext map[interface{}]interface{}

// Deadline stubs out context.T's call of the same name.
func (dc dummyContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done stubs out context.T's call of the same name.
func (dc dummyContext) Done() (c <-chan struct{}) {
	return c
}

// Err stubs out context.T's call of the same name.
func (dc dummyContext) Err() error {
	return nil
}

// Runtime stubs out context.T's call of the same name.
func (dc dummyContext) Runtime() interface{} {
	return nil
}

// Value returns the value corresponding to key in dc's Value map.
// It implements context.T's Value function.
func (dc dummyContext) Value(key interface{}) interface{} {
	return dc[key]
}

// WithCancel stubs out context.T's call of the same name.
func (dc dummyContext) WithCancel() (ctx context.T, cancel context.CancelFunc) {
	return nil, nil
}

// WithDeadline stubs out context.T's call of the same name.
func (dc dummyContext) WithDeadline(deadline time.Time) (context.T, context.CancelFunc) {
	return nil, nil
}

// WithTimeout stubs out context.T's call of the same name.
func (dc dummyContext) WithTimeout(timeout time.Duration) (context.T, context.CancelFunc) {
	return nil, nil
}

// WithValue returns a dummyContext with Value(key)==val.
func (dc dummyContext) WithValue(key interface{}, val interface{}) context.T {
	newDC := make(dummyContext)
	for k, v := range dc {
		newDC[k] = v
	}
	newDC[key] = val
	return newDC
}

// TestBaseLangID tests LangIDFromContext() and ContextWithLangID.
func TestLangIDFromContext(t *testing.T) {
	var dcWithoutLangID context.T = make(dummyContext)
	dcWithEN := ContextWithLangID(dcWithoutLangID, "en")
	dcWithFR := ContextWithLangID(dcWithEN, "fr")
	var got LangID

	got = LangIDFromContext(dcWithoutLangID)
	if got != NoLangID {
		t.Errorf("LangIDFromContext(dcWithoutLangID); got %v, want \"\"", got)
	}

	got = LangIDFromContext(dcWithEN)
	if got != LangID("en") {
		t.Errorf("LangIDFromContext(dcWithEN); got %v, want \"en\"", got)
	}

	got = LangIDFromContext(dcWithFR)
	if got != LangID("fr") {
		t.Errorf("LangIDFromContext(dcWithFR); got %v, want \"fr\"", got)
	}
}
