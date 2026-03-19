// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gyaml

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/google/go-cmp/cmp"
)

var zero = 0
var emptyStr = ""

func ptr[T any](v T) *T {
	return &v
}

type TestTextMarshaler string

func (t TestTextMarshaler) MarshalText() ([]byte, error) {
	return []byte(t), nil
}

type TestTextUnmarshalerContainer struct {
	V TestTextMarshaler
}

// test for debugging
func TestEncode(t *testing.T) {
	t.Run("anchors with flow", func(t *testing.T) {
		type Person struct {
			*Person `yaml:",omitempty"`
			Name    string `yaml:",omitempty"`
			Age     int    `yaml:",omitempty"`
		}
		defaultPerson := &Person{Name: "John Smith", Age: 20}
		people := []*Person{
			{
				Person: defaultPerson,
				Name:   "Ken",
				Age:    10,
			},
			defaultPerson,
		}
		var doc struct {
			Default *Person   `yaml:"default,anchor,flow"`
			People  []*Person `yaml:"people,flow"`
		}
		doc.Default = defaultPerson
		doc.People = people
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `default: &default {name: John Smith, age: 20}
people: [{name: Ken, age: 10}, *default]
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})
}

func TestEncodeTable(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"null\n",
			(*struct{})(nil),
			nil,
		},
		{
			"v: null\n",
			map[string]map[string]string{"v": nil},
			nil,
		},
		{
			"v: hi\n",
			map[string]string{"v": "hi"},
			nil,
		},
		{
			"v: true\n",
			map[string]interface{}{"v": true},
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
			"v: 4444444444\n",
			map[string]uint64{"v": uint64(4444444444)},
			nil,
		},
		{
			"3333\n",
			uint64(3333),
			nil,
		},
		{
			"v: 0.1\n",
			map[string]interface{}{"v": 0.1},
			nil,
		},
		{
			"v: 0.99\n",
			map[string]float32{"v": 0.99},
			nil,
		},
		{
			"v: 1e-06\n",
			map[string]float32{"v": 1e-06},
			nil,
		},
		{
			"v: 1e-06\n",
			map[string]float64{"v": 0.000001},
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
			"v: 1.0\n",
			map[string]float64{"v": 1.0},
			nil,
		},
		{
			"v: 1e+06\n",
			map[string]float64{"v": 1000000},
			nil,
		},
		{
			"v: 1e-06\n",
			map[string]float64{"v": 0.000001},
			nil,
		},
		{
			"v: 1e-06\n",
			map[string]float64{"v": 1e-06},
			nil,
		},
		{
			"v: .inf\n",
			map[string]interface{}{"v": math.Inf(0)},
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
		{
			"v: null\n",
			map[string]interface{}{"v": nil},
			nil,
		},
		{
			"v: []\n", //is this ok to encode nil slice as zero-length?
			map[string][]string{"v": nil},
			nil,
		},
		{
			"v:\n- A\n- B\n",
			map[string][]string{"v": {"A", "B"}},
			nil,
		},
		{
			"v:\n- A\n- B\n",
			struct{ V []string }{V: []string{"A", "B"}},
			nil,
		},
		{
			"v:\n  - A\n  - B\n",
			map[string][]string{"v": {"A", "B"}},
			func(e *Encoder) *Encoder { return e.WithIndentSequence(true) },
		},
		{
			"v:\n  - A\n  - B\n",
			struct{ V []string }{V: []string{"A", "B"}},
			func(e *Encoder) *Encoder { return e.WithIndentSequence(true) },
		},
		{
			"v:\n- A\n- B\n",
			map[string][2]string{"v": {"A", "B"}},
			nil,
		},
		{
			"v:\n  - A\n  - B\n",
			map[string][2]string{"v": {"A", "B"}},
			func(e *Encoder) *Encoder { return e.WithIndentSequence(true) },
		},
		{
			"1: v\n",
			map[int]string{1: "v"},
			nil,
		},
		{
			"1: v\n",
			map[uint]string{1: "v"},
			nil,
		},
		{
			"1.1: v\n",
			map[float32]string{1.1: "v"},
			nil,
		},
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
			"v: \"# comment\\nusername: hello\\npassword: hello123\"\n",
			map[string]interface{}{"v": "# comment\nusername: hello\npassword: hello123"},
			func(e *Encoder) *Encoder { return e.WithLiteralMultilineStyle(false) },
		},
		{
			"v:\n- A\n- 1\n- B:\n  - 2\n  - 3\n",
			map[string]any{
				"v": []any{
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
			func(e *Encoder) *Encoder { return e.WithIndentSequence(true) },
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
		},
		{
			"a: 1.2.3.4\n",
			map[string]string{"a": "1.2.3.4"},
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
			"a: 1\nb: []\n",
			struct {
				A int
				B []string
			}{
				1, ([]string)(nil),
			},
			nil,
		},
		{
			"a: 1\nb: []\n",
			struct {
				A int
				B []string
			}{
				1, []string{},
			},
			nil,
		},
		{
			"a: {}\n",
			struct {
				A map[string]interface{}
			}{
				map[string]interface{}{},
			},
			nil,
		},
		{
			"a: b\nc: d\n",
			struct {
				A string
				C string `yaml:"c"`
			}{
				"b", "d",
			},
			nil,
		},
		{
			"a: 1\n",
			struct {
				A int
				B int `yaml:"-"`
			}{
				1, 0,
			},
			nil,
		},
		{
			"a: null\n",
			struct {
				A *string
			}{
				nil,
			},
			nil,
		},
		{
			"a: null\n",
			struct {
				A *int
			}{
				nil,
			},
			nil,
		},
		{
			"a: 0\n",
			struct {
				A *int
			}{
				&zero,
			},
			nil,
		},

		// No quoting in non-flow mode
		{
			"a:\n- b\n- c,d\n- e\n",
			struct {
				A []string `yaml:"a"`
			}{[]string{"b", "c,d", "e"}},
			nil,
		},
		// Multi bytes
		{
			"v: あいうえお\nv2: かきくけこ\n",
			map[string]string{"v": "あいうえお", "v2": "かきくけこ"},
			nil,
		},
		{
			"v: test\n",
			TestTextUnmarshalerContainer{V: "test"},
			nil,
		},
		{
			"v: \"1\"\n",
			TestTextUnmarshalerContainer{V: "1"},
			nil,
		},
		{
			"v: \"#\"\n",
			TestTextUnmarshalerContainer{V: "#"},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

func TestEncodeTime(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"2015-01-01T00:00:00Z: v\n",
			map[time.Time]string{time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC): "v"},
			nil,
		},
		{
			"v: \"0001-01-01T00:00:00Z\"\n",
			map[string]time.Time{"v": {}},
			nil,
		},
		{
			"v: \"2015-01-01T00:00:00Z\"\n",
			struct{ V time.Time }{V: time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
			nil,
		},
		{
			"v: \"0001-01-01T00:00:00Z\"\n",
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
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

func TestEncodeOmitEmpty(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"a: 1\n",
			struct {
				A int `yaml:"a,omitempty"`
				B int `yaml:"b,omitempty"`
			}{1, 0},
			nil,
		},
		{
			"{}\n",
			struct {
				A int `yaml:"a,omitempty"`
				B int `yaml:"b,omitempty"`
			}{0, 0},
			nil,
		},
		{
			"a: {}\n",
			struct {
				A *struct {
					X string `yaml:"x,omitempty"`
					Y string `yaml:"y,omitempty"`
				}
			}{&struct {
				X string `yaml:"x,omitempty"`
				Y string `yaml:"y,omitempty"`
			}{}},
			nil,
		},
		{
			"a: 1.0\n",
			struct {
				A float64 `yaml:"a,omitempty"`
				B float64 `yaml:"b,omitempty"`
			}{1, 0},
			nil,
		},
		{
			"a: 1\n",
			struct {
				A int
				B []string `yaml:"b,omitempty"`
			}{
				1, []string{},
			},
			nil,
		},
		//same behavior as in encoding/json, omitempty does not care for structs
		//use omitzero for that
		{
			"a: \"\"\nb:\n  x: 0\n",
			struct {
				// This type has a custom IsZero method.
				A netip.Addr         `yaml:"a,omitempty"`
				B struct{ X, y int } `yaml:"b,omitempty"`
			}{},
			nil,
		},
		{
			"a: 1\n",
			struct {
				A int
				B int `yaml:"b,omitempty"`
			}{1, 0},
			func(e *Encoder) *Encoder { return e.WithOmitEmpty(true) },
		},
		{
			"{}\n",
			struct {
				A int
				B int `yaml:"b,omitempty"`
			}{0, 0},
			func(e *Encoder) *Encoder { return e.WithOmitEmpty(true) },
		},
		{
			"a: \"\"\nb: {}\n",
			struct {
				A netip.Addr         `yaml:"a"`
				B struct{ X, y int } `yaml:"b"`
			}{},
			func(e *Encoder) *Encoder { return e.WithOmitEmpty(true) },
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

type zeroInt int

func (z zeroInt) IsZero() bool {
	return z < 2
}

type ZeroStruct struct {
	X, y int
}

func (a ZeroStruct) IsZero() bool {
	return a.X == 0
}

func TestEncodeOmitZero(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"a: 1\n",
			struct {
				A int `yaml:"a,omitzero"`
				B int `yaml:"b,omitzero"`
			}{1, 0},
			nil,
		},
		{
			"{}\n",
			struct {
				A int `yaml:"a,omitzero"`
				B int `yaml:"b,omitzero"`
			}{0, 0},
			nil,
		},
		{
			"a: {}\n",
			struct {
				A *struct {
					X string `yaml:"x,omitzero"`
					Y string `yaml:"y,omitzero"`
				}
			}{&struct {
				X string `yaml:"x,omitzero"`
				Y string `yaml:"y,omitzero"`
			}{}},
			nil,
		},
		{
			"a: 1.0\n",
			struct {
				A float64 `yaml:"a,omitzero"`
				B float64 `yaml:"b,omitzero"`
			}{1, 0},
			nil,
		},
		{
			"a: 1\nb: []\n",
			struct {
				A int
				B []string `yaml:"b,omitzero"`
			}{
				1, []string{},
			},
			nil,
		},
		{
			"{}\n",
			struct {
				A netip.Addr         `yaml:"a,omitzero"`
				B struct{ X, y int } `yaml:"b,omitzero"`
			}{},
			nil,
		},
		{
			"a: 1\n",
			struct {
				A int
				B int
			}{1, 0},
			func(e *Encoder) *Encoder { return e.WithOmitZero(true) },
		},
		{
			"{}\n",
			struct {
				A int
				B int
			}{0, 0},
			func(e *Encoder) *Encoder { return e.WithOmitZero(true) },
		},
		{
			"{}\n",
			struct {
				A netip.Addr         `yaml:"a"`
				B struct{ X, y int } `yaml:"b"`
			}{},
			func(e *Encoder) *Encoder { return e.WithOmitZero(true) },
		},
		{
			"a: 1\n",
			struct {
				A int     `yaml:"a,omitzero"`
				B zeroInt `yaml:"b,omitzero"`
			}{1, 1},
			nil,
		},
		//works like encoding/json omitzero
		{
			"a:\n  x: 0\n",
			struct {
				A struct{ X, y int } `yaml:"a,omitzero"`
			}{struct{ X, y int }{0, 1}},
			nil,
		},
		{
			"{}\n",
			struct {
				A ZeroStruct `yaml:"a,omitzero"`
			}{ZeroStruct{X: 0, y: 1}},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

func TestEncodeMultilineString(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
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
			map[string]any{"v": "username: hello\npassword: hello123"},
			func(e *Encoder) *Encoder { return e.WithLiteralMultilineStyle(true) },
		},
		{
			"v: |-\n  # comment\n  username: hello\n  password: hello123\n",
			map[string]any{"v": "# comment\nusername: hello\npassword: hello123"},
			func(e *Encoder) *Encoder { return e.WithLiteralMultilineStyle(true) },
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

func TestEncodeFlow(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"a: {x: 1}\n",
			struct {
				A *struct{ X, y int } `yaml:"a,omitempty,flow"`
			}{&struct{ X, y int }{1, 2}},
			nil,
		},
		{
			"{}\n",
			struct {
				A *struct{ X, y int } `yaml:"a,omitempty,flow"`
			}{nil},
			nil,
		},
		{
			"a: {x: 0}\n",
			struct {
				A *struct{ X, y int } `yaml:"a,omitempty,flow"`
			}{&struct{ X, y int }{}},
			nil,
		},
		{
			"a: {x: 1}\n",
			struct {
				A struct{ X, y int } `yaml:"a,omitempty,flow"`
			}{struct{ X, y int }{1, 2}},
			nil,
		},
		// {
		// 	"{}\n",
		// 	struct {
		// 		A struct{ X, y int } `yaml:"a,omitempty,flow"`
		// 	}{struct{ X, y int }{0, 1}},
		// 	nil,
		// },
		{
			"a: {x: 1}\n",
			struct {
				A *struct{ X, y int } `yaml:"a,omitzero,flow"`
			}{&struct{ X, y int }{1, 2}},
			nil,
		},
		{
			"{}\n",
			struct {
				A *struct{ X, y int } `yaml:"a,omitzero,flow"`
			}{nil},
			nil,
		},
		{
			"a: {x: 0}\n",
			struct {
				A *struct{ X, y int } `yaml:"a,omitzero,flow"`
			}{&struct{ X, y int }{}},
			nil,
		},
		{
			"a: {x: 1}\n",
			struct {
				A struct{ X, y int } `yaml:"a,omitzero,flow"`
			}{struct{ X, y int }{1, 2}},
			nil,
		},
		//works like encoding/json omitzero
		{
			"a: {x: 0}\n",
			struct {
				A struct{ X, y int } `yaml:"a,omitzero,flow"`
			}{struct{ X, y int }{0, 1}},
			nil,
		},
		{
			"{}\n",
			struct {
				A ZeroStruct `yaml:"a,omitzero,flow"`
			}{ZeroStruct{X: 0, y: 1}},
			nil,
		},
		{
			"a: [1, 2]\n",
			struct {
				A []int `yaml:"a,flow"`
			}{[]int{1, 2}},
			nil,
		},
		{
			"a: {b: c, d: e}\n",
			&struct {
				A map[string]string `yaml:"a,flow"`
			}{map[string]string{"b": "c", "d": "e"}},
			nil,
		},
		{
			"a: {b: c, d: e}\n",
			struct {
				A struct {
					B, D string
				} `yaml:"a,flow"`
			}{struct{ B, D string }{"c", "e"}},
			nil,
		},
		// Quoting in flow mode
		{
			`a: [b, "c,d", e]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c,d", "e"}},
			func(e *Encoder) *Encoder { return e.WithSingleQuote(false) },
		},
		{
			`a: [b, "c]", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c]", "d"}},
			func(e *Encoder) *Encoder { return e.WithSingleQuote(false) },
		},
		{
			`a: [b, "c}", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c}", "d"}},
			func(e *Encoder) *Encoder { return e.WithSingleQuote(false) },
		},
		{
			`a: [b, "c\"", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", `c"`, "d"}},
			func(e *Encoder) *Encoder { return e.WithSingleQuote(false) },
		},
		{
			`a: [b, "c'", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c'", "d"}},
			func(e *Encoder) *Encoder { return e.WithSingleQuote(false) },
		},
		{
			`a: [b, "c]", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c]", "d"}},
			nil,
		},
		{
			`a: [b, "c}", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c}", "d"}},
			nil,
		},
		{
			`a: [b, "c\"", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", `c"`, "d"}},
			nil,
		},
		{
			`a: [b, "c'", d]` + "\n",
			struct {
				A []string `yaml:"a,flow"`
			}{[]string{"b", "c'", "d"}},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
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
	bytes, err := Marshal(T{
		A: U{
			M: map[string]string{"x": "z"},
		},
	})
	if err != nil {
		t.Fatalf("%+v", err)
	}
	expect := "a:\n  m:\n    x: z"
	actual := string(bytes)
	if actual != expect {
		t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
	}
}

func TestEncodeSliceOfStructs(t *testing.T) {
	type B struct {
		X, Y string
	}
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"a:\n- x: xx\n  y: yy\n- x: xxx\n  y: yyy\n",
			struct {
				A []B `yaml:"a"`
			}{[]B{
				{"xx", "yy"},
				{"xxx", "yyy"},
			}},
			nil,
		},
		{
			"a: [{x: xx, y: yy}, {x: xxx, y: yyy}]\n",
			struct {
				A []B `yaml:"a,flow"`
			}{[]B{
				{"xx", "yy"},
				{"xxx", "yyy"},
			}},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

func TestEncodeSliceOfMaps(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
		{
			"a:\n- x: xx\n  y: yy\n- x: xxx\n  y: yyy\n",
			struct {
				A []map[string]string `yaml:"a"`
			}{[]map[string]string{
				{"x": "xx", "y": "yy"},
				{"x": "xxx", "y": "yyy"},
			}},
			nil,
		},
		{
			"a: [{x: xx, y: yy}, {x: xxx, y: yyy}]\n",
			struct {
				A []map[string]string `yaml:"a,flow"`
			}{[]map[string]string{
				{"x": "xx", "y": "yy"},
				{"x": "xxx", "y": "yyy"},
			}},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

func TestEncodeDefinedTypeKeyMap(t *testing.T) {
	type K string
	type U struct {
		M map[K]string
	}
	bytes, err := Marshal(U{
		M: map[K]string{K("x"): "z"},
	})
	if err != nil {
		t.Fatalf("%+v", err)
	}
	expect := "m:\n  x: z"
	actual := string(bytes)
	if actual != expect {
		t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
	}
}

func TestEncoder_Flow(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf).WithFlowStyle(true)
	var v struct {
		A int
		B string
		C struct {
			D int
			E string
		}
		F []int `yaml:"F"`
	}
	v.A = 1
	v.B = "hello"
	v.C.D = 3
	v.C.E = "world"
	v.F = []int{1, 2}
	if err := enc.Encode(v); err != nil {
		t.Fatalf("%+v", err)
	}
	expect := `
{a: 1, b: hello, c: {d: 3, e: world}, F: [1, 2]}
`
	actual := "\n" + buf.String()
	if expect != actual {
		t.Fatalf("flow style marshal error: expect=[%s] actual=[%s]", expect, actual)
	}
}

func TestEncoder_FlowRecursive(t *testing.T) {
	var v struct {
		M map[string][]int `yaml:",flow"`
	}
	v.M = map[string][]int{
		"test": {1, 2, 3},
	}
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("%+v", err)
	}
	expect := `m: {test: [1, 2, 3]}
`
	actual := buf.String()
	if expect != actual {
		t.Fatalf("flow style marshal error: expect=[%s] actual=[%s]", expect, actual)
	}
}

func TestEncoder_JSON(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf).WithJSONStyle(true).WithFlowStyle(true)
	type st struct {
		I int8
		S string
		F float32
	}
	if err := enc.Encode(struct {
		I        int
		U        uint
		S        string
		F        float64
		Struct   *st
		Slice    []int
		Map      map[string]interface{}
		Time     time.Time
		Duration time.Duration
	}{
		I: -10,
		U: 10,
		S: "hello",
		F: 3.14,
		Struct: &st{
			I: 2,
			S: "world",
			F: 1.23,
		},
		Slice: []int{1, 2, 3, 4, 5},
		Map: map[string]interface{}{
			"a": 1,
			"b": 1.23,
			"c": "json",
		},
		Time:     time.Time{},
		Duration: 5 * time.Minute,
	}); err != nil {
		t.Fatalf("%+v", err)
	}
	expect := `
{"i": -10, "u": 10, "s": "hello", "f": 3.14, "struct": {"i": 2, "s": "world", "f": 1.23}, "slice": [1, 2, 3, 4, 5], "map": {"a": 1, "b": 1.23, "c": "json"}, "time": "0001-01-01T00:00:00Z", "duration": "5m0s"}
`
	actual := "\n" + buf.String()
	if expect != actual {
		t.Fatalf("JSON style marshal error: expect=[%s] actual=[%s]", expect, actual)
	}
}

func TestEncoder_AutoInt(t *testing.T) {
	for _, test := range []struct {
		desc     string
		input    any
		expected string
	}{
		{
			desc: "int-convertible float64",
			input: map[string]float64{
				"key": 1.0,
			},
			expected: "key: 1\n",
		},
		{
			desc: "non int-convertible float64",
			input: map[string]float64{
				"key": 1.1,
			},
			expected: "key: 1.1\n",
		},
		{
			desc: "int-convertible float32",
			input: map[string]float32{
				"key": 1.0,
			},
			expected: "key: 1\n",
		},
		{
			desc: "non int-convertible float32",
			input: map[string]float32{
				"key": 1.1,
			},
			expected: "key: 1.1\n",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf).WithAutoInt(true)
			if err := enc.Encode(test.input); err != nil {
				t.Fatalf("failed to encode: %s", err)
			}
			if actual := buf.String(); actual != test.expected {
				t.Errorf("expect:\n%s\nactual\n%s\n", test.expected, actual)
			}
		})
	}
}

func TestEncoder_Inline(t *testing.T) {
	type base struct {
		A int
		B string
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(struct {
		*base `yaml:",inline"`
		C     bool
	}{
		base: &base{
			A: 1,
			B: "hello",
		},
		C: true,
	}); err != nil {
		t.Fatalf("%+v", err)
	}
	expect := `
a: 1
b: hello
c: true
`
	actual := "\n" + buf.String()
	if expect != actual {
		t.Fatalf("inline marshal error: expect=[%s] actual=[%s]", expect, actual)
	}
}

func TestEncoder_InlineAndConflictKey(t *testing.T) {
	type base struct {
		A int
		B string
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(struct {
		*base `yaml:",inline"`
		A     int // conflict
		C     bool
	}{
		base: &base{
			A: 1,
			B: "hello",
		},
		A: 0, // default value
		C: true,
	}); err != nil {
		t.Fatalf("%+v", err)
	}
	expect := `
b: hello
a: 0
c: true
`
	actual := "\n" + buf.String()
	if expect != actual {
		t.Fatalf("inline marshal error: expect=[%s] actual=[%s]", expect, actual)
	}
}

func TestEncoder_InlineNil(t *testing.T) {
	type base struct {
		A int
		B string
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(struct {
		*base `yaml:",inline"`
		C     bool
	}{
		C: true,
	}); err != nil {
		t.Fatalf("%+v", err)
	}
	expect := `
c: true
`
	actual := "\n" + buf.String()
	if expect != actual {
		t.Fatalf("inline marshal error: expect=[%s] actual=[%s]", expect, actual)
	}
}

func TestEncoder_MultipleDocuments(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(1); err != nil {
		t.Fatalf("failed to encode: %s", err)
	}
	if err := enc.Encode(2); err != nil {
		t.Fatalf("failed to encode: %s", err)
	}
	if actual, expect := buf.String(), "1\n---\n2\n"; actual != expect {
		t.Errorf("expect:\n[%s]\nactual\n[%s]\n", expect, actual)
	}
}

func TestEncoder_UnmarshallableTypes(t *testing.T) {
	for _, test := range []struct {
		desc        string
		input       any
		expectedErr string
	}{
		{
			desc:        "channel",
			input:       make(chan int),
			expectedErr: "unknown value type chan int",
		},
		{
			desc:        "function",
			input:       func() {},
			expectedErr: "unknown value type func()",
		},
		{
			desc:        "complex number",
			input:       complex(10, 11),
			expectedErr: "unknown value type complex128",
		},
		{
			desc:        "unsafe pointer",
			input:       unsafe.Pointer(&struct{}{}),
			expectedErr: "unknown value type unsafe.Pointer",
		},
		{
			desc:        "uintptr",
			input:       uintptr(0x1234),
			expectedErr: "unknown value type uintptr",
		},
		{
			desc:        "map with channel",
			input:       map[string]any{"key": make(chan string)},
			expectedErr: "unknown value type chan string",
		},
		{
			desc: "nested map with func",
			input: map[string]any{
				"a": map[string]any{
					"b": func(_ string) {},
				},
			},
			expectedErr: "unknown value type func(string)",
		},
		{
			desc:        "slice with channel",
			input:       []any{make(chan bool)},
			expectedErr: "unknown value type chan bool",
		},
		{
			desc:        "nested slice with complex number",
			input:       []any{[]any{complex(10, 11)}},
			expectedErr: "unknown value type complex128",
		},
		{
			desc: "struct with unsafe pointer",
			input: struct {
				Field unsafe.Pointer `yaml:"field"`
			}{},
			expectedErr: "unknown value type unsafe.Pointer",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			var buf bytes.Buffer
			err := NewEncoder(&buf).Encode(test.input)
			if err == nil {
				t.Errorf("expect error:\n%s\nbut got none\n", test.expectedErr)
			} else if err.Error() != test.expectedErr {
				t.Errorf("expect error:\n%s\nactual\n%s\n", test.expectedErr, err)
			}
		})
	}
}

type tMarshal []string

func (t *tMarshal) MarshalYAML() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("tags:")
	for i, v := range *t {
		if i == 0 {
			fmt.Fprintf(&buf, "\n- %s", v)
		} else {
			fmt.Fprintf(&buf, "\n  %s", v)
		}
	}
	return buf.Bytes(), nil
}

type flowMarshal []string

func (f flowMarshal) MarshalYAML() ([]byte, error) {
	return MarshalWithOptions([]string(f), EncoderOptions{IndentSize: 2, FlowStyle: true})
}

func Test_Marshaler(t *testing.T) {
	t.Run("custom marshaler", func(t *testing.T) {
		const expected = "tags:\n- hello-world"

		buf, err := Marshal(&tMarshal{"hello-world"})
		if err != nil {
			t.Fatalf("failed to marshal: %s", err)
		}

		if string(buf) != expected {
			t.Fatalf("expected [%s], got [%s]", expected, buf)
		}
	})

	t.Run("custom nil marshaler", func(t *testing.T) {
		const expected = "null"

		buf, err := Marshal((*tMarshal)(nil))
		if err != nil {
			t.Fatalf("failed to marshal: %s", err)
		}

		if string(buf) != expected {
			t.Fatalf("expected [%s], got [%s]", expected, buf)
		}
	})

	t.Run("custom flow marshaler", func(t *testing.T) {
		const expected = "[hello, world]"

		buf, err := Marshal(flowMarshal{"hello", "world"})
		if err != nil {
			t.Fatalf("failed to marshal: %s", err)
		}

		if string(buf) != expected {
			t.Fatalf("expected [%s], got [%s]", expected, buf)
		}
	})

	t.Run("custom flow marshaler with encode", func(t *testing.T) {
		const expected = "[hello, world]\n"

		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		err := enc.Encode(flowMarshal{"hello", "world"})
		if err != nil {
			t.Fatalf("failed to marshal: %s", err)
		}

		got := buf.String()
		if expected != got {
			t.Fatalf("expected [%s], got [%s]", expected, got)
		}
	})
}

func TestMarshalIndentWithMultipleText(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]any
		indent int
		want   string
	}{
		{
			name: "depth1",
			input: map[string]any{
				"key": []string{`line1
line2
line3`},
			},
			indent: 2,
			want: `key:
- |-
  line1
  line2
  line3
`,
		},
		{
			name: "depth2",
			input: map[string]interface{}{
				"key": map[string]interface{}{
					"key2": []string{`line1
line2
line3`},
				},
			},
			indent: 2,
			want: `key:
  key2:
  - |-
    line1
    line2
    line3
`,
		},
		{
			name: "raw string new lines",
			input: map[string]any{
				"key": "line1\nline2\nline3",
			},
			indent: 4,
			want: `key: |-
    line1
    line2
    line3
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			enc := NewEncoder(&buf).WithIndent(tt.indent)
			err := enc.Encode(tt.input)
			if err != nil {
				t.Fatalf("failed to marshal yaml: %v", err)
			}
			got := buf.String()
			if tt.want != got {
				t.Fatalf("failed to encode.\nexpected:\n[%s]\nbut got:\n[%s]\n", tt.want, got)
			}
		})
	}
}

type bytesMarshaler struct{}

func (b *bytesMarshaler) MarshalYAML() ([]byte, error) {
	return []byte("foo"), nil
}

func TestBytesMarshaler(t *testing.T) {
	b, err := Marshal(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": &bytesMarshaler{},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := `
a:
  b:
    c: foo`
	got := "\n" + string(b)
	if expected != got {
		t.Fatalf("expected [%s], got [%s]", expected, got)
	}
}

type badMarshaler struct{}

func (b *badMarshaler) MarshalYAML() ([]byte, error) {
	return nil, errors.New("bad marshaler error")
}

func TestBadMarshaler(t *testing.T) {
	_, err := Marshal(map[string]any{
		"v": &badMarshaler{},
	})
	if err == nil {
		t.Fatal("expected non nil error")
	}
}

type valueMarshaler struct{}

func (v valueMarshaler) MarshalYAML() ([]byte, error) {
	return []byte("value"), nil
}

func TestValueMarshaler(t *testing.T) {
	t.Run("custom pointer marshal", func(t *testing.T) {
		b, err := Marshal(map[string]any{
			"v": &valueMarshaler{},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := string(b)
		expected := "v: value"
		if got != expected {
			t.Fatalf("expected [%s] got [%s]", expected, got)
		}
	})
	t.Run("custom value marshal", func(t *testing.T) {
		b, err := Marshal(map[string]any{
			"v": valueMarshaler{},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := string(b)
		expected := "v: value"
		if got != expected {
			t.Fatalf("expected [%s] got [%s]", expected, got)
		}
	})
	t.Run("custom value marshal in struct", func(t *testing.T) {
		type T struct {
			V  valueMarshaler
			VP *valueMarshaler
		}
		b, err := Marshal(T{
			V:  valueMarshaler{},
			VP: &valueMarshaler{},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := string(b)
		expected := "v: value\nvp: value"
		if got != expected {
			t.Fatalf("expected [%s] got [%s]", expected, got)
		}
	})
}

func TestEncodeAnchors(t *testing.T) {
	t.Run("anchor in slice", func(t *testing.T) {
		type Person struct {
			*Person `yaml:",omitempty"`
			Name    string `yaml:",omitempty"`
			Age     int    `yaml:",omitempty"`
		}
		defaultPerson := &Person{
			Name: "John Smith",
			Age:  20,
		}
		people := []*Person{
			{
				Person: defaultPerson,
				Name:   "Ken",
				Age:    10,
			},
			defaultPerson,
		}
		var doc struct {
			Default *Person   `yaml:"default,anchor"`
			People  []*Person `yaml:"people"`
		}
		doc.Default = defaultPerson
		doc.People = people
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `default: &default
  name: John Smith
  age: 20
people:
- name: Ken
  age: 10
- *default
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("anchor and alias", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		type T struct {
			A int
			B string
		}
		var v struct {
			A *T `yaml:"a,anchor=c"`
			B *T `yaml:"b,alias=c"`
		}
		v.A = &T{A: 1, B: "hello"}
		v.B = v.A
		if err := enc.Encode(v); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "a: &c\n  a: 1\n  b: hello\nb: *c\n"
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("anchor with auto alias", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		type T struct {
			I int
			S string
		}
		var v struct {
			A *T `yaml:"a,anchor=a"`
			B *T `yaml:"b,anchor=b"`
			C *T `yaml:"c"`
			D *T `yaml:"d"`
		}
		v.A = &T{I: 1, S: "hello"}
		v.B = &T{I: 2, S: "world"}
		v.C = v.A
		v.D = v.B
		if err := enc.Encode(v); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `a: &a
  i: 1
  s: hello
b: &b
  i: 2
  s: world
c: *a
d: *b
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("implicit anchor and alias", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		type T struct {
			I int
			S string
		}
		var v struct {
			A *T `yaml:"a,anchor"`
			B *T `yaml:"b,anchor"`
			C *T `yaml:"c"`
			D *T `yaml:"d"`
		}
		v.A = &T{I: 1, S: "hello"}
		v.B = &T{I: 2, S: "world"}
		v.C = v.A
		v.D = v.B
		if err := enc.Encode(v); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `a: &a
  i: 1
  s: hello
b: &b
  i: 2
  s: world
c: *a
d: *b
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("make unique anchors", func(t *testing.T) {
		type Host struct {
			Hostname string
			Username string
			Password string
		}
		type HostDecl struct {
			Host *Host `yaml:",anchor"`
		}
		type Queue struct {
			Name string `yaml:","`
			Host *Host
		}
		var doc struct {
			Hosts  []*HostDecl `yaml:"hosts"`
			Queues []*Queue    `yaml:"queues"`
		}
		host1 := &Host{
			Hostname: "host1.example.com",
			Username: "userA",
			Password: "pass1",
		}
		host2 := &Host{
			Hostname: "host2.example.com",
			Username: "userB",
			Password: "pass2",
		}
		doc.Hosts = []*HostDecl{
			{Host: host1},
			{Host: host2},
		}
		doc.Queues = []*Queue{
			{
				Name: "queue",
				Host: host1,
			}, {
				Name: "queue2",
				Host: host2,
			},
		}

		var buf bytes.Buffer
		if err := NewEncoder(&buf).Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `hosts:
- host: &host
    hostname: host1.example.com
    username: userA
    password: pass1
- host: &host1
    hostname: host2.example.com
    username: userB
    password: pass2
queues:
- name: queue
  host: *host
- name: queue2
  host: *host1
`
		got := buf.String()
		if got != expect {
			t.Fatalf("expect: [%s], actual: [%s]", expect, got)
		}
	})

	t.Run("anchoring the slice", func(t *testing.T) {
		type Person struct {
			//*Person `yaml:",omitempty"`
			Name string `yaml:",omitempty"`
			Age  int    `yaml:",omitempty"`
		}
		defaultPeople := []Person{
			{Name: "John Smith", Age: 20},
			{Name: "Mary White", Age: 25},
		}
		people := []Person{
			{Name: "Ken", Age: 10},
			{Name: "Ben", Age: 12},
		}
		var doc struct {
			Default []Person `yaml:"default,anchor"`
			People  []Person `yaml:"people"`
			Staff   []Person `yaml:","`
		}
		doc.Default = defaultPeople
		doc.People = people
		doc.Staff = defaultPeople
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `default: &default
- name: John Smith
  age: 20
- name: Mary White
  age: 25
people:
- name: Ken
  age: 10
- name: Ben
  age: 12
staff: *default
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("anchoring the map", func(t *testing.T) {
		type Person struct {
			//*Person `yaml:",omitempty"`
			Name string `yaml:",omitempty"`
			Age  int    `yaml:",omitempty"`
		}
		defaultPeople := map[string]Person{
			"husband": {Name: "John Smith", Age: 20},
			"wife":    {Name: "Mary White", Age: 25},
		}
		var doc struct {
			Default map[string]Person `yaml:"default,anchor"`
			Staff   map[string]Person `yaml:","`
		}
		doc.Default = defaultPeople
		doc.Staff = defaultPeople
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `default: &default
  husband:
    name: John Smith
    age: 20
  wife:
    name: Mary White
    age: 25
staff: *default
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("anchors with aliases inside flow slice", func(t *testing.T) {
		type Person struct {
			*Person `yaml:",omitempty"`
			Name    string `yaml:",omitempty"`
			Age     int    `yaml:",omitempty"`
		}
		defaultPerson := &Person{Name: "John Smith", Age: 20}
		people := []*Person{
			{
				Person: defaultPerson,
				Name:   "Ken",
				Age:    10,
			},
			defaultPerson,
		}
		var doc struct {
			Default *Person   `yaml:"default,anchor"`
			People  []*Person `yaml:"people,flow"`
		}
		doc.Default = defaultPerson
		doc.People = people
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `default: &default
  name: John Smith
  age: 20
people: [{name: Ken, age: 10}, *default]
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("anchors with flow", func(t *testing.T) {
		type Person struct {
			*Person `yaml:",omitempty"`
			Name    string `yaml:",omitempty"`
			Age     int    `yaml:",omitempty"`
		}
		defaultPerson := &Person{Name: "John Smith", Age: 20}
		people := []*Person{
			{
				Person: defaultPerson,
				Name:   "Ken",
				Age:    10,
			},
			defaultPerson,
		}
		var doc struct {
			Default *Person   `yaml:"default,anchor,flow"`
			People  []*Person `yaml:"people,flow"`
		}
		doc.Default = defaultPerson
		doc.People = people
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `default: &default {name: John Smith, age: 20}
people: [{name: Ken, age: 10}, *default]
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})

	t.Run("anchors with global flow", func(t *testing.T) {
		type Person struct {
			*Person `yaml:",omitempty"`
			Name    string `yaml:",omitempty"`
			Age     int    `yaml:",omitempty"`
		}
		defaultPerson := &Person{Name: "John Smith", Age: 20}
		people := []*Person{
			{
				Person: defaultPerson,
				Name:   "Ken",
				Age:    10,
			},
			defaultPerson,
		}
		var doc struct {
			Default *Person   `yaml:"default,anchor"`
			People  []*Person `yaml:"people"`
		}
		doc.Default = defaultPerson
		doc.People = people
		var buf bytes.Buffer
		enc := NewEncoder(&buf).WithFlowStyle(true)
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("%+v", err)
		}
		expect := `{default: &default {name: John Smith, age: 20}, people: [{name: Ken, age: 10}, *default]}
`
		if expect != buf.String() {
			t.Fatalf("expect = [%s], actual = [%s]", expect, buf.String())
		}
	})
}

func TestEncodeQuoted(t *testing.T) {
	tests := []struct {
		source  string
		value   any
		options func(*Encoder) *Encoder
	}{
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
			"v: \".inf\"\n",
			map[string]string{"v": ".inf"},
			nil,
		},
		{
			"v: \".nan\"\n",
			map[string]string{"v": ".nan"},
			nil,
		},
		{
			"v: \"Null\"\n",
			map[string]string{"v": "Null"},
			nil,
		},
		{
			"v: \"\"\n",
			map[string]string{"v": ""},
			nil,
		},
		{
			"v: \"{abc}\"\n",
			map[string]string{"v": "{abc}"},
			nil,
		},
		{
			"v: \"[abc]\"\n",
			map[string]string{"v": "[abc]"},
			nil,
		},
		{
			"v: 'true'\n",
			map[string]string{"v": "true"},
			func(e *Encoder) *Encoder { return e.WithSingleQuote(true) },
		},
		{
			"a: \"-\"\n",
			map[string]string{"a": "-"},
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
			"a: \"1:1\"\n",
			map[string]string{"a": "1:1"},
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
			"a: \"\\\\0\"\n",
			map[string]string{"a": "\\0"},
			nil,
		},
		{
			"a: \"\"\n",
			struct {
				A string
			}{
				"",
			},
			nil,
		},
		{
			"a: \"\"\n",
			struct {
				A *string
			}{
				&emptyStr,
			},
			nil,
		},
		{
			"a:\n  y: \"\"\n",
			struct {
				A *struct {
					X string `yaml:"x,omitempty"`
					Y string
				}
			}{&struct {
				X string `yaml:"x,omitempty"`
				Y string
			}{}},
			nil,
		},
		{
			"a:\n  y: \"\"\n",
			struct {
				A *struct {
					X string `yaml:"x,omitzero"`
					Y string
				}
			}{&struct {
				X string `yaml:"x,omitzero"`
				Y string
			}{}},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			if test.options != nil {
				enc = test.options(enc)
			}
			if err := enc.Encode(test.value); err != nil {
				t.Fatalf("%+v", err)
			}
			if test.source != buf.String() {
				t.Fatalf("expect = [%s], actual = [%s]", test.source, buf.String())
			}
		})
	}
}

type quotedString string

func (s quotedString) MarshalText() ([]byte, error) {
	return []byte(strconv.Quote(string(s))), nil
}

func TestIssue174(t *testing.T) {
	buf := bytes.Buffer{}
	enc := NewEncoder(&buf).WithFlowStyle(true)
	data := map[quotedString][]int{
		"00:00:00-23:59:59": {1, 2, 3},
	}

	if err := enc.Encode(data); err != nil {
		t.Fatal(err)
	}
	expect := `{"00:00:00-23:59:59": [1, 2, 3]}
`
	got := buf.String()
	if got != expect {
		t.Fatalf("expect [%s], got: [%s]", expect, got)
	}
}

func TestIssue259(t *testing.T) {
	type AnchorValue struct {
		Foo uint64
		Bar string
	}

	type Value struct {
		Baz   string       `yaml:"baz"`
		Value *AnchorValue `yaml:"value,anchor"`
	}

	type Schema struct {
		Values []*Value
	}

	schema := Schema{}
	anchorValue := AnchorValue{Foo: 3, Bar: "bar"}
	schema.Values = []*Value{
		{Baz: "xxx", Value: &anchorValue},
		{Baz: "yyy", Value: &anchorValue},
		{Baz: "zzz", Value: &anchorValue},
	}
	b, err := Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	expected := `values:
- baz: xxx
  value: &value
    foo: 3
    bar: bar
- baz: yyy
  value: *value
- baz: zzz
  value: *value`
	got := string(b)
	if expected != got {
		t.Fatalf("expected [%s], got [%s]", expected, got)
	}
}

func TestEncodeSliceOfSlices(t *testing.T) {
	t.Run("slice of slices", func(t *testing.T) {
		bytes, err := Marshal([][]int{
			{1, 2},
			{4, 6},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "- - 1\n  - 2\n- - 4\n  - 6"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
	t.Run("slice of slices of slices", func(t *testing.T) {
		bytes, err := Marshal([][][]int{
			{{1, 2}, {3, 4}},
			{{5, 6}},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "- - - 1\n    - 2\n  - - 3\n    - 4\n- - - 5\n    - 6"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
	t.Run("slice of slices of structs", func(t *testing.T) {
		type T struct {
			A int
			B int
		}
		bytes, err := Marshal([][]T{
			{{1, 2}, {3, 4}},
			{{4, 6}},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "- - a: 1\n    b: 2\n  - a: 3\n    b: 4\n- - a: 4\n    b: 6"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
	t.Run("slice of slices of maps", func(t *testing.T) {
		bytes, err := Marshal([][]map[string]int{
			{{"a": 1, "b": 2}, {"c": 3}},
			{{"d": 4}},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "- - a: 1\n    b: 2\n  - c: 3\n- - d: 4"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
	t.Run("map of slices of slices", func(t *testing.T) {
		bytes, err := Marshal(map[string][][]int{
			"a": {{1, 2}, {4, 6}},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "a:\n- - 1\n  - 2\n- - 4\n  - 6"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
	t.Run("struct of slices of slices", func(t *testing.T) {
		type T struct {
			A [][]int
		}
		bytes, err := Marshal(T{
			A: [][]int{{1, 2}, {4, 6}},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "a:\n- - 1\n  - 2\n- - 4\n  - 6"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
}

func TestEncodeStructOfStruct(t *testing.T) {
	t.Run("struct of struct", func(t *testing.T) {
		type V struct {
			A int
			B int
		}
		type T struct {
			Vi V
		}
		bytes, err := Marshal(T{
			Vi: V{1, 2},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "vi:\n  a: 1\n  b: 2"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
	t.Run("struct of struct of struct", func(t *testing.T) {
		type S struct {
			A int
			B int
		}
		type V struct {
			Si S
		}
		type T struct {
			Vi V
		}
		bytes, err := Marshal(T{
			Vi: V{Si: S{1, 2}},
		})
		if err != nil {
			t.Fatalf("%+v", err)
		}
		expect := "vi:\n  si:\n    a: 1\n    b: 2"
		actual := string(bytes)
		if actual != expect {
			t.Fatalf("unexpected output. expect:[%s] actual:[%s]", expect, actual)
		}
	})
}

func TestEncodeWrongIndent(t *testing.T) {
	type indentTest struct {
		indent   int
		hasError bool
	}
	indents := []indentTest{
		{-2, true},
		{-1, true},
		{0, true},
		{1, true},
		{2, false},
	}
	var buf bytes.Buffer

	for _, ind := range indents {
		enc := NewEncoder(&buf).WithIndent(ind.indent)
		err := enc.Encode("a")
		if ind.hasError && err == nil || !ind.hasError && err != nil {
			t.Fatalf("expected error result for indent: %d", ind.indent)
		}
		buf.Reset()
	}
}
func TestEncodeScalar(t *testing.T) {
	type Test struct {
		source   string
		result   string
		hasError bool
	}
	tests := []Test{
		{"foo", "foo", false},
		{"foo bar", "foo bar", false},
		{"foo bar:", "\"foo bar:\"", false},
		{" foo bar", "\" foo bar\"", false},
		{"foo bar\n", "|\n  foo bar", false},
	}
	var buf bytes.Buffer

	for _, test := range tests {
		res, err := Marshal(test.source)
		if err != nil && !test.hasError {
			t.Fatalf("expected no error for [%s]", test.source)
		}
		if test.result != string(res) {
			t.Errorf("expected [%s] got [%s]", test.result, string(res))
		}
		buf.Reset()
	}
}

func TestEncodeFromExampleV2(t *testing.T) {
	type MetadataEntryV2 struct {
		Name    string
		Size    int64
		Volume  float64
		Enabled bool
		Since   time.Time
		Codes   []int `yaml:",flow"`
		Inf     float64
		Staff   map[string]string
	}
	type TOCV2 struct {
		StatisticsEntries []MetadataEntryV2
	}

	toc := func() TOCV2 {
		size := 2
		toc := TOCV2{}
		now := time.Date(2026, 3, 10, 1, 2, 3, 4, time.UTC)
		for i := range size {
			me := MetadataEntryV2{
				Name:    fmt.Sprintf("Name %d", i),
				Size:    int64(i),
				Volume:  1.1 * float64(i),
				Enabled: i%2 == 0,
				Since:   now.Add(time.Duration(i) * time.Second),
				Codes:   []int{i / 3, i / 2, i, i + 5},
				Inf:     math.Inf(-1 + i%2),
				Staff:   map[string]string{"admin": fmt.Sprintf("Boris %d", i), "chief": fmt.Sprintf("BulletDodger %d", i)},
			}
			toc.StatisticsEntries = append(toc.StatisticsEntries, me)
		}
		return toc
	}()
	expected := `
statisticsentries:
- name: Name 0
  size: 0
  volume: 0.0
  enabled: true
  since: "2026-03-10T01:02:03.000000004Z"
  codes: [0, 0, 0, 5]
  inf: -.inf
  staff:
    admin: Boris 0
    chief: BulletDodger 0
- name: Name 1
  size: 1
  volume: 1.1
  enabled: false
  since: "2026-03-10T01:02:04.000000004Z"
  codes: [0, 0, 1, 6]
  inf: .inf
  staff:
    admin: Boris 1
    chief: BulletDodger 1
`
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(toc); err != nil {
		t.Fatalf("%+v", err)
	}
	got := "\n" + buf.String()
	if expected != got {
		t.Fatalf("diff: %s", cmp.Diff(expected, got))
		// t.Fatalf("expect = [%s], actual = [%s]", expected, buf.String())
	}
}
