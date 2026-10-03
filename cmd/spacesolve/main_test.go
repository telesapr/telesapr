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
		{args: []string{"a.spcs"}, files: []string{"a.spcs"}},
		{args: []string{"--agent", "a.spcs"}, files: []string{"a.spcs"}, agent: true},
		{args: []string{"a.spcs", "--script", "s.go", "--agent"}, files: []string{"a.spcs"},
			script: "s.go", sargs: []string{"--agent"}},
		{args: []string{"--renderer", "software", "a.spcs"}, files: []string{"a.spcs"}, renderer: "software"},
		{args: []string{"--renderer=opengl"}, renderer: "opengl"},
		{args: []string{"--renderer=vulkan", "a.spcs"}, files: []string{"a.spcs"}},
		{args: []string{"--renderer"}},
		{args: []string{"a.spcs", "--script", "s.go", "--renderer", "x"}, files: []string{"a.spcs"},
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
