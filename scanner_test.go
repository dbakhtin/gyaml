package gyaml

import (
	"testing"
)

// a test for debugging
func TestValidCustom(t *testing.T) {
	t.Run("handpicked tests", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"\"\":6", true},                 //strictly speaking this should be true only in flow mode, but probably let it be
			{"0:\r- \"\":6 \x7f\xff ", true}, //same thing
			{"0:\r- '0'\r- '$- ''\r- ':", true},
			{"0:\r- - }\r- - 0", false},
			{"0:\r- \r.", false},
			{"0:\r-   \r--\x98\x98", false},
			{"0:\r- 0b0:X\xfe, ", true},
			//
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
		tests := []struct {
			data string
			ok   bool
		}{
			{"1", true},
			{"1.2", true},
			{"-1.2", true},
			{"+1.2", true},
			{"-1.2e2", true},
			{"1.2e+2", true},
			{"1.2e-2", true},
			{"0.2", true},
			{".2", true},
			{"0b01", true},
			{"-0b01", true},
			{"0o17", true},
			{"0x2f", true},
			{".inf", true},
			{"-.inf", true},
			{"+.inf", true},
			{".nan", true},
			{".Nan", true},
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
			{"<foo>", true},
			{"<<:", false}, //merge not supported
			{"<< :", false},
			{"<<", false},
			{"<<a", false},
			{"!a", false},
			{"!!a", true},
			{"!!float 2.3", true},
			{"1.2.3", true},
			{"0B01", true},
			{"50cent_of_dollar", true},
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
			{"v:\n- A\n- |-\n  B\n  C\n", true},
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
			{"\"a\x2Fb\"", true},
			{"\"a\u002F\"", true},
			{"\"a\U0000002F\"", true},
			{`"\" \a \b \e \f"`, true},
			{`"\n \r \t \v \0"`, true},
			{`"\  \_ \N \L \P"`, true},
			{"\"A \\\n \x41 \u0041 \U00000041\"", true},
			{`"\  \_ \N \L \P \
\x41 \u0041 \U00000041"`, true},
			{"\"a\\\r\nb\\\r\nc\"", true},
			{"\"a\\\nb\\\nc\"", true},
			{"\"a\\\rb\\\rc\"", true},
			{"\"   1\n    2\n    3\"", false},
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
			// {"'v", false}, //uncatchable by scanner atm, decoder should though
			{"v'", true},
			{"v'u", true},
			{`'"v"'`, true},
			{`''''`, true},
			{`'''2'''`, true},
			{`'B''z'`, true},
			{"'   1\n    2\n    3'", true},
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
			{"v: a, b", true},
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
			{"1.1e-2:\n 2.2e+ 2", true}, //incorrect numbers are parsed as strings
			{"v: !!float 2.3", true},
			{"v: 2015-01-01", true},
			{"a: b\rc: d", true},
			{"v: /a/{b}", true},
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
			{"a: 1\nsub:\n  e: 5\n", true},
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
			{"tags:\n- hello-world\na: foo", true},
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
			{"- a:\n    b:\n- c: d", true}, //b is null here
			{"- a: [2 , 2]      ", true},   //with tabs in the end
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
			{"{a: , b: c}", true},
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
			// {"v: [a: b, c: d]", true}, //unsupported atm
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
			{"1\n--- \n2", true},
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
			{"1\n... \n2", true},
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

func TestValidAnchor(t *testing.T) {
	t.Run("scalars", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"a: &x 1\nc: *x", true},
			{"a: & 1\nc: *x", false},
			{"a: &x 1\nc: *", false},
			{"a: &- 1\nc: *-", true},
			{"a: &: 1\nc: *x", false},
			{"a: &x 1\nc: *:", false},
			{"a: &a {c: 1}\nb: *a\n", true},
			{"{a: &a c, *a : b}", true},
			{"a: &a 1 # comment\n*a: 2 # comment2\n", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestValidComment(t *testing.T) {
	t.Run("scalars", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"# comment", true},
			{"#comment", true},
			{"#", true},
			{"--- #comment", true},
			{"... #comment", true},
			{"| #comment\n abc", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
	t.Run("maps", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"v: 1\n# comment\nu: 2", true},
			{"v:\n  a: 1\n# comment\nu: 2", true},
			{"v:\n  a: 1\n # comment\nu: 2", true},
			{"'v': '1' # comment", true},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestFailedfuzz(t *testing.T) {
	t.Run("tests that failed fuzz_test.go", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"{a: &a c, *a : b}", true},
			{"- : ", false},
			{" : ", false},
			{"-1: \r - : ", false},
			{"v: [{a: b}, {c: d, e: f}]", true},
			//---
			{"-1: - : - ", false},
			{"0: 0:", false},
			{"v: a: b", false},
			{"0: 0:", false},
			{"-1: - : - ", false},
			{"&0 }", false},
			{"0:\n 0: ", true},
			{" \"\":", true},
			{"{'", false},
			{"&00 :", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}

func TestDirectives(t *testing.T) {
	t.Run("directives", func(t *testing.T) {
		tests := []struct {
			data string
			ok   bool
		}{
			{"%YAML 1.2\n---\n", true},
			{"%YAML 1.2", true},
			{" %YAML 1.2", false},
			{"%AnY 1 #beny", true},
			{"%YAML 1.2\n---\n", true},
			{"%YAML 1.2\n ---\n", false},
			{"%YAML 1.2\nany", false},
		}
		for _, tt := range tests {
			if ok := Valid([]byte(tt.data)); ok != tt.ok {
				t.Errorf("Valid(%q) = %v, want %v", tt.data, ok, tt.ok)
			}
		}
	})
}
