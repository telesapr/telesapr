package main

import (
	"slices"
	"testing"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args     []string
		files    []string
		renderer string
		script   string
		sargs    []string
		agent    bool
	}{
		{args: []string{"a.tlspr"}, files: []string{"a.tlspr"}},
		{args: []string{"--agent", "a.tlspr"}, files: []string{"a.tlspr"}, agent: true},
		{args: []string{"a.tlspr", "--script", "s.go", "--agent"}, files: []string{"a.tlspr"},
			script: "s.go", sargs: []string{"--agent"}},
		{args: []string{"--renderer", "software", "a.tlspr"}, files: []string{"a.tlspr"}, renderer: "software"},
		{args: []string{"--renderer=opengl"}, renderer: "opengl"},
		{args: []string{"--renderer=vulkan", "a.tlspr"}, files: []string{"a.tlspr"}},
		{args: []string{"--renderer"}},
		{args: []string{"a.tlspr", "--script", "s.go", "--renderer", "x"}, files: []string{"a.tlspr"},
			script: "s.go", sargs: []string{"--renderer", "x"}},
		{args: []string{"--script"}},
	}
	for _, c := range cases {
		o := parseArgs(c.args)
		if !slices.Equal(o.files, c.files) || o.renderer != c.renderer || o.scriptFile != c.script ||
			!slices.Equal(o.scriptArgs, c.sargs) || o.agent != c.agent {
			t.Errorf("parseArgs(%q) = %+v", c.args, o)
		}
	}
}
