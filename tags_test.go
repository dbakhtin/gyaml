package gyaml

import "testing"

func TestTagParsing(t *testing.T) {
	name, opts := parseTag("field,foobar,foo")
	if name != "field" {
		t.Fatalf("name = %q, want field", name)
	}
	for _, tt := range []struct {
		opt  string
		want bool
	}{
		{"foobar", true},
		{"foo", true},
		{"bar", false},
	} {
		if opts.Contains(tt.opt) != tt.want {
			t.Errorf("Contains(%q) = %v, want %v", tt.opt, !tt.want, tt.want)
		}
	}
}

func TestTagParsingWithPrefix(t *testing.T) {
	_, opts := parseTag("field,foobar,foo,anchor,anchor2=abc")
	for _, tt := range []struct {
		opt  string
		want bool
	}{
		{"foobar", true},
		{"foo", true},
		{"bar", false},
		{"anchor", true},
		{"anchor2", true},
	} {
		if opts.ContainsWithPrefix(tt.opt) != tt.want {
			t.Errorf("Contains(%q) = %v, want %v", tt.opt, !tt.want, tt.want)
		}
	}
}
