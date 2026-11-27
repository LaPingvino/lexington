package main

import (
	"runtime/debug"
	"testing"
)

// The build flags win; without them, go install …@vX says vX and a checkout
// its commit.
func TestBuildVersion(t *testing.T) {
	bi := &debug.BuildInfo{Main: debug.Module{Version: "v1.67.0"}, Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.time", Value: "2026-11-27T10:00:00Z"}}}
	for _, c := range []struct {
		name        string
		bi          *debug.BuildInfo
		flags, want [3]string
	}{
		{"flags", bi, [3]string{"v1.68.0", "def456", "2026-11-28"}, [3]string{"v1.68.0", "def456", "2026-11-28"}},
		{"go install", bi, [3]string{}, [3]string{"v1.67.0", "abc123", "2026-11-27T10:00:00Z"}},
		{"checkout", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, [3]string{}, [3]string{"dev", "unknown", "unknown"}},
		{"nothing", nil, [3]string{}, [3]string{"dev", "unknown", "unknown"}},
	} {
		v, cm, d := buildVersion(c.bi, c.flags[0], c.flags[1], c.flags[2])
		if got := [3]string{v, cm, d}; got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
