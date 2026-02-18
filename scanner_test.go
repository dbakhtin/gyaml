package gyaml

import (
	"encoding/json"
	"testing"
)

func TestValidCustom(t *testing.T) {
	t.Run("handpicked tests", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v: [a: b, c: d]", true},
			// {"'v", false},
			// {"\"v", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidNumbers(t *testing.T) {
	t.Run("numbers", func(t *testing.T) {
		const covid = false
		tests := []struct {
			data string
			ok   bool
		}{
			{"1", true},
			{"1_2", true},
			{"1.2", true},
			{"1.2_3", true},
			{"-1.2", true},
			{"+1.2", true},
			{"-1.2e2", true},
			{"1.2e+2", true},
			{"1.2e-2", true},
			{"0.2", true},
			{".2", true},
			{"0b01", true},
			{"0b01_01", true},
			{"-0b01", true},
			{"0b02", false},
			{"0o17", true},
			{"0o17_24", true},
			{"0o18", false},
			{"0x2f", true},
			{"0x2f_3a", true},
			{"0x2F", true},
			{"0x5G", covid},
			{".inf", true},
			{"-.inf", true},
			{"+.inf", true},
			{".Inf", true},
			{".INF", true},
			{".inF", false},
			{"-.inF", false},
			{"+.inF", false},
			{".iNf", false},
			{".nan", true},
			{".Nan", true},
			{".NAN", true},
			{".nAN", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidUnquoted(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v", true},
			{"-v", true},
			{"v u", true},
			{" v u", true},
			{"&a", true},
			{"*a", true},
			{"+a", true},
			{"<<:", true},
			{"<< :", true},
			{"<<", false},
			{"<<a", false},
			{"!a", false},
			{"!!a", true},
			{"!!float 2.3", true},
			{"1.2.3", true},
			{"0B01", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("multiline", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"|\na", false},
			{"|\n a", true},
			{"|+\n a", true},
			{"|-\n a", true},
			{">\n a", true},
			{">+\n a", true},
			{">-\n a", true},
			{"|\n a\n b", true},
			{"|\n a\n\n b", true},
			{"|\n a\n b\n  c", true},
			{"|\n a\n b\nc", false},
			{"v: |\na", false},
			{"v: |\n a", true},
			{"v: |\n <div class=\"cl\">\n  <p>Hello</p>\n </div>", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("time", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"2015-01-01", true},
			{"2015-02-24T18:19:39.12Z", true},
			{"2015-2-3T3:4:5Z", true},
			{"2015-02-24t18:19:39Z", true},
			{"2015-02-24 18:19:39", true},
			{"60s", true},
			{"-0.5h", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidQuoted(t *testing.T) {
	t.Run("double-quoted", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"\"v\"", true},
			{"\"v", false},
			{"v\"", true},
			{"v\"u", true},
			{`'"v"'`, true},
			{"'v", false},
			// {`''''`, true},
			{`' 1\n  2'`, true},
			{"a\x2Fb", true},
			{"a\u002F", true},
			{"a\U0000002F", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("single-quoted", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"'v'", true},
			{"'v", false},
			{"v'", true},
			{"v'u", true},
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
	t.Run("simple", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v:\ns", false},
			{"v:", true},
			{"v : \n s", true},
			{"v: 1\nu: 2", true},
			{"v: \nu:", true},
			{"v: 1.1\nu: 2.2e-5", true},
			{"v: a\nu: b", true},
			{"v: \"a\"\nu: \"b\"", true},
			{"\"v: 1", false},
			{"v: \"u", false},
			{"v: 1\n u: 2", false},
			{"v: a: b", false},
			{"v: 1: b", false},
			{"v: 1\nu:2", false},
			{"v:1\nu: 2", false},
			{"1: 2\n3: 4", true},
			{"1 : 2", true},
			{"a : b", true},
			{"a:: b\nc: d", true}, // parsed as "a:": b, c:d
			{"1:\n2", false},
			{"1.1:\n 2.2", true},
			{"1.1e-2:\n 2.2e+2", true},
			{"1.1e-2:\n 2.2e+ 2", false},
			{"v: !!float 2.3", true},
			{"v: 2015-01-01", true},
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
			{"v:\n  u: 2", true},
			{"v:\n  a: b", true},
			{"\"v\":\n  \"a\": \"b\"", true},
			{"v:\n  a: 2.2", true},
			{"1:\n  2: 2.2", true},
			{"v:\n  u:\n   t: 2", true},
			{"v:\n  u:\n   t:\n    2", true},
			{"v:\n  u:\n  2", false},
			{"v:\n  u:\n   t:\n   2", false},
			{"v:\n u: c\nt: b", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("nested arrays", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v:\n- a\n- b", true},
			{"v:\n - a\n - b", true},
			{"v:\n - a\n- b", false},
			{"v:\n - a\n  - b", false},
			{"v:\n- a: b\n- c: d", true},
			{"v:\n u:\n  - 1\n - 2", false},
			{"v:\n- a:\n  - b\n- c: d", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("multi-level nested", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v:\n a:\n  b:\n  - 1\nu: 2", true},
			// {"v:\n- a:\n  - b\n- c: d", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
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
			{"- a: b\n- 1", true},
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
		{`}{`, false},
		{`{]`, false},
		{`{}`, true},
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
			{"a:\n {foo: bar}", true},
			{"- [a, b]\n- [c, d]", true},
			{"v:\n- [a, b]\n- [c, d]", true},
			{"v: [a: b, c: d]", true},
			{"v: [{a: b}, {c: d, e: f}]", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidDocumentSeparator(t *testing.T) {
	t.Run("separator", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"1\n---\n2", true},
			{"1\n ---\n2", false},
			{"1\n--- \n2", false},
			{"1\n \n---\n2", true},
			{"- 1\n---\n- 2", true},
			{"v:\n- 1\n---\n- 2", true},
			{"v:\n - 1\n---\n- 2", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("end document", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"...", true},
			{"...\n2", true},
			{"1\n...\n2", true},
			{"1\n ...\n2", false},
			{"1\n... \n2", false},
			{"1\n \n...\n2", true},
			{"- 1\n...\n- 2\nanything", true},
			{"- 1\nanything\n...\n- 2", false},
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
