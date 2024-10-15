package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestViewport(t *testing.T) {
	tests := map[string]struct {
		s             string
		x, y          int
		width, height int
		want          string
	}{
		"empty": {},
		"simple": {
			s:      "1. one\n2. two\n3. three",
			x:      3,
			y:      1,
			width:  5,
			height: 2,
			want:   "two\nthree",
		},
		"colors": {
			s: ("\x1b[31m1. one\x1b[0m\n" +
				"\x1b[34m2. two\x1b[0m\n" +
				"\x1b[32m3. three\x1b[0m"),
			x:      3,
			y:      1,
			width:  5,
			height: 2,
			want:   "\x1b[34mtwo\x1b[0m\n\x1b[32mthree\x1b[0m",
		},
		"hyperlink": {
			s:      "\x1b]8;;http://example.com\x1b\\Example Domain\x1b]8;;\x1b\\",
			x:      8,
			y:      0,
			width:  6,
			height: 1,
			want:   "\x1b]8;;http://example.com\x1b\\Domain\x1b]8;;\x1b\\",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := parseText(test.s).viewport(test.x, test.y, test.width, test.height)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("viewport(...) has diff(-want +got):\n%s", diff)
			}
		})
	}
}
