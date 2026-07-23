package version

import "testing"

func TestIsRelease(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v1.7.0", true},
		{"v0.0.0", true},
		{"v1.10.3", true},
		{"dev", false},
		{"(devel)", false},
		{"", false},
		{"1.7.0", false},        // missing the v
		{"v1.7", false},         // two components
		{"v1.7.0.1", false},     // four components
		{"v1.7.0-rc1", false},   // prerelease
		{"v1.7.0+dirty", false}, // build metadata
		{"v1.6.1-0.20260723143812-50d39f89c2fe", false}, // pseudo-version (go build, Go 1.24+)
		{"v1.7.x", false},
		{"v1.+7.0", false}, // Atoi would accept "+7": must be rejected
		{"v1..0", false},
		{"v-1.7.0", false},
	}
	for _, c := range cases {
		if got := IsRelease(c.in); got != c.want {
			t.Errorf("IsRelease(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		name            string
		current, latest string
		want            bool
	}{
		{"patch", "v1.7.0", "v1.7.1", true},
		{"minor", "v1.7.9", "v1.8.0", true},
		{"major", "v1.9.9", "v2.0.0", true},
		{"equal", "v1.7.0", "v1.7.0", false},
		{"older", "v1.8.0", "v1.7.0", false},
		{"numeric not lexicographic", "v1.9.0", "v1.10.0", true},
		{"numeric not lexicographic patch", "v1.0.9", "v1.0.10", true},
		{"current not a release", "dev", "v1.8.0", false},
		{"current is a pseudo-version", "v1.6.1-0.20260723143812-50d39f8", "v1.8.0", false},
		{"latest not a release", "v1.7.0", "garbage", false},
		{"both empty", "", "", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("%s: Newer(%q, %q) = %v, want %v", c.name, c.current, c.latest, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name             string
		ldflags, mainVer string
		want             string
	}{
		{"ldflags wins", "v1.8.0", "v1.7.0", "v1.8.0"},
		{"build info when no ldflags", "", "v1.7.0", "v1.7.0"},
		{"pseudo-version passes through", "", "v1.6.1-0.2026-abc", "v1.6.1-0.2026-abc"},
		{"nothing available", "", "", Dev},
	}
	for _, c := range cases {
		if got := Resolve(c.ldflags, c.mainVer); got != c.want {
			t.Errorf("%s: Resolve(%q, %q) = %q, want %q", c.name, c.ldflags, c.mainVer, got, c.want)
		}
	}
}
