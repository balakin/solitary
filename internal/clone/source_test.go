package clone

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		arg     string
		url     string
		path    string
		display string
		name    string
	}{
		{
			arg:     "balakin/nvim-cell",
			url:     "https://github.com/balakin/nvim-cell.git",
			display: "github.com/balakin/nvim-cell",
			name:    "nvim-cell",
		},
		{
			arg:     "balakin/cells/claude",
			url:     "https://github.com/balakin/cells.git",
			path:    "claude",
			display: "github.com/balakin/cells",
			name:    "claude",
		},
		{
			arg:     "balakin/cells#claude",
			url:     "https://github.com/balakin/cells.git",
			path:    "claude",
			display: "github.com/balakin/cells",
			name:    "claude",
		},
		{
			arg:     "balakin/cells/agents/claude",
			url:     "https://github.com/balakin/cells.git",
			path:    "agents/claude",
			display: "github.com/balakin/cells",
			name:    "claude",
		},
		{
			arg:     "https://gitlab.com/me/cells.git#rust",
			url:     "https://gitlab.com/me/cells.git",
			path:    "rust",
			display: "https://gitlab.com/me/cells.git",
			name:    "rust",
		},
		{
			arg:     "https://gitlab.com/me/cells.git",
			url:     "https://gitlab.com/me/cells.git",
			display: "https://gitlab.com/me/cells.git",
			name:    "cells",
		},
		{
			arg:     "git@github.com:me/cells.git",
			url:     "git@github.com:me/cells.git",
			display: "git@github.com:me/cells.git",
			name:    "cells",
		},
		{
			arg:     "../my-cells#claude",
			url:     "../my-cells",
			path:    "claude",
			display: "../my-cells",
			name:    "claude",
		},
		{
			arg:     "/srv/cells/",
			url:     "/srv/cells/",
			display: "/srv/cells/",
			name:    "cells",
		},
	}

	for _, tc := range cases {
		t.Run(tc.arg, func(t *testing.T) {
			source, err := Parse(tc.arg)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.arg, err)
			}
			if source.URL != tc.url {
				t.Errorf("URL = %q, want %q", source.URL, tc.url)
			}
			if source.Path != tc.path {
				t.Errorf("Path = %q, want %q", source.Path, tc.path)
			}
			if source.Display != tc.display {
				t.Errorf("Display = %q, want %q", source.Display, tc.display)
			}
			if got := source.DefaultName(); got != tc.name {
				t.Errorf("DefaultName() = %q, want %q", got, tc.name)
			}
		})
	}
}

func TestParseRefuses(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"one segment":      "cells",
		"no repository":    "#claude",
		"escaping path":    "me/cells#../../etc",
		"named twice":      "me/cells/claude#rust",
		"trailing nothing": "me/",
	}

	for what, arg := range cases {
		t.Run(what, func(t *testing.T) {
			if source, err := Parse(arg); err == nil {
				t.Errorf("Parse(%q) accepted it as %+v", arg, source)
			}
		})
	}
}
