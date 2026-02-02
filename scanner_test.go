package gyaml

import (
	"encoding/json"
	"testing"
)

func TestValidUnquoted(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v: s", true},
			{"v:\ns", false},
			{"v:\n s", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("numbers", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"1:\n  2", true},
			{"1:\n2", false},
			{"1:\n 2", true},
			{"1.1:\n 2.2", true},
			{"1.1e-2:\n 2.2e+2", true},
			{"1.1e-2:\n 2.2e+ 2", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("bools", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"true:\n  false", true},
			{"false:\nfalse", false},
			{"false:\n true", true},
			{"FALSE:\n TRUE", true},
			{"False:\n True", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("null", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"null", true},
			{"null:\nnull", false},
			{"null:\n 1", true},
			{"1:\n null", true}, //partial bools scanned as unquoted strings
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("mixed", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"true:\n  false", true},
			{"false:\nfalse", false},
			{"true:\n true", true},
			{"true:\n tru", true}, //partial bools scanned as unquoted strings
			{"fals:\n tr", true},
			{"fale:\n tri", true}, //incorrect bools parsed as unquoted strings
			{"fa:\n t", true},
			{"nul:\n t", true},
			{"nu:\n t", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidQuoted(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"\"v\":\n  \"s\"", true},
			{"\"v\":\n\"s\"", false},
			{"\"v\":\n \"s\"", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidMap2(t *testing.T) {
	t.Run("with nesting arrays", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			// {"v:\n- a:\n  - b\n- c: d", true},
			{"v:\n - a\n- b", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}
func TestValidMap(t *testing.T) {
	//Worked
	t.Run("non-nesting", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v: 1\nu: 2", true},
			{"v: 1.1\nu: 2.2e-5", true},
			{"v: a\nu: b", true},
			{"v: \"a\"\nu: \"b\"", true},
			{"v: 1\n u: 2", false},
			{"v: a: b", false},
			{"v: 1: b", false},
			{"v: 1\nu:2", false},
			{"v:1\nu: 2", false},
			{"1: 2\n3: 4", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("with nesting", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v:\n  u: 2", true},
			{"v:\n  a: b", true},
			{"\"v\":\n  \"a\": \"b\"", true},
			{"v:\n  a: 2.2", true},
			{"1:\n  2: 2.2", true},
			{"v:\n  u:\n   t: 2", true},
			{"v:\n  u:\n   t:\n    2", true},
			{"v:\n  u:\n  2", false},
			{"v:\n  u:\n   t:\n   2", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("with nesting arrays", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v:\n- a\n- b", true},
			{"v:\n - a\n - b", true},
			{"v:\n - a\n- b", false},
			{"v:\n - a\n  - b", false},
			{"v:\n- a: b\n- c: d", true},
			// {"v:\n- a:\n  - b\n- c: d", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidSlice2(t *testing.T) {
	tests := []struct {
		data string
		ok   bool
	}{
		{"- a: b\n  c: d", true},
	}
	for _, tt := range tests {
		if ok := Valid([]byte(tt.data)); ok != tt.ok {
			t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
		}
	}
}

func TestValidSlice(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"- 1", true},
			{"- 1\n", true},
			{"- 1\n- s", true},
			{"- 1\n2", false},
			{"- 1\n-2", false},
			{"- 1\n -2", false},
			{"- 1\n  -2", false},
			{"- 1\n  2", false},
			{"- 1\n   2", false},
			{"- 1\n- 2", true},
			{"- a\n- b", true},
			{"- true\n- false", true},
			{"- null\n- \"c\"", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})

	t.Run("nested", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"- a: b\n- c: d", true},
			{"- a: b\n-c: d", false},
			{"- a: b\n\"-c\": d", false},
			{"\"-a\": b\n\"-c\": d", true},
			{"- - 1\n  - 2\n- - 3", true},
			{"- - a: b\n  - c: d\n- - e: f", true},
			{"- a: b\n  c: d", true},
			{"- - a: b\n    c: d", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValid(t *testing.T) {
	tests := []struct {
		data string
		ok   bool
	}{
		{"1", true},
		{"false", true},
		{"foo", true},
		{"foo bar", true},
		{"foo bar:", true},
		{" foo bar", true},
		{"foo bar\n", true},
		{`"foo"`, true},
		{`}{`, false},
		{`{]`, false},
		{`{}`, true},
		{"v:\n  s", true},
		{"v:\ns", false},
		{"v:\n s", true},
		{"v:\n  s", true},
		{"f:\n  b", true},
		{"foo:\n  bar", true},
		{`foo:\n bar`, true},
	}
	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		})
	}
}

func TestValidFlow(t *testing.T) {
	t.Run("flow global", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{`{foo: bar}`, true},
			{`{"foo": "bar"}`, true},
			{`{"foo":"bar"}`, true},
			{`{"foo":}`, true},
			{`{"foo"}`, false},
			{`{"foo":"bar", "bar": "baz"}`, true},
			{`{fo: ba, re: {mo: [qu]}}`, true},
			{`{fo: ba, re:{mo:[qu]}}`, true},
			{`{fo: ba, re:{mo:[qu,no]}}`, true},
			{`{fo: ba, re{mo: [qu]}}`, false},
			{`{fo: ba, re:{mo: [qu, no]}}`, true},
			{`{fo:ba, re:{mo:[qu]}}`, false},
			{`{"foo": "bar","bar": {"baz": ["qux"]}}`, true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("flow local", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{`{foo: bar}`, true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidJson(t *testing.T) {
	tests := []struct {
		data string
		ok   bool
	}{
		{"1", true},
	}
	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			if ok := json.Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		})
	}
}
