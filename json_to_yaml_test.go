package gyaml_test

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/denisbakhtin/gyaml"
	"github.com/denisbakhtin/gyaml/json"
)

var zero = 0
var emptyStr = ""

type TestTextMarshaler string

func (t TestTextMarshaler) MarshalText() ([]byte, error) {
	return []byte(t), nil
}

type TestTextUnmarshalerContainer struct {
	V TestTextMarshaler
}

func ptr[T any](v T) *T {
	return &v
}

func TestEncodeOne(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		{
			"v: 30s\n",
			map[string]time.Duration{"v": 30 * time.Second},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			b := bytes.Buffer{}
			err := gyaml.NewEncoder(&b).Encode(test.value)
			if err != nil {
				t.Error(err)
			}
			got := b.String()
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeSimple(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		{
			"one: one value\n",
			map[string]string{"one": "one value"},
			nil,
		},

		{
			"v: \"true\"\n",
			map[string]string{"v": "true"},
			nil,
		},
		{
			"v: \"false\"\n",
			map[string]string{"v": "false"},
			nil,
		},
		{
			"v: true\n",
			map[string]any{"v": true},
			nil,
		},
		{
			"v: false\n",
			map[string]bool{"v": false},
			nil,
		},
		{
			"v: 10\n",
			map[string]int{"v": 10},
			nil,
		},
		{
			"v: -10\n",
			map[string]int{"v": -10},
			nil,
		},
		{
			"v: 4294967296\n",
			map[string]int64{"v": int64(4294967296)},
			nil,
		},
		{
			"v: 0.1\n",
			map[string]any{"v": 0.1},
			nil,
		},
		{
			"v: 0.99\n",
			map[string]float32{"v": 0.99},
			nil,
		},
		{
			"v: 1e-7\n",
			map[string]float32{"v": 1e-07},
			nil,
		},
		{
			"v: 1e-7\n",
			map[string]float64{"v": 0.0000001},
			nil,
		},
		{
			"v: 0.123456789\n",
			map[string]float64{"v": 0.123456789},
			nil,
		},
		{
			"v: -0.1\n",
			map[string]float64{"v": -0.1},
			nil,
		},
		{
			"v: 1\n",
			map[string]float64{"v": 1.0},
			nil,
		},
		{
			"v: 1000000\n",
			map[string]float64{"v": 1000000},
			nil,
		},
		/*
			{
				"v: .inf\n",
				map[string]any{"v": math.Inf(0)},
				nil,
			},
			{
				"v: -.inf\n",
				map[string]interface{}{"v": math.Inf(-1)},
				nil,
			},
			{
				"v: .nan\n",
				map[string]interface{}{"v": math.NaN()},
				nil,
			},
		*/
		{
			"v: null\n",
			map[string]interface{}{"v": nil},
			nil,
		},
		{
			"v: \"\"\n",
			map[string]string{"v": ""},
			nil,
		},
		{
			"a: \"-\"\n",
			map[string]string{"a": "-"},
			nil,
		},
		{
			"1: v\n",
			map[int]string{1: "v"},
			nil,
		},
		/*
				{
					"1.1: v\n",
					map[float64]string{1.1: "v"},
					nil,
				},

			{
				"true: v\n",
				map[bool]string{true: "v"},
				nil,
			},
		*/
		{
			"2015-01-01T00:00:00Z: v\n",
			map[time.Time]string{time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC): "v"},
			nil,
		},
		{
			"123\n",
			123,
			nil,
		},
		{
			"hello: world\n",
			map[string]string{"hello": "world"},
			nil,
		},

		{
			"a: 3s\n",
			map[string]string{
				"a": "3s",
			},
			nil,
		},
		{
			"a: <foo>\n",
			map[string]string{"a": "<foo>"},
			nil,
		}, /*
			{
				"a: \"1:1\"\n",
				map[string]string{"a": "1:1"},
				nil,
			},
			{
				"a: 1.2.3.4\n",
				map[string]string{"a": "1.2.3.4"},
				nil,
			},
			{
				"a: \"b: c\"\n",
				map[string]string{"a": "b: c"},
				nil,
			},
			{
				"a: \"Hello #comment\"\n",
				map[string]string{"a": "Hello #comment"},
				nil,
			},
			{
				"a: \" b\"\n",
				map[string]string{"a": " b"},
				nil,
			},
			{
				"a: \"b \"\n",
				map[string]string{"a": "b "},
				nil,
			},
			{
				"a: \" b \"\n",
				map[string]string{"a": " b "},
				nil,
			},
			{
				"a: \"`b` c\"\n",
				map[string]string{"a": "`b` c"},
				nil,
			},
			{
				"a: 100.5\n",
				map[string]interface{}{
					"a": 100.5,
				},
				nil,
			},
			{
				"a: \"\\\\0\"\n",
				map[string]string{"a": "\\0"},
				nil,
			},
			{
				"A: 1\nB: []\n",
				struct {
					A int
					B []string
				}{
					1, ([]string)(nil),
				},
				nil,
			},
			{
				"A: 1\nB: []\n",
				struct {
					A int
					B []string
				}{
					1, []string{},
				},
				nil,
			},
			{
				"A: {}\n",
				struct {
					A map[string]interface{}
				}{
					map[string]interface{}{},
				},
				nil,
			},
			{
				"A: b\nc: d\n",
				struct {
					A string
					C string `json:"c"`
				}{
					"b", "d",
				},
				nil,
			},
			{
				"A: 1\n",
				struct {
					A int
					B int `json:"-"`
				}{
					1, 0,
				},
				nil,
			},
			{
				"A: \"\"\n",
				struct {
					A string
				}{
					"",
				},
				nil,
			},
			{
				"A: null\n",
				struct {
					A *string
				}{
					nil,
				},
				nil,
			},
			{
				"A: \"\"\n",
				struct {
					A *string
				}{
					&emptyStr,
				},
				nil,
			},
			{
				"A: null\n",
				struct {
					A *int
				}{
					nil,
				},
				nil,
			},
			{
				"A: 0\n",
				struct {
					A *int
				}{
					&zero,
				},
				nil,
			},
		*/
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			bytes, err := json.MarshalIndent(test.value, "", "  ")
			if err != nil {
				t.Fatalf("%+v", err)
			}
			bytes, err = gyaml.JsonToYaml(bytes)
			if err != nil {
				t.Error(err)
			}
			got := string(bytes)
			// b := bytes.Buffer{}
			// err := gyaml.NewEncoder(&b).Encode(test.value)
			// if err != nil {
			// 	t.Error(err)
			// }
			// got := b.String()
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeOmitGlobal(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		// OmitEmpty global option.
		{
			"a: 1\n",
			struct {
				A int
				B int `json:"b,omitempty"`
			}{1, 0},
			[]gyaml.EncodeOption{
				gyaml.OmitEmpty(),
			},
		},
		{
			"{}\n",
			struct {
				A int
				B int `json:"b,omitempty"`
			}{0, 0},
			[]gyaml.EncodeOption{
				gyaml.OmitEmpty(),
			},
		},
		{
			"a: \"\"\nb: {}\n",
			struct {
				A netip.Addr         `json:"a"`
				B struct{ X, y int } `json:"b"`
			}{},
			[]gyaml.EncodeOption{
				gyaml.OmitEmpty(),
			},
		},

		// OmitZero global option.
		{
			"a: 1\n",
			struct {
				A int
				B int
			}{1, 0},
			[]gyaml.EncodeOption{
				gyaml.OmitZero(),
			},
		},
		{
			"{}\n",
			struct {
				A int
				B int
			}{0, 0},
			[]gyaml.EncodeOption{
				gyaml.OmitZero(),
			},
		},
		{
			"{}\n",
			struct {
				A netip.Addr         `json:"a"`
				B struct{ X, y int } `json:"b"`
			}{},
			[]gyaml.EncodeOption{
				gyaml.OmitZero(),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			b := bytes.Buffer{}
			err := gyaml.NewEncoder(&b).Encode(test.value)
			if err != nil {
				t.Error(err)
			}
			got := b.String()
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeOmitEmpty(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		{
			"a: 1\n",
			struct {
				A int `json:"a,omitempty"`
				B int `json:"b,omitempty"`
			}{1, 0},
			nil,
		},

		{
			"{}\n",
			struct {
				A int `json:"a,omitempty"`
				B int `json:"b,omitempty"`
			}{0, 0},
			nil,
		},
		{
			"a: 1\n",
			struct {
				A float64 `json:"a,omitempty"`
				B float64 `json:"b,omitempty"`
			}{1, 0},
			nil,
		},
		{
			"A: 1\n",
			struct {
				A int
				B []string `json:"b,omitempty"`
			}{
				1, []string{},
			},
			nil,
		},
		//TODO: do I need this????????????? omitzero works like a charm
		// Highlighting differences of go-yaml omitempty vs std encoding/json
		// omitempty. Encoding/json will emit the following fields: https://go.dev/play/p/VvNpdM0GD4d
		{
			"{}\n",
			struct {
				// This type has a custom IsZero method.
				A netip.Addr         `json:"a,omitempty"`
				B struct{ X, y int } `json:"b,omitempty"`
			}{},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			bytes, err := json.MarshalIndent(test.value, "", "  ")
			if err != nil {
				t.Fatalf("%+v", err)
			}
			bytes, err = gyaml.JsonToYaml(bytes)
			if err != nil {
				t.Error(err)
			}
			got := string(bytes)
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeOmitZero(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		// omitzero flag.
		{
			"a: 1\n",
			struct {
				A int `json:"a,omitzero"`
				B int `json:"b,omitzero"`
			}{1, 0},
			nil,
		},

		{
			"{}\n",
			struct {
				A int `json:"a,omitzero"`
				B int `json:"b,omitzero"`
			}{0, 0},
			nil,
		},

		{
			"A: {}\n",
			struct {
				A *struct {
					X string `json:"x,omitzero"`
					Y string `json:"y,omitzero"`
				}
			}{&struct {
				X string `json:"x,omitzero"`
				Y string `json:"y,omitzero"`
			}{}},
			nil,
		},

		{
			"a: 1\n",
			struct {
				A float64 `json:"a,omitzero"`
				B float64 `json:"b,omitzero"`
			}{1, 0},
			nil,
		},
		{
			"A: 1\nb: []\n",
			struct {
				A int
				B []string `json:"b,omitzero"`
			}{
				1, []string{},
			},
			nil,
		},
		{
			"{}\n",
			struct {
				A netip.Addr         `json:"a,omitzero"`
				B struct{ X, y int } `json:"b,omitzero"`
			}{},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			bytes, err := json.MarshalIndent(test.value, "", "  ")
			if err != nil {
				t.Fatalf("%+v", err)
			}
			bytes, err = gyaml.JsonToYaml(bytes)
			if err != nil {
				t.Error(err)
			}
			got := string(bytes)
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeTime(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		{
			"v: 0001-01-01T00:00:00Z\n",
			map[string]time.Time{"v": {}},
			nil,
		},
		{
			"v: 0001-01-01T00:00:00Z\n",
			map[string]*time.Time{"v": {}},
			nil,
		},
		{
			"v: null\n",
			map[string]*time.Time{"v": nil},
			nil,
		},
		{
			"v: 30s\n",
			map[string]time.Duration{"v": 30 * time.Second},
			nil,
		},
		{
			"v: 30s\n",
			map[string]*time.Duration{"v": ptr(30 * time.Second)},
			nil,
		},
		{
			"v: null\n",
			map[string]*time.Duration{"v": nil},
			nil,
		},
		{
			"V: test\n",
			TestTextUnmarshalerContainer{V: "test"},
			nil,
		},
		{
			"V: \"1\"\n",
			TestTextUnmarshalerContainer{V: "1"},
			nil,
		},
		{
			"V: \"#\"\n",
			TestTextUnmarshalerContainer{V: "#"},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			//b := bytes.Buffer{}
			//err := gyaml.NewEncoder(&b).Encode(test.value)
			bytes, err := json.MarshalIndent(test.value, "", "  ")
			if err != nil {
				t.Error(err)
			}
			bytes, err = gyaml.JsonToYaml(bytes)
			if err != nil {
				t.Error(err)
			}
			//got := b.String()
			got := string(bytes)
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeFlow(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		// Flow flag.
		{
			"a: [1, 2]\n",
			struct {
				A []int `json:"a,flow"`
			}{[]int{1, 2}},
			nil,
		},
		{
			"a: {b: c, d: e}\n",
			&struct {
				A map[string]string `json:"a,flow"`
			}{map[string]string{"b": "c", "d": "e"}},
			nil,
		},
		{
			"a: {b: c, d: e}\n",
			struct {
				A struct {
					B, D string
				} `json:"a,flow"`
			}{struct{ B, D string }{"c", "e"}},
			nil,
		},
		// Quoting in flow mode
		{
			`a: [b, "c,d", e]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c,d", "e"}},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(false),
			},
		},
		{
			`a: [b, "c]", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c]", "d"}},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(false),
			},
		},
		{
			`a: [b, "c}", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c}", "d"}},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(false),
			},
		},
		{
			`a: [b, "c\"", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", `c"`, "d"}},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(false),
			},
		},
		{
			`a: [b, "c'", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c'", "d"}},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(false),
			},
		},
		// No quoting in non-flow mode
		{
			"a:\n- b\n- c,d\n- e\n",
			struct {
				A []string `json:"a"`
			}{[]string{"b", "c,d", "e"}},
			nil,
		},
		{
			`a: [b, "c]", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c]", "d"}},
			nil,
		},
		{
			`a: [b, "c}", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c}", "d"}},
			nil,
		},
		{
			`a: [b, "c\"", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", `c"`, "d"}},
			nil,
		},
		{
			`a: [b, "c'", d]` + "\n",
			struct {
				A []string `json:"a,flow"`
			}{[]string{"b", "c'", "d"}},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			b := bytes.Buffer{}
			err := gyaml.NewEncoder(&b).Encode(test.value)
			if err != nil {
				t.Error(err)
			}
			got := b.String()
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeQuote(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		// Quote style
		{
			`v: '''a''b'` + "\n",
			map[string]string{"v": `'a'b`},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(true),
			},
		},
		{
			`v: "'a'b"` + "\n",
			map[string]string{"v": `'a'b`},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(false),
			},
		},
		{
			`a: '\.yaml'` + "\n",
			map[string]string{"a": `\.yaml`},
			[]gyaml.EncodeOption{
				gyaml.UseSingleQuote(true),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			b := bytes.Buffer{}
			err := gyaml.NewEncoder(&b).Encode(test.value)
			if err != nil {
				t.Error(err)
			}
			got := b.String()
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}

func TestEncodeStructIncludeMap(t *testing.T) {
	type U struct {
		M map[string]string
	}
	type T struct {
		A U
	}
	bytes, err := json.MarshalIndent(T{
		A: U{
			M: map[string]string{"x": "y"},
		},
	}, "", "  ")
	if err != nil {
		t.Fatalf("%+v", err)
	}
	bytes, err = gyaml.JsonToYaml(bytes)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	expect := "A:\n  M:\n    x: \"y\"\n"
	actual := string(bytes)
	if actual != expect {
		t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
	}
}

func TestEncodeDefinedTypeKeyMap(t *testing.T) {
	type K string
	type U struct {
		M map[K]string
	}
	bytes, err := json.MarshalIndent(U{
		M: map[K]string{K("x"): "y"},
	}, "", "  ")
	if err != nil {
		t.Fatalf("%+v", err)
	}
	bytes, err = gyaml.JsonToYaml(bytes)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	expect := "M:\n  x: \"y\"\n"
	actual := string(bytes)
	if actual != expect {
		t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
	}
}

func TestEncodeDefinedTypeKeyMapIndent(t *testing.T) {
	type K string
	type U struct {
		M map[K]string
	}
	bytes, err := json.MarshalIndent(U{
		M: map[K]string{K("x"): "y"},
	},
		"",
		"  ")
	if err != nil {
		t.Fatalf("%+v", err)
	}
	bytes, err = gyaml.JsonToYaml(bytes)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	expect := "M:\n  x: \"y\"\n"
	actual := string(bytes)
	if actual != expect {
		t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
	}
}

func TestEncodeAdvanced(t *testing.T) {
	tests := []struct {
		want    string
		value   any
		options []gyaml.EncodeOption
	}{
		{
			"v:\n- A\n- B\n",
			map[string][]string{"v": {"A", "B"}},
			nil,
		},
		{
			"v:\n  - A\n  - B\n",
			map[string][]string{"v": {"A", "B"}},
			[]gyaml.EncodeOption{
				gyaml.IndentSequence(true),
			},
		},
		{
			"v:\n- A\n- B\n",
			map[string][2]string{"v": {"A", "B"}},
			nil,
		},
		{
			"v:\n  - A\n  - B\n",
			map[string][2]string{"v": {"A", "B"}},
			[]gyaml.EncodeOption{
				gyaml.IndentSequence(true),
			},
		},
		{
			"hello: |\n  hello\n  world\n",
			map[string]string{"hello": "hello\nworld\n"},
			nil,
		},
		{
			"hello: |-\n  hello\n  world\n",
			map[string]string{"hello": "hello\nworld"},
			nil,
		},
		{
			"hello: |+\n  hello\n  world\n\n",
			map[string]string{"hello": "hello\nworld\n\n"},
			nil,
		},
		{
			"hello:\n  hello: |\n    hello\n    world\n",
			map[string]map[string]string{"hello": {"hello": "hello\nworld\n"}},
			nil,
		},
		{
			"hello: |\r  hello\r  world\n",
			map[string]string{"hello": "hello\rworld\r"},
			nil,
		},
		{
			"hello: |\r\n  hello\r\n  world\n",
			map[string]string{"hello": "hello\r\nworld\r\n"},
			nil,
		},
		{
			"v: |-\n  username: hello\n  password: hello123\n",
			map[string]interface{}{"v": "username: hello\npassword: hello123"},
			[]gyaml.EncodeOption{
				gyaml.UseLiteralStyleIfMultiline(true),
			},
		},
		{
			"v: |-\n  # comment\n  username: hello\n  password: hello123\n",
			map[string]interface{}{"v": "# comment\nusername: hello\npassword: hello123"},
			[]gyaml.EncodeOption{
				gyaml.UseLiteralStyleIfMultiline(true),
			},
		},
		{
			"v: \"# comment\\nusername: hello\\npassword: hello123\"\n",
			map[string]interface{}{"v": "# comment\nusername: hello\npassword: hello123"},
			[]gyaml.EncodeOption{
				gyaml.UseLiteralStyleIfMultiline(false),
			},
		},
		{
			"v:\n- A\n- 1\n- B:\n  - 2\n  - 3\n",
			map[string]interface{}{
				"v": []interface{}{
					"A",
					1,
					map[string][]int{
						"B": {2, 3},
					},
				},
			},
			nil,
		},
		{
			"v:\n  - A\n  - 1\n  - B:\n      - 2\n      - 3\n  - 2\n",
			map[string]interface{}{
				"v": []interface{}{
					"A",
					1,
					map[string][]int{
						"B": {2, 3},
					},
					2,
				},
			},
			[]gyaml.EncodeOption{
				gyaml.IndentSequence(true),
			},
		},
		{
			"a:\n  b: c\n",
			map[string]interface{}{
				"a": map[string]string{
					"b": "c",
				},
			},
			nil,
		},
		{
			"t2: \"2018-01-09T10:40:47Z\"\nt4: \"2098-01-09T10:40:47Z\"\n",
			map[string]string{
				"t2": "2018-01-09T10:40:47Z",
				"t4": "2098-01-09T10:40:47Z",
			},
			nil,
		},
		{
			"a:\n  b: c\n  d: e\n",
			map[string]interface{}{
				"a": map[string]string{
					"b": "c",
					"d": "e",
				},
			},
			nil,
		},
		{
			"a: 1\nb: 2\nc: 3\nd: 4\nsub:\n  e: 5\n",
			map[string]interface{}{
				"a": 1,
				"b": 2,
				"c": 3,
				"d": 4,
				"sub": map[string]int{
					"e": 5,
				},
			},
			nil,
		},
		{
			"A:\n  \"y\": \"\"\n",
			struct {
				A *struct {
					X string `json:"x,omitempty"`
					Y string
				}
			}{&struct {
				X string `json:"x,omitempty"`
				Y string
			}{}},
			nil,
		},
		{
			"a: {}\n",
			struct {
				A *struct {
					X string `json:"x,omitempty"`
					Y string `json:"y,omitempty"`
				}
			}{&struct {
				X string `json:"x,omitempty"`
				Y string `json:"y,omitempty"`
			}{}},
			nil,
		},
		{
			"a: {x: 1}\n",
			struct {
				A *struct{ X, y int } `json:"a,omitempty,flow"`
			}{&struct{ X, y int }{1, 2}},
			nil,
		},
		{
			"{}\n",
			struct {
				A *struct{ X, y int } `json:"a,omitempty,flow"`
			}{nil},
			nil,
		},
		{
			"a: {x: 0}\n",
			struct {
				A *struct{ X, y int } `json:"a,omitempty,flow"`
			}{&struct{ X, y int }{}},
			nil,
		},
		{
			"a: {x: 1}\n",
			struct {
				A struct{ X, y int } `json:"a,omitempty,flow"`
			}{struct{ X, y int }{1, 2}},
			nil,
		},
		{
			"{}\n",
			struct {
				A struct{ X, y int } `json:"a,omitempty,flow"`
			}{struct{ X, y int }{0, 1}},
			nil,
		},
		{
			"a:\n  \"y\": \"\"\n",
			struct {
				A *struct {
					X string `json:"x,omitzero"`
					Y string
				}
			}{&struct {
				X string `json:"x,omitzero"`
				Y string
			}{}},
			nil,
		},
		{
			"a: {x: 1}\n",
			struct {
				A *struct{ X, y int } `json:"a,omitzero,flow"`
			}{&struct{ X, y int }{1, 2}},
			nil,
		},
		{
			"{}\n",
			struct {
				A *struct{ X, y int } `json:"a,omitzero,flow"`
			}{nil},
			nil,
		},
		{
			"a: {x: 0}\n",
			struct {
				A *struct{ X, y int } `json:"a,omitzero,flow"`
			}{&struct{ X, y int }{}},
			nil,
		},
		{
			"a: {x: 1}\n",
			struct {
				A struct{ X, y int } `json:"a,omitzero,flow"`
			}{struct{ X, y int }{1, 2}},
			nil,
		},
		{
			"{}\n",
			struct {
				A struct{ X, y int } `json:"a,omitzero,flow"`
			}{struct{ X, y int }{0, 1}},
			nil,
		},
		// Multi bytes
		{
			"v: あいうえお\nv2: かきくけこ\n",
			map[string]string{"v": "あいうえお", "v2": "かきくけこ"},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			b := bytes.Buffer{}
			err := gyaml.NewEncoder(&b).Encode(test.value)
			if err != nil {
				t.Error(err)
			}
			got := b.String()
			if got != test.want {
				t.Errorf("Want: [%s], got: [%s]\n", test.want, got)
			}
		})
	}
}
