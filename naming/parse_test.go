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

func TestJoinAddressNameFixed(t *testing.T) {
	cases := []struct {
		address, name, joined string
	}{
		{"", "", ""},
		{"", "a", "//a"},
		{"", "/a", "//a"},
		{"", "//a", "//a"},
		{"", "///a", "//a"},
		{"/", "", ""},
		{"//", "", ""},
		{"/a", "", "/a"},
		{"//a", "", "/a"},
		{"aaa", "", "/aaa"},
		{"/aaa", "aa", "/aaa//aa"},
		{"ab", "/cd", "/ab//cd"},
		{"/ab", "/cd", "/ab//cd"},
		{"ab", "//cd", "/ab//cd"},
	}
	for _, c := range cases {
		if got, want := JoinAddressNameFixed(c.address, c.name), c.joined; want != got {
			t.Errorf("%q %q: unexpected join: %q not %q", c.address, c.name, got, want)
		}
	}
}

func TestJoin(t *testing.T) {
	cases := []struct {
		name, suffix, joined string
	}{
		{"a", "b", "a/b"},
		{"", "b", "b"},
		{"a", "", "a"},
		{"/a", "b", "/a/b"},
		{"/a/b", "c", "/a/b/c"},
		{"/a/b", "c/d", "/a/b/c/d"},
		{"/a/b", "/c/d", "/a/b/c/d"},
		{"/a/b", "//c/d", "/a/b//c/d"},
		{"", "//a/b", "//a/b"},
	}
	for _, c := range cases {
		if got, want := Join(c.name, c.suffix), c.joined; want != got {
			t.Errorf("%q %q: unexpected join: %q not %q", c.name, c.suffix, got, want)
		}
	}
}

func TestDepth(t *testing.T) {
	cases := []struct {
		name  string
		depth int
	}{
		{"", 0},
		{"foo", 1},
		{"foo/", 1},
		{"foo/bar", 2},
		{"foo//bar", 2},
		{"/foo/bar", 2},
		{"//", 0},
		{"//foo//bar", 2},
		{"/foo/bar//baz//baf/", 4},
	}
	for _, c := range cases {
		if got, want := Depth(c.name), c.depth; want != got {
			t.Errorf("%q: unexpected depth: %d not %d", c.name, got, want)
		}
	}
}

func TestFixed(t *testing.T) {
	cases := []struct {
		name, fixed string
	}{
		{"a", "//a"},
		{"a/b", "//a/b"},
		{"/a/b", "/a//b"},
		{"/a//b", "/a//b"},
		{"/a//b/c", "/a//b/c"},
		{"/a//b//c", "/a//b//c"},
		{"/a", "/a"},
		{"//a", "//a"},
	}
	for _, c := range cases {
		if got, want := MakeFixed(c.name), c.fixed; want != got {
			t.Errorf("%q: unexpected fixed: %q not %q", c.name, got, want)
		} else if !Fixed(got) {
			t.Errorf("%q: expected to be fixed", got)
		}
	}
}

func TestNotFixed(t *testing.T) {
	cases := []struct {
		name, notFixed string
	}{
		{"a", "a"},
		{"a/b", "a/b"},
		{"/a/b", "/a/b"},
		{"//a", "a"},
		{"//a/b", "a/b"},
		{"/a//b", "/a/b"},
		{"//a//b", "a//b"},
		{"a//b", "a//b"},
	}
	for _, c := range cases {
		if got, want := MakeNotFixed(c.name), c.notFixed; want != got {
			t.Errorf("%q: unexpected not fixed: %q not %q", c.name, got, want)
		} else if Fixed(got) {
			t.Errorf("%q: expected to be not fixed", got)
		}
	}
}

func TestStripSuffix(t *testing.T) {
	cases := []struct {
		name, suffix, stripped string
	}{
		{"a/b", "b", "a"},
		{"/a/b", "b", "/a"},
		{"/a/b", "/b", "/a"},
		{"/a/b", "", "/a/b"},
		{"/a/b//", "", "/a/b"},
		{"a", "a", ""},
		{"/a", "a", ""},
		{"/a//b", "b", "/a"},
		{"/a//b", "/b", "/a"},
		{"/a//b", "//b", "/a"},
		{"/a//b", "///b", "/a//b"},
		{"/a/b//c", "b//c", "/a"},
		{"/a/b//c", "b/c", "/a/b//c"},
		{"/yes/we/can", "we/can", "/yes"},
		{"/yes/we/can", "e/can", "/yes/we/can"},
		{"/yes/we/can", "yes/we/can", ""},
		{"/yes/we/can", "/yes/we/can", ""},
		{"/yes/we/can", "/yes/we/can", ""},
		{"/yes/we/can", "/yes/we/ca", "/yes/we/can"},
	}
	for _, c := range cases {
		if want, got := c.stripped, StripSuffix(c.name, c.suffix); want != got {
			t.Errorf("StripSuffix(%q, %q): want %q, got %q", c.name, c.suffix, want, got)
		}
	}
}
