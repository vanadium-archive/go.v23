package naming

import (
	"testing"
)

func TestSplitName(t *testing.T) {
	cases := []struct {
		input, address, name string
	}{
		{"", "", ""},
		{"/", "", ""},
		{"//", "", "//"},
		{"//abc@@/foo", "", "//abc@@/foo"},
		{"a", "", "a"},
		{"/a", "a", ""},
		{"/a/", "a", ""},
		{"a/b", "", "a/b"},
		{"/a/b", "a", "b"},
		{"abc@@/foo", "", "abc@@/foo"},
		{"/abc@@/foo", "abc@@", "foo"},
		{"/abc/foo", "abc", "foo"},
		{"/abc/foo//x", "abc", "foo//x"},
		{"/abc:20/foo", "abc:20", "foo"},
		{"/abc//foo/bar", "abc", "//foo/bar"},
		{"/0abc:20/foo", "0abc:20", "foo"},
		{"/abc1.2:20/foo", "abc1.2:20", "foo"},
		{"/abc:xx/foo", "abc:xx", "foo"},
		{"/-abc/foo", "-abc", "foo"},
		{"/a.-abc/foo", "a.-abc", "foo"},
		{"/[01:02::]:444", "[01:02::]:444", ""},
		{"/[01:02::]:444/foo", "[01:02::]:444", "foo"},
		{"/12.3.4.5:444", "12.3.4.5:444", ""},
		{"/12.3.4.5:444/foo", "12.3.4.5:444", "foo"},
		{"/12.3.4.5", "12.3.4.5", ""},
		{"/12.3.4.5/foo", "12.3.4.5", "foo"},
		{"/12.3.4.5//foo", "12.3.4.5", "//foo"},
		{"/12.3.4.5/foo//bar", "12.3.4.5", "foo//bar"},
	}
	for _, c := range cases {
		addr, name := SplitAddressName(c.input)
		if addr != c.address {
			t.Errorf("%q: unexpected address: %q not %q", c.input, addr, c.address)
		}
		if name != c.name {
			t.Errorf("%q: unexpected name: %q not %q", c.input, name, c.name)
		}
	}
}

func TestJoinAddressName(t *testing.T) {
	cases := []struct {
		address, name, joined string
	}{
		{"", "", ""},
		{"", "a", "a"},
		{"", "/a", "/a"},
		{"", "//a", "//a"},
		{"", "///a", "///a"},
		{"/", "", ""},
		{"//", "", ""},
		{"/a", "", "/a"},
		{"//a", "", "/a"},
		{"aaa", "", "/aaa"},
		{"/aaa", "aa", "/aaa/aa"},
		{"ab", "/cd", "/ab/cd"},
		{"/ab", "/cd", "/ab/cd"},
		{"ab", "//cd", "/ab//cd"},
	}
	for _, c := range cases {
		joined := JoinAddressName(c.address, c.name)
		if joined != c.joined {
			t.Errorf("%q %q: unexpected join: %q not %q", c.address, c.name, joined, c.joined)
		}
	}
}

func TestJoin(t *testing.T) {
	cases := []struct {
		elems  []string
		joined string
	}{
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"", ""}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", ""}, "a"},
		{[]string{"a/"}, "a/"},
		{[]string{"a/", ""}, "a"},
		{[]string{"a", "/"}, "a"},
		{[]string{"", "a"}, "a"},
		{[]string{"", "/a"}, "a"},
		{[]string{"a", "b"}, "a/b"},
		{[]string{"a/", "b/"}, "a/b/"},
		{[]string{"a/", "/b"}, "a/b"},
		{[]string{"/a", "b"}, "/a/b"},
		{[]string{"a", "/", "b"}, "a/b"},
		{[]string{"a", "/", "/b"}, "a/b"},
		{[]string{"a/", "/", "/b"}, "a/b"},
		{[]string{"/a/b", "c"}, "/a/b/c"},
		{[]string{"/a", "b", "c"}, "/a/b/c"},
		{[]string{"/a/", "/b/", "/c/"}, "/a/b/c/"},
		{[]string{"a", "b", "c"}, "a/b/c"},
		{[]string{"a", "", "c"}, "a/c"},
		{[]string{"a", "", "", "c"}, "a/c"},
		{[]string{"/a/b", "c/d"}, "/a/b/c/d"},
		{[]string{"/a/b", "/c/d"}, "/a/b/c/d"},
		{[]string{"/a/b", "//c/d"}, "/a/b//c/d"},
		{[]string{"/a//", "c"}, "/a//c"},
		{[]string{"/a", "//"}, "/a//"},
		{[]string{"", "//a/b"}, "//a/b"},
		{[]string{"a", "b//"}, "a/b//"},
		{[]string{"a", "//", "b"}, "a//b"},
		{[]string{"a", "//", "/b"}, "a//b"},
		{[]string{"a", "//", "//b"}, "a//b"},
		{[]string{"a/", "//", "b"}, "a//b"},
		{[]string{"a//", "//", "b"}, "a//b"},
		{[]string{"a//", "//", "//b"}, "a//b"},
		{[]string{"a", "/", "/", "b"}, "a/b"},
		{[]string{"a/", "/", "/", "/b"}, "a/b"},
		{[]string{"a", "//", "//", "b"}, "a//b"},
		{[]string{"a//", "//", "//", "//b"}, "a//b"},
		{[]string{"a//", "//b//", "//c//"}, "a//b//c//"},
		{[]string{"a//", "", "//c//"}, "a//c//"},
		{[]string{"a///", "////b"}, "a//b"},
		{[]string{"////a", "b"}, "////a/b"},
		{[]string{"a", "b////"}, "a/b////"},
		{[]string{"/ep//", ""}, "/ep//"},
		{[]string{"/ep//", "a"}, "/ep//a"},
		{[]string{"/ep//", "//a"}, "/ep//a"},
	}
	for _, c := range cases {
		if got, want := Join(c.elems...), c.joined; want != got {
			t.Errorf("%q: unexpected join: %q not %q", c.elems, got, want)
		}
	}
}

func TestSplitJoin(t *testing.T) {
	cases := []struct {
		name, address, relative string
	}{
		{"/a/b", "a", "b"},
		{"/a//b", "a", "//b"},
		{"/a:10//b/c", "a:10", "//b/c"},
		{"/a:10/b//c", "a:10", "b//c"},
	}
	for _, c := range cases {
		a, r := SplitAddressName(c.name)
		if got, want := a, c.address; got != want {
			t.Errorf("%q: got %q, want %q", c.name, got, want)
		}
		if got, want := r, c.relative; got != want {
			t.Errorf("%q: got %q, want %q", c.name, got, want)
		}
		j := JoinAddressName(a, r)
		if got, want := j, c.name; got != want {
			t.Errorf("%q: got %q, want %q", c.name, got, want)
		}
	}
}

func TestTrimSuffix(t *testing.T) {
	cases := []struct {
		name, suffix, prefix string
	}{
		{"", "", ""},
		{"a", "", "a"},
		{"a", "a", ""},
		{"/a", "a", "/a"},
		{"a/b", "b", "a"},
		{"a/b", "/b", "a/b"},
		{"a/b/", "b/", "a"},
		{"/a/b", "b", "/a"},
		{"/a/b/c", "c", "/a/b"},
		{"/a/b/c/d", "c/d", "/a/b"},
		{"/a/b//c/d", "c/d", "/a/b//"},
		{"/a/b//c/d", "/c/d", "/a/b//c/d"},
		{"/a/b//c/d", "//c/d", "/a/b"},
		{"//a/b", "//a/b", ""},
		{"/a/b", "/a/b", ""},
		{"//a", "a", "//"},
	}
	for _, c := range cases {
		if p := TrimSuffix(c.name, c.suffix); p != c.prefix {
			t.Errorf("TrimSuffix(%q, %q): got %q, want %q", c.name, c.suffix, p, c.prefix)
		}
	}
}

func TestTerminal(t *testing.T) {
	ep := "/" + FormatEndpoint("tcp", "h:0")
	for _, c := range []string{
		"",
		"/",
		"/a",
		"//",
		"//a/b",
		ep + "",
		ep + "//",
		ep + "//a",
		ep + "//a/b",
	} {
		if !Terminal(c) {
			t.Errorf("Expected %q to be terminal", c)
		}
	}
	for _, c := range []string{
		"a",
		"/a/b",
		"a/b",
		"a//b",
		ep + "/a",
		ep + "/a//b",
	} {
		if Terminal(c) {
			t.Errorf("Expected %q to not be terminal", c)
		}
	}
}

func TestMakeTerminal(t *testing.T) {
	ep := "/" + FormatEndpoint("tcp", "h:0")
	type testcases struct {
		name   string
		result string
	}
	cases := []testcases{
		// relative names
		{"", ""},
		{"a", "//a"},
		{"a/b", "//a/b"},
		// rooted names
		{ep + "", ep + ""},
		{ep + "/", ep + ""},
		{ep + "//", ep + "//"},
		{ep + "///", ep + "//"},
		{ep + "/a", ep + "//a"},
		{ep + "//a", ep + "//a"},
		{ep + "///a", ep + "//a"},
		{ep + "/a/", ep + "//a/"},
		{ep + "/a/c", ep + "//a/c"},
		{ep + "/a//c", ep + "//a//c"},
		{ep + "/a/b//c", ep + "//a/b//c"},
		// corner cases
		{"/", ""},
		{"//", "//"},
		{"///", "//"},
		{"////", "//"},
		{"///" + ep + "/", "/" + ep + "/"},
		{"////" + ep + "/", "/" + ep + "/"},
	}
	for _, c := range cases {
		if got, want := MakeTerminal(c.name), c.result; want != got {
			t.Errorf("MakeTerminal(%q) : got %q, want %q", c.name, got, want)
		} else if !Terminal(got) {
			t.Errorf("%q is not terminal", got)
		}
	}
}

func TestMakeTerminalAtIndex(t *testing.T) {
	ep := "/" + FormatEndpoint("tcp", "h:0")
	_ = ep
	type testcases struct {
		name   string
		index  int
		result string
	}
	cases := []testcases{
		// examples from the comments:

		{"a/b/c", 1, "a//b/c"},
		{"a/b/c", -1, "a/b//c"},
		{"a/b/c/", 3, "a/b/c//"},
		{"a////b/c/", 3, "a////b/c//"},
		{"a/b///c", -1, "a/b//c"},
		{"a/b///c/", -1, "a/b///c//"},
		{"a/b///c", 1, "a//b///c"},
		{"", 0, ""},
		{"//a//b", 0, "//a//b"},
		{"", -1, ""},
		{"", 2, ""},
		{"", 1, ""},
		{"", -2, ""},
		{"//", 0, "//"},
		{"//", -1, "//"},
		{"//", 2, "//"},
		{"//", 1, "//"},
		{"//", -2, "//"},
		{"a", 0, "//a"},
		{"a/b", 1, "a//b"},
		{"a/b", 2, "a/b//"},
		{"a/b", 3, "a/b//"},
		{"a/b", -1, "a//b"},
		{"a/b", -2, "//a/b"},
		{"a/b", -3, "//a/b"},
		{"a/b//", -3, "//a/b//"},
		{"//a/b//", -3, "//a/b//"},
		{"a/b/c/d", -2, "a/b//c/d"},
		{"a/b/c/d", 1, "a//b/c/d"},
		{"a/b/c/d", 2, "a/b//c/d"},
		{"a/b/c/d", 3, "a/b/c//d"},
		{"a/b/c/d/", 4, "a/b/c/d//"},
		{"aa/bb", 1, "aa//bb"},
		{"aa/bb", 2, "aa/bb//"},
		{"aa/bb", 3, "aa/bb//"},
		{"aa/bb", -1, "aa//bb"},
		{"aa/bb", -2, "//aa/bb"},
		{"aa/bb", -3, "//aa/bb"},
		{"aa/bb//", -3, "//aa/bb//"},
		{"//aa/bb//", -3, "//aa/bb//"},
		{"aa/bb/cc/dd", -2, "aa/bb//cc/dd"},
		{"aa/bb/cc/dd", 1, "aa//bb/cc/dd"},
		{"aa/bb/cc/dd", 2, "aa/bb//cc/dd"},
		{"aa/bb/cc/dd", 3, "aa/bb/cc//dd"},
		{"aa/bb/cc/dd/", 4, "aa/bb/cc/dd//"},
		{ep + "//", 0, ep + "//"},
		{ep + "//", -1, ep + "//"},
		{ep + "//", 1, ep + "//"},
		{ep + "/a", 0, ep + "//a"},
		{ep + "/a/b", 0, ep + "//a/b"},
		{ep + "/a/b", 1, ep + "/a//b"},
		{ep + "/a/b", 2, ep + "/a/b//"},
		{ep + "/a/b", 100, ep + "/a/b//"},
		{ep + "/a/b/c", 0, ep + "//a/b/c"},
		{ep + "/a/b/c", 1, ep + "/a//b/c"},
		{ep + "/a/b/c", 2, ep + "/a/b//c"},
		{ep + "/a/b/c", 3, ep + "/a/b/c//"},
		{ep + "/a/b/c", -1, ep + "/a/b//c"},
		{ep + "/a/b/c", -2, ep + "/a//b/c"},
		{ep + "/a/b/c", -3, ep + "//a/b/c"},
		{ep + "/a/b/c", -4, ep + "//a/b/c"},
		{ep + "//a//b//c/", -1, ep + "//a//b//c//"},
		{ep + "//a//b//c//", -1, ep + "//a//b//c//"},
		{ep + "//a//b//c//", 1, ep + "//a//b//c//"},
		{ep + "//a//b//c//", 2, ep + "//a//b//c//"},
	}
	for _, c := range cases {
		if got, want := MakeTerminalAtIndex(c.name, c.index), c.result; want != got {
			t.Errorf("InstertTerminalAtIndex(%q, %d) : got %q want %q", c.name, c.index, got, want)
		}
	}
}

func TestRooted(t *testing.T) {
	ep := "/" + FormatEndpoint("tcp", "h:0")
	// should be rooted.
	cases := []string{
		"/",
		"/a",
		"/a/b",
		ep + "/",
	}
	for _, c := range cases {
		if !Rooted(c) {
			t.Errorf("Rooted(%q) return false, not true", c)
		}

	}
	cases = []string{
		"",
		"//a",
		"//b",
		"//" + ep,
	}
	for _, c := range cases {
		if Rooted(c) {
			t.Errorf("Rooted(%q) return true, not false", c)
		}

	}

}

func TestResolvable(t *testing.T) {
	ep := "/" + FormatEndpoint("tcp", "h:0")
	cases := []struct {
		name, notTerminal string
	}{
		{"", ""},
		{"/", "/"},
		{"//", ""},
		{"a", "a"},
		{"a/b", "a/b"},
		{"//a/b", "a/b"},
		{"///a/b", "a/b"},
		{"a//b", "a/b"},
		{"a//b//", "a/b//"},
		{"//a//b//", "a//b//"},
		{"//a//b///", "a//b///"},
		{ep + "//a", ep + "/a"},
		{ep + "//a/b", ep + "/a/b"},
		{ep + "//a///b", ep + "/a///b"},
		{ep + "///a//b", ep + "/a//b"},
		{ep + "///a//b//", ep + "/a//b//"},
	}
	for _, c := range cases {
		if got, want := MakeResolvable(c.name), c.notTerminal; want != got {
			t.Errorf("MakeResolvable(%q): unexpected not terminal: %q not %q", c.name, got, want)
		} else if !(got == "" || got == "/") && Terminal(got) {
			t.Errorf("MakeResolvable(%q): expected to be resolveable: got %q", c.name, got)
		}
	}
}
