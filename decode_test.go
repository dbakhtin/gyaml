package gyaml

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// tabled test for debugging
func TestCustomTable(t *testing.T) {
	tests := []struct {
		source string
		value  any
	}{
		{
			source: "v: .inf\n",
			value:  map[string]any{"v": math.Inf(0)},
		},
		//
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			typ := reflect.ValueOf(test.value).Type()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil {
				t.Fatalf("%s: %+v", test.source, err)
			}
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}

// test for debugging
func TestCustom(t *testing.T) {
	tests := []struct {
		source string
		value  any
	}{
		//
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			typ := reflect.TypeFor[any]()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil {
				t.Fatalf("%s: %+v", test.source, err)
			}
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}
func TestUnmarshal(t *testing.T) {
	t.Run("document separators", func(t *testing.T) {
		tests := []struct {
			source string
			value  any
		}{
			{
				source: "---\na: b\n",
				value:  map[string]string{"a": "b"},
			},
			{
				source: "---\n",
				value:  (*struct{})(nil),
			},
			{
				source: "--- # comment\n",
				value:  (*struct{})(nil),
			},
			{
				source: "...",
				value:  (*struct{})(nil),
			},
			{
				source: "... # comment",
				value:  (*struct{})(nil),
			},
			{
				source: "a: b\n...\nc: d",
				value:  map[string]string{"a": "b"},
			},
			{
				source: "-a: b\n",
				value:  map[string]string{"-a": "b"},
			},
		}
		for _, test := range tests {
			t.Run(test.source, func(t *testing.T) {
				typ := reflect.ValueOf(test.value).Type()
				value := reflect.New(typ)
				if err := Unmarshal([]byte(test.source), value.Interface()); err != nil && err != io.EOF {
					t.Fatalf("%s: %+v", test.source, err)
				}
				actual := fmt.Sprintf("%+v", value.Elem().Interface())
				expect := fmt.Sprintf("%+v", test.value)
				if actual != expect {
					t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
				}
			})
		}
	})

	t.Run("some border cases from fuzz test", func(t *testing.T) {
		tests := []string{
			"0:\r- \"\":6 \x7f\xff ",
			"0:\r- '0'\r- '$- ''\r- ':",
			"0:\r- 0b0:X\xfe, ",
			"0:\r-   \r--- ",
		}
		for _, test := range tests {
			var v any
			if err := Unmarshal([]byte(test), &v); err != nil {
				t.Errorf("%v", err)
			}
		}
	})
}

func TestUnmarshalScalar(t *testing.T) {
	tnil := (*int)(nil)
	tests := []struct {
		source string
		value  any
	}{
		//ints
		{"12", 12},
		{"-12", -12},
		{"0b0111", 0b111},
		{"0o0734", 0o0734},
		{"0x9f3e", 0x9f3e},
		{"010", 10},
		{"-010", -10},
		{"-0o10", -8},
		{"0010", 10},
		//floats
		{"1.2", 1.2},
		{"-1.2", -1.2},
		{"1.2e2", 1.2e2},
		//strings
		{`"abc"`, "abc"},
		{"abc", "abc"},
		{"'abc'", "abc"},
		{"abc'", "abc'"},
		{"ab'c", "ab'c"},
		{"tru", "tru"},
		{"fals", "fals"},
		{`"\/"`, "/"},
		//bools
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		//nulls
		{"null", tnil},
		{"Null", tnil},
		{"NULL", tnil},
		{"~", tnil},
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			typ := reflect.ValueOf(test.value).Type()
			value := reflect.New(typ)
			if err := Unmarshal([]byte(test.source), value.Interface()); err != nil {
				t.Fatalf("%s: %+v", test.source, err)
			}
			actual := value.Elem().Interface()
			expect := test.value
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}

type Child struct {
	B int
	C int `yaml:"-"`
}
type TestString string

func TestDecoder(t *testing.T) {
	tests := []struct {
		source string
		value  any
	}{
		{
			source: "v: hi\n",
			value:  map[string]string{"v": "hi"},
		},
		{
			source: "v: hi\n",
			value:  map[string]TestString{"v": "hi"},
		},
		{
			source: "v: \"true\"\n",
			value:  map[string]string{"v": "true"},
		},
		{
			source: "v: \"false\"\n",
			value:  map[string]string{"v": "false"},
		},
		{
			source: "v: true\n",
			value:  map[string]any{"v": true},
		},
		{
			source: "v: true\n",
			value:  map[string]string{"v": "true"},
		},
		{
			source: "v: 10\n",
			value:  map[string]string{"v": "10"},
		},
		{
			source: "v: 10\n",
			value:  map[string]TestString{"v": "10"},
		},
		{
			source: "v: -10\n",
			value:  map[string]string{"v": "-10"},
		},
		{
			source: "v: 1.234\n",
			value:  map[string]string{"v": "1.234"},
		},
		{
			source: "v: \" foo\"\n",
			value:  map[string]string{"v": " foo"},
		},
		{
			source: "v: \"foo \"\n",
			value:  map[string]string{"v": "foo "},
		},
		{
			source: "v: \" foo \"\n",
			value:  map[string]string{"v": " foo "},
		},
		{
			source: "v: false\n",
			value:  map[string]bool{"v": false},
		},
		{
			source: "v: 10\n",
			value:  map[string]int{"v": 10},
		},
		{
			source: "v: 10",
			value:  map[string]any{"v": 10},
		},
		{
			source: "v: 0b10",
			value:  map[string]any{"v": 2},
		},
		{
			source: "v: -0b101010",
			value:  map[string]any{"v": -42},
		},
		{
			source: "v: -0b1000000000000000000000000000000000000000000000000000000000000000",
			value:  map[string]any{"v": int64(-9223372036854775808)},
		},
		{
			source: "v: 0xA",
			value:  map[string]any{"v": 10},
		},
		{
			source: "v: .1",
			value:  map[string]any{"v": 0.1},
		},
		{
			source: "v: -.1",
			value:  map[string]any{"v": -0.1},
		},
		{
			source: "v: -10\n",
			value:  map[string]int{"v": -10},
		},
		{
			source: "v: 4294967296\n",
			value:  map[string]int64{"v": int64(4294967296)},
		},
		{
			source: "v: 0.1\n",
			value:  map[string]any{"v": 0.1},
		},
		{
			source: "v: 0.99\n",
			value:  map[string]float32{"v": 0.99},
		},
		{
			source: "v: -0.1\n",
			value:  map[string]float64{"v": -0.1},
		},
		{
			source: "v: 6.8523e+5",
			value:  map[string]any{"v": 6.8523e+5},
		},
		{
			source: "v: 685.23015e+03",
			value:  map[string]any{"v": 685.23015e+03},
		},
		{
			source: "v: 685230.15",
			value:  map[string]any{"v": 685230.15},
		},
		{
			source: "v: 685230.15",
			value:  map[string]float64{"v": 685230.15},
		},
		{
			source: "v: 685230",
			value:  map[string]any{"v": 685230},
		},
		{
			source: "v: +685230",
			value:  map[string]any{"v": 685230},
		},
		{
			source: "v: 02472256",
			value:  map[string]any{"v": 2472256},
		},
		{
			source: "v: 0b10100111010010101110",
			value:  map[string]any{"v": 685230},
		},
		{
			source: "v: +685230",
			value:  map[string]int{"v": 685230},
		},
		{
			source: "v: 0x0A74AE",
			value:  map[string]any{"v": 685230},
		},
		// Bools from spec
		{
			source: "v: True",
			value:  map[string]any{"v": true},
		},
		{
			source: "v: TRUE",
			value:  map[string]any{"v": true},
		},
		{
			source: "v: False",
			value:  map[string]any{"v": false},
		},
		{
			source: "v: FALSE",
			value:  map[string]any{"v": false},
		},
		{
			source: "v: y",
			value:  map[string]any{"v": "y"}, // y or yes or Yes is string
		},
		{
			source: "v: NO",
			value:  map[string]any{"v": "NO"}, // no or No or NO is string
		},
		{
			source: "v: on",
			value:  map[string]any{"v": "on"}, // on is string
		},
		// Some cross type conversions
		{
			source: "v: 42",
			value:  map[string]uint{"v": 42},
		},
		{
			source: "v: 4294967296",
			value:  map[string]uint64{"v": uint64(4294967296)},
		},
		// int
		{
			source: "v: 2147483647",
			value:  map[string]int{"v": math.MaxInt32},
		},
		{
			source: "v: -2147483648",
			value:  map[string]int{"v": math.MinInt32},
		},
		// int64
		{
			source: "v: 9223372036854775807",
			value:  map[string]int64{"v": math.MaxInt64},
		},
		{
			source: "v: 0b111111111111111111111111111111111111111111111111111111111111111",
			value:  map[string]int64{"v": math.MaxInt64},
		},
		{
			source: "v: -9223372036854775808",
			value:  map[string]int64{"v": math.MinInt64},
		},
		{
			source: "v: -0b111111111111111111111111111111111111111111111111111111111111111",
			value:  map[string]int64{"v": -math.MaxInt64},
		},
		// uint
		{
			source: "v: 0",
			value:  map[string]uint{"v": 0},
		},
		{
			source: "v: 4294967295",
			value:  map[string]uint{"v": math.MaxUint32},
		},
		// uint64
		{
			source: "v: 0",
			value:  map[string]uint{"v": 0},
		},
		{
			source: "v: 18446744073709551615",
			value:  map[string]uint64{"v": math.MaxUint64},
		},
		{
			source: "v: 0b1111111111111111111111111111111111111111111111111111111111111111",
			value:  map[string]uint64{"v": math.MaxUint64},
		},
		{
			source: "v: 9223372036854775807",
			value:  map[string]uint64{"v": math.MaxInt64},
		},
		// float32
		{
			source: "v: 3.40282346638528859811704183484516925440e+38",
			value:  map[string]float32{"v": math.MaxFloat32},
		},
		{
			source: "v: 1.401298464324817070923729583289916131280e-45",
			value:  map[string]float32{"v": math.SmallestNonzeroFloat32},
		},
		{
			source: "v: 18446744073709551615",
			value:  map[string]float32{"v": float32(math.MaxUint64)},
		},
		{
			source: "v: 18446744073709551616",
			value:  map[string]float32{"v": float32(math.MaxUint64 + 1)},
		},
		{
			source: "v: 1e-06",
			value:  map[string]float32{"v": 1e-6},
		},
		// float64
		{
			source: "v: 1.797693134862315708145274237317043567981e+308",
			value:  map[string]float64{"v": math.MaxFloat64},
		},
		{
			source: "v: 4.940656458412465441765687928682213723651e-324",
			value:  map[string]float64{"v": math.SmallestNonzeroFloat64},
		},
		{
			source: "v: 18446744073709551615",
			value:  map[string]float64{"v": float64(math.MaxUint64)},
		},
		{
			source: "v: 18446744073709551616",
			value:  map[string]float64{"v": float64(math.MaxUint64 + 1)},
		},
		{
			source: "v: 1e-06",
			value:  map[string]float64{"v": 1e-06},
		},
		// Timestamps
		{
			// Date only.
			source: "v: 2015-01-01\n",
			value:  map[string]time.Time{"v": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		{
			// RFC3339
			source: "v: 2015-02-24T18:19:39.12Z\n",
			value:  map[string]time.Time{"v": time.Date(2015, 2, 24, 18, 19, 39, .12e9, time.UTC)},
		},
		{
			// RFC3339 with short dates.
			source: "v: 2015-2-3T3:4:5Z",
			value:  map[string]time.Time{"v": time.Date(2015, 2, 3, 3, 4, 5, 0, time.UTC)},
		},
		{
			// ISO8601 lower case t
			source: "v: 2015-02-24t18:19:39Z\n",
			value:  map[string]time.Time{"v": time.Date(2015, 2, 24, 18, 19, 39, 0, time.UTC)},
		},
		{
			// space separate, no time zone
			source: "v: 2015-02-24 18:19:39\n",
			value:  map[string]time.Time{"v": time.Date(2015, 2, 24, 18, 19, 39, 0, time.UTC)},
		},
		{
			source: "v: 60s\n",
			value:  map[string]time.Duration{"v": time.Minute},
		},
		{
			source: "v: -0.5h\n",
			value:  map[string]time.Duration{"v": -30 * time.Minute},
		},
		// Single Quoted values.
		{
			source: `'1': '2'`,
			value:  map[string]any{"1": `2`},
		},
		{
			source: `'1': '"2"'`,
			value:  map[string]any{"1": `"2"`},
		},
		{
			source: `'1': ''''`,
			value:  map[string]any{"1": `'`},
		},
		{
			source: `'1': '''2'''`,
			value:  map[string]any{"1": `'2'`},
		},
		{
			source: `'1': 'B''z'`,
			value:  map[string]any{"1": `B'z`},
		},
		{
			source: `'1': '\'`,
			value:  map[string]any{"1": `\`},
		},
		{
			source: `'1': '\\'`,
			value:  map[string]any{"1": `\\`},
		},
		{
			source: `'1': '\u0101'`,
			value:  map[string]any{"1": "\\u0101"},
		},
		{
			source: `
      - 'Fun with \'
      - '\" \a \b \e \f'
      - '\n \r \t \v \0'
      - '\  \_ \N \L \P'
    `,
			value: []string{`Fun with \`, `\" \a \b \e \f`, `\n \r \t \v \0`, `\  \_ \N \L \P`},
		},
		{
			source: `'1': '\n'`,
			value:  map[string]any{"1": "\\n"},
		},
		{
			source: `'1': '\"2\"'`,
			value:  map[string]any{"1": `\"2\"`},
		},
		{
			source: `'1': '\\"2\\"'`,
			value:  map[string]any{"1": `\\"2\\"`},
		},
		//here it differs from go-yaml but corresponds to https://www.yamllint.com
		//go-yaml parses this map value as "   1 2 3" but single-quoted strings should not interpret escape chars
		{
			source: "'1': '   1\n    2\n    3'",
			value:  map[string]any{"1": "   1\\n    2\\n    3"},
		},
		{
			source: "'1': '\n    2\n    3'",
			value:  map[string]any{"1": "\\n    2\\n    3"},
		},
		{
			source: `"1": "a\x2Fb"`,
			value:  map[string]any{"1": `a/b`},
		},
		{
			source: `"1": "a\u002Fb"`,
			value:  map[string]any{"1": `a/b`},
		},
		{
			source: `"1": 'a\u002Fb'`,
			value:  map[string]any{"1": "a\\u002Fb"},
		},
		{
			source: `"1": "a\x2Fb\u002Fc\U0000002Fd"`,
			value:  map[string]any{"1": `a/b/c/d`},
		},
		{
			source: "'1': \"2\\n3\"",
			value:  map[string]any{"1": "2\n3"},
		},
		{
			source: "'1': \"2\\r\\n3\"",
			value:  map[string]any{"1": "2\r\n3"},
		},
		{
			source: "'1': \"a\\\nb\\\nc\"",
			value:  map[string]any{"1": "abc"},
		},
		{
			source: "'1': \"a\\\r\nb\\\r\nc\"",
			value:  map[string]any{"1": "abc"},
		},
		{
			source: "'1': \"a\\\rb\\\rc\"",
			value:  map[string]any{"1": "abc"},
		},

		{
			source: "a: -b_c",
			value:  map[string]any{"a": "-b_c"},
		},
		{
			source: "a: +b_c",
			value:  map[string]any{"a": "+b_c"},
		},
		{
			source: "a: 50cent_of_dollar",
			value:  map[string]any{"a": "50cent_of_dollar"},
		},

		// Unconventional keys
		{
			source: "1: v\n",
			value:  map[int]string{1: "v"},
		},
		{
			source: "1.1: v\n",
			value:  map[float64]string{1.1: "v"},
		},
		{
			source: "true: v\n",
			value:  map[bool]string{true: "v"},
		},
		{
			source: "2015-01-01T00:00:00Z: v\n",
			value:  map[time.Time]string{time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC): "v"},
		},
		{
			source: "40s: v\n",
			value:  map[time.Duration]string{40 * time.Second: "v"},
		},
		// Nulls
		{
			source: "null",
			value:  (*struct{})(nil),
		},
		{
			source: "~",
			value:  (*struct{})(nil),
		},
		{
			source: "v:",
			value:  map[string]any{"v": nil},
		},
		{
			source: "v: ~",
			value:  map[string]any{"v": nil},
		},
		{
			source: "v:",
			value:  map[string]*bool{"v": nil},
		},
		{
			source: "v: null",
			value:  map[string]*string{"v": nil},
		},
		{
			source: "v: null",
			value:  map[string]string{"v": ""},
		},
		{
			source: "v: null",
			value:  map[string]any{"v": nil},
		},
		{
			source: "v: Null",
			value:  map[string]any{"v": nil},
		},
		{
			source: "v: NULL",
			value:  map[string]any{"v": nil},
		},
		{
			source: "v: ~",
			value:  map[string]*string{"v": nil},
		},
		{
			source: "v: ~",
			value:  map[string]string{"v": ""},
		},

		//reserved numbers
		{
			source: "v: .inf\n",
			value:  map[string]any{"v": math.Inf(0)},
		},
		{
			source: "v: .Inf\n",
			value:  map[string]any{"v": math.Inf(0)},
		},
		{
			source: "v: .INF\n",
			value:  map[string]any{"v": math.Inf(0)},
		},
		{
			source: "v: -.inf\n",
			value:  map[string]any{"v": math.Inf(-1)},
		},
		{
			source: "v: -.Inf\n",
			value:  map[string]any{"v": math.Inf(-1)},
		},
		{
			source: "v: -.INF\n",
			value:  map[string]any{"v": math.Inf(-1)},
		},
		{
			source: "v: .nan\n",
			value:  map[string]any{"v": math.NaN()},
		},
		{
			source: "v: .NaN\n",
			value:  map[string]any{"v": math.NaN()},
		},
		{
			source: "v: .NAN\n",
			value:  map[string]any{"v": math.NaN()},
		},
		// Explicit tags.
		{
			source: "v: !!float '1.1'",
			value:  map[string]any{"v": 1.1},
		},
		{
			source: "v: !!float 0",
			value:  map[string]any{"v": float64(0)},
		},
		{
			source: "v: !!float -1",
			value:  map[string]any{"v": float64(-1)},
		},
		{
			source: "v: !!null ''",
			value:  map[string]any{"v": nil},
		},
		{
			source: "v: !!bool False",
			value:  map[string]any{"v": false},
		},
		{
			source: "v: !!str False",
			value:  map[string]any{"v": "False"},
		},
		{
			source: "v: !!str 123",
			value:  map[string]any{"v": "123"},
		},

		// Flow sequence
		{
			source: "v: [A,B]",
			value:  map[string]any{"v": []any{"A", "B"}},
		},
		{
			source: "v: [A,B,C,]",
			value:  map[string][]string{"v": {"A", "B", "C"}},
		},
		{
			source: "v: [A,1,C]",
			value:  map[string][]string{"v": {"A", "1", "C"}},
		},
		{
			source: "v: [A,1,C]",
			value:  map[string]any{"v": []any{"A", 1, "C"}},
		},
		// not supported yet
		// {
		//  source: "v: [a: b, c: d]",
		//  value: map[string]any{"v": []any{
		//    map[string]any{"a": "b"},
		//    map[string]any{"c": "d"},
		//  }},
		// },
		{
			source: "v: [{a: b}, {c: d, e: f}]",
			value: map[string]any{"v": []any{
				map[string]any{"a": "b"},
				map[string]any{
					"c": "d",
					"e": "f",
				},
			}},
		},

		// Block sequence
		{
			source: "- 1",
			value:  []any{1},
		},
		{
			source: "v:\n - A\n - B",
			value:  map[string]any{"v": []any{"A", "B"}},
		},
		{
			source: "v:\n - A\n - B\n - C",
			value:  map[string][]string{"v": {"A", "B", "C"}},
		},
		{
			source: "v:\n - A\n - 1\n - C",
			value:  map[string][]string{"v": {"A", "1", "C"}},
		},
		{
			source: "v:\n - A\n - 1\n - C",
			value:  map[string]any{"v": []any{"A", 1, "C"}},
		},

		// Map inside interface with no type hints.
		{
			source: "a: {b: c}",
			value:  map[string]any{"a": map[any]any{"b": "c"}},
		},

		{
			source: "v: \"\"\n",
			value:  map[string]string{"v": ""},
		},
		{
			source: "v:\n- A\n- B\n",
			value:  map[string][]string{"v": {"A", "B"}},
		},
		{
			source: "a: '-'\n",
			value:  map[string]string{"a": "-"},
		},
		{
			source: "123\n",
			value:  123,
		},
		{
			source: "hello: world\n",
			value:  map[string]string{"hello": "world"},
		},
		{
			source: "hello: world\r\n",
			value:  map[string]string{"hello": "world"},
		},
		{
			source: "hello: world\rGo: Gopher",
			value:  map[string]string{"hello": "world", "Go": "Gopher"},
		},

		// Structs and type conversions.
		{
			source: "hello: world",
			value:  struct{ Hello string }{"world"},
		},
		{
			source: "a: {b: c}",
			value:  struct{ A struct{ B string } }{struct{ B string }{"c"}},
		},
		{
			source: "a: {b: c}",
			value:  struct{ A map[string]string }{map[string]string{"b": "c"}},
		},
		{
			source: "a:",
			value:  struct{ A map[string]string }{},
		},
		{
			source: "a: 1",
			value:  struct{ A int }{1},
		},
		{
			source: "a: 1",
			value:  struct{ A float64 }{1},
		},
		{
			source: "a: [1, 2]",
			value:  struct{ A []int }{[]int{1, 2}},
		},
		{
			source: "a: [1, 2]",
			value:  struct{ A [2]int }{[2]int{1, 2}},
		},
		{
			source: "a: 1",
			value:  struct{ B int }{0},
		},
		{
			source: "a: 1",
			value: struct {
				B int `yaml:"a"`
			}{1},
		},
		{
			source: "v:\n- A\n- 1\n- B:\n  - 2\n  - 3\n",
			value: map[string]any{
				"v": []any{
					"A",
					1,
					map[string][]int{
						"B": {2, 3},
					},
				},
			},
		},
		{
			source: "a:\n  b: c\n",
			value: map[string]any{
				"a": map[string]string{
					"b": "c",
				},
			},
		},
		{
			source: "a: {x: 1}\n",
			value: map[string]map[string]int{
				"a": {
					"x": 1,
				},
			},
		},
		{
			source: "t2: 2018-01-09T10:40:47Z\nt4: 2098-01-09T10:40:47Z\n",
			value: map[string]string{
				"t2": "2018-01-09T10:40:47Z",
				"t4": "2098-01-09T10:40:47Z",
			},
		},
		{
			source: "a: [1, 2]\n",
			value: map[string][]int{
				"a": {1, 2},
			},
		},
		{
			source: "a: {b: c, d: e}\n",
			value: map[string]any{
				"a": map[string]string{
					"b": "c",
					"d": "e",
				},
			},
		},
		{
			source: "a: 3s\n",
			value: map[string]string{
				"a": "3s",
			},
		},
		{
			source: "a: <foo>\n",
			value:  map[string]string{"a": "<foo>"},
		},
		{
			source: "a: \"1:1\"\n",
			value:  map[string]string{"a": "1:1"},
		},
		{
			source: "a: 1.2.3.4\n",
			value:  map[string]string{"a": "1.2.3.4"},
		},
		{
			source: "a: 'b: c'\n",
			value:  map[string]string{"a": "b: c"},
		},
		{
			source: "a: 'Hello #comment'\n",
			value:  map[string]string{"a": "Hello #comment"},
		},
		{
			source: "a: 100.5\n",
			value: map[string]any{
				"a": 100.5,
			},
		},
		{
			source: "a: \"\\f\"\n",
			value:  map[string]string{"a": "\f"},
		},
		{
			source: "a: \"\\0\"\n",
			value:  map[string]string{"a": "\x00"},
		},
		{
			source: "a: \"\\x00\"\n",
			value:  map[string]string{"a": "\x00"},
		},
		{
			source: "a: 1\nsub:\n  e: 5\n",
			value: map[string]any{
				"a": 1,
				"sub": map[string]int{
					"e": 5,
				},
			},
		},
		{
			source: "       a       :          b        \n",
			value:  map[string]string{"a": "b"},
		},
		{
			source: "a: b # comment\nb: c\n",
			value: map[string]string{
				"a": "b",
				"b": "c",
			},
		},
		{
			source: "'a': 'b' # comment\nb: c\n",
			value: map[string]string{
				"a": "b",
				"b": "c",
			},
		},
		{
			source: "a: &a b # comment\nb: *a # comment2\n",
			value: map[string]string{
				"a": "b",
				"b": "b",
			},
		},
		//multiline literals
		{
			source: "|\n  a\n  b",
			value:  "a\nb\n",
		},
		{
			source: "|-\n  a\n  b",
			value:  "a\nb",
		},
		{
			source: "|+\n  a\n  b\n\n",
			value:  "a\nb\n\n",
		},
		{
			source: "|\n  a\n  b\n\n",
			value:  "a\nb\n",
		},
		{
			source: "|-\n  a\n  b\n\n",
			value:  "a\nb",
		},
		{
			source: ">\n  a\n  b",
			value:  "a b\n",
		},
		{
			source: ">-\n  a\n  b",
			value:  "a b",
		},
		{
			source: ">+\n  a\n  b\n\n",
			value:  "a b\n\n",
		},
		{
			source: ">\n  a\n  b\n\n",
			value:  "a b\n",
		},
		{
			source: ">-\n  a\n  b\n\n",
			value:  "a b",
		},
		{
			source: "- |\n  hello\n \n  \n- world",
			value:  []string{"hello\n", "world"},
		},
		//document separators
		{
			source: "---\na: b\n",
			value:  map[string]string{"a": "b"},
		},
		{
			source: "a: b\n...\n",
			value:  map[string]string{"a": "b"},
		},
		// not supported
		// {
		//  source: "%YAML 1.2\n---\n",
		//  value:  (*struct{})(nil),
		//  eof:    true,
		// },
		{
			source: "---\n",
			value:  (*struct{})(nil),
		},
		{
			source: "--- # comment\n",
			value:  (*struct{})(nil),
		},
		{
			source: "...",
			value:  (*struct{})(nil),
		},
		{
			source: "... # comment",
			value:  (*struct{})(nil),
		},
		{
			source: "v: go test ./...",
			value:  map[string]string{"v": "go test ./..."},
		},
		{
			source: "v: echo ---",
			value:  map[string]string{"v": "echo ---"},
		},
		{
			source: "v: |\n  hello\n  ...\n  world\n",
			value:  map[string]string{"v": "hello\n...\nworld\n"},
		},
		{
			source: "v: |\r\n  hello\r\n  ...\r\n  world\r\n",
			value:  map[string]string{"v": "hello\n...\nworld\n"},
		},
		{
			source: "v: |\r  hello\r  ...\r  world\r",
			value:  map[string]string{"v": "hello\n...\nworld\n"},
		},
		{
			source: "v:\n- A\n- |-\n  B\n  C\n",
			value: map[string][]string{
				"v": {
					"A", "B\nC",
				},
			},
		},
		{
			source: "v:\r\n- A\r\n- |-\r\n  B\r\n  C\r\n",
			value: map[string][]string{
				"v": {
					"A", "B\nC",
				},
			},
		},
		{
			source: "v:\r- A\r- |-\r  B\r  C\r",
			value: map[string][]string{
				"v": {
					"A", "B\nC",
				},
			},
		},
		{
			source: "v:\n- A\n- |-\n  B\n  C\n\n\n",
			value: map[string][]string{
				"v": {
					"A", "B\nC",
				},
			},
		},
		{
			source: "v:\n- A\n- >-\n  B\n  C\n",
			value: map[string][]string{
				"v": {
					"A", "B C",
				},
			},
		},
		{
			source: "v:\r\n- A\r\n- >-\r\n  B\r\n  C\r\n",
			value: map[string][]string{
				"v": {
					"A", "B C",
				},
			},
		},
		{
			source: "v:\r- A\r- >-\r  B\r  C\r",
			value: map[string][]string{
				"v": {
					"A", "B C",
				},
			},
		},
		{
			source: "v:\n- A\n- >-\n  B\n  C\n\n\n",
			value: map[string][]string{
				"v": {
					"A", "B C",
				},
			},
		},
		{
			source: "a: b\nc: d\n",
			value: struct {
				A string
				C string `yaml:"c"`
			}{
				"b", "d",
			},
		},
		{
			source: "a: 1\nb: 2\n",
			value: struct {
				A int
				B int `yaml:"-"`
			}{
				1, 0,
			},
		},
		{
			source: "a: 1\nb: 2\n",
			value: struct {
				A int
				Child
			}{
				1,
				Child{
					B: 2,
					C: 0,
				},
			},
		},

		// Anchors and aliases.
		{source: "a: &x 1\nb: &y 2\nc: *x\nd: *y\n",
			value: struct{ A, B, C, D int }{1, 2, 1, 2},
		},
		{source: "a: &x 2s\nc: *x\n",
			value: struct{ A, C time.Duration }{2 * time.Second, 2 * time.Second},
		},
		{
			source: "a: &a {c: 1}\nb: *a\n",
			value: struct {
				A, B struct {
					C int
				}
			}{struct{ C int }{1}, struct{ C int }{1}},
		},
		{
			source: "a: &a [1, 2]\nb: *a\n",
			value:  struct{ A, B []int }{[]int{1, 2}, []int{1, 2}},
		},
		{
			source: `{a: &a c, *a : b}`,
			value:  map[string]string{"a": "c", "c": "b"},
		},
		{
			source: "tags:\n- hello-world\na: foo",
			value: struct {
				Tags []string
				A    string
			}{Tags: []string{"hello-world"}, A: "foo"},
		},
		{
			source: "",
			value:  (*struct{})(nil),
		},
		{
			source: "{}",
			value:  struct{}{},
		},
		{
			source: "{a: , b: c}",
			value:  map[string]any{"a": nil, "b": "c"},
		},
		{
			source: "[a,]",
			value:  []any{"a"},
		},
		{
			source: "v: /a/{b}",
			value:  map[string]string{"v": "/a/{b}"},
		},
		{
			source: "v: 1[]{},!%?&*",
			value:  map[string]string{"v": "1[]{},!%?&*"},
		},
		{
			source: "v: user's item",
			value:  map[string]string{"v": "user's item"},
		},
		{
			source: "v: [1,[2,[3,[4,5],6],7],8]",
			value: map[string]any{
				"v": []any{
					1,
					[]any{
						2,
						[]any{
							3,
							[]int{4, 5},
							6,
						},
						7,
					},
					8,
				},
			},
		},
		{
			source: "v: {a: {b: {c: {d: e},f: g},h: i},j: k}",
			value: map[string]any{
				"v": map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": map[string]string{
								"d": "e",
							},
							"f": "g",
						},
						"h": "i",
					},
					"j": "k",
				},
			},
		},
		{
			source: "---\n- a:\n    b:\n- c: d",
			value: []map[string]any{
				{
					"a": map[string]any{
						"b": nil,
					},
				},
				{
					"c": "d",
				},
			},
		},
		{
			source: "---\na:\n  b:\nc: d",
			value: map[string]any{
				"a": map[string]any{
					"b": nil,
				},
				"c": "d",
			},
		},
		{
			source: "---\na:\nb:\nc:\n",
			value: map[string]any{
				"a": nil,
				"b": nil,
				"c": nil,
			},
		},
		{
			source: "---\na: go test ./...\nb:\nc:",
			value: map[string]any{
				"a": "go test ./...",
				"b": nil,
				"c": nil,
			},
		},
		{
			source: "---\na: |\n  hello\n  ...\n  world\nb:\nc:",
			value: map[string]any{
				"a": "hello\n...\nworld\n",
				"b": nil,
				"c": nil,
			},
		},
		//directives
		{
			source: "%YAML 1.2 # comment\n---\na: b",
			value: map[string]any{
				"a": "b",
			},
		},

		// Multi bytes
		{
			source: "v: あいうえお\nv2: かきくけこ",
			value:  map[string]string{"v": "あいうえお", "v2": "かきくけこ"},
		},
		{
			source: `
- "Fun with \\"
- "\" \a \b \e \f"
- "\n \r \t \v \0"
- "\  \_ \N \L \P \
\x41 \u0041 \U00000041"
    `,
			value: []string{"Fun with \\", "\" \u0007 \b \u001b \f", "\n \r \t \u000b \u0000", "\u0020 \u00a0 \u0085 \u2028 \u2029 A A A"},
		},
		{
			source: `"A \
\x41 \u0041 \U00000041"`,
			value: "A A A A",
		},
		{
			source: `"\ud83e\udd23"`,
			value:  "🤣",
		},
		{
			source: `"\uD83D\uDE00\uD83D\uDE01"`,
			value:  "😀😁",
		},
		{
			source: `"\uD83D\uDE00a\uD83D\uDE01"`,
			value:  "😀a😁",
		},
		{
			source: "42: 100",
			value:  map[string]any{"42": 100},
		},
		{
			source: "42: 100",
			value:  map[int]any{42: 100},
		},
		//multi dimensional arrays
		{
			source: "- [a, b]\n- [c]",
			value:  [][]string{{"a", "b"}, {"c"}},
		},
		//does not work atm, left for later
		// {
		// 	source: "- - a\n  - b\n- - c",
		// 	value:  [][]string{{"a", "b"}, {"c"}},
		// },
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			typ := reflect.ValueOf(test.value).Type()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil && err != io.EOF {
				t.Fatalf("%s: %+v", test.source, err)
			}
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}

func TestDecoderMapAny(t *testing.T) {
	tests := []struct {
		source string
		value  any
		ktype  string //key type
	}{
		{
			source: "v: 1",
			value:  map[any]any{"v": 1},
			ktype:  "string",
		},
		{
			source: "1: 1",
			value:  map[any]any{1: 1},
			ktype:  "int64",
		},
		{
			source: "0.1: 1",
			value:  map[any]any{0.1: 1},
			ktype:  "float64",
		},
		{
			source: ".1: 1",
			value:  map[any]any{0.1: 1},
			ktype:  "float64",
		},
		{
			source: "true: 1",
			value:  map[any]any{true: 1},
			ktype:  "bool",
		},
		{
			source: "tru: 1",
			value:  map[any]any{"tru": 1},
			ktype:  "string",
		},
		{
			source: "null: 1",
			value:  map[any]any{nil: 1},
			ktype:  "",
		},
		{
			source: "~: 1",
			value:  map[any]any{nil: 1},
			ktype:  "",
		},
		{
			source: "0b01: 1",
			value:  map[any]any{1: 1},
			ktype:  "int64",
		},
		{
			source: "-.inf: 1",
			value:  map[any]any{math.Inf(-1): 1},
			ktype:  "float64",
		},
		{
			source: ".nan: 1",
			value:  map[any]any{math.NaN(): 1},
			ktype:  "float64",
		},
		{
			source: "'1': 1",
			value:  map[any]any{"1": 1},
			ktype:  "string",
		},
		//
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			typ := reflect.ValueOf(test.value).Type()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil {
				t.Fatalf("%s: %+v", test.source, err)
			}
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
			if value.Elem().Kind() != reflect.Map {
				t.Fatalf("value must be a map, got: %v", value.Kind())
			}
			keys := value.Elem().MapKeys()
			if len(keys) == 0 {
				t.Fatalf("expected map key type %v, got %v", test.ktype, value.Elem().Type().Key())
			}
			//no idea how to simplify this, it looks like reflect loses exact type when I call v.Set() instead of v.SetBool
			//and similar in func valueAny() or just my lack of knowledge xD
			//straight value.Elem().Type().Key().Name() returns empty
			ktype := reflect.TypeOf(keys[0].Interface()).Name()
			if ktype != test.ktype {
				t.Fatalf("expected map key type %q, got %q", test.ktype, ktype)
			}
		})
	}
}

func TestDecoderInvalid(t *testing.T) {
	tests := []struct {
		src    string
		expect string
	}{
		{
			"*-0",
			"\nyaml: anchor \"-0\" not found, trying to unmarshal \"*-0\" into *interface {}",
		},
		{
			"a:\n- b\n  c: d",
			"\ninvalid character 'c' unexpected end of array",
		},
	}
	for _, test := range tests {
		t.Run(test.src, func(t *testing.T) {
			var v any
			err := Unmarshal([]byte(test.src), &v)
			if err == nil {
				t.Fatal("cannot catch decode error")
			}
			actual := "\n" + err.Error()
			if !strings.HasPrefix(actual, test.expect) {
				t.Fatalf("expected: [%s] but got [%s]", test.expect, actual)
			}
		})
	}
}

func TestDecoderTypeConversionError(t *testing.T) {
	t.Run("type conversion for struct", func(t *testing.T) {
		type T struct {
			A int
			B uint
			C float32
			D bool
		}
		type U struct {
			*T
		}
		t.Run("string to int", func(t *testing.T) {
			var v T
			err := Unmarshal([]byte(`a: str`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "yaml: cannot unmarshal string into Go struct field T.a of type int"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
		t.Run("string to uint", func(t *testing.T) {
			var v T
			err := Unmarshal([]byte(`b: str`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal string into Go struct field T.b of type uint"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
		t.Run("string to bool", func(t *testing.T) {
			var v T
			err := Unmarshal([]byte(`d: str`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal string into Go struct field T.d of type bool"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
		t.Run("string to int", func(t *testing.T) {
			var v U
			err := Unmarshal([]byte(`a: str`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal string into Go struct field U.T.a of type int"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
	})
	t.Run("type conversion for array", func(t *testing.T) {
		t.Run("string to int", func(t *testing.T) {
			var v map[string][]int
			err := Unmarshal([]byte(`v: [A,1,C]`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal string into Go value of type int"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
		t.Run("string to int", func(t *testing.T) {
			var v map[string][]int
			err := Unmarshal([]byte("v:\n - A\n - 1\n - C"), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal string into Go value of type int"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
	})
	t.Run("overflow error", func(t *testing.T) {
		t.Run("negative number to uint", func(t *testing.T) {
			var v map[string]uint
			err := Unmarshal([]byte("v: -42"), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal number -42 into Go value of type uint"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
			if v["v"] != 0 {
				t.Fatal("failed to decode value")
			}
		})
		t.Run("negative number to uint64", func(t *testing.T) {
			var v map[string]uint64
			err := Unmarshal([]byte("v: -4294967296"), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal number -4294967296 into Go value of type uint64"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
			if v["v"] != 0 {
				t.Fatal("failed to decode value")
			}
		})
		t.Run("larger number for int32", func(t *testing.T) {
			var v map[string]int32
			err := Unmarshal([]byte("v: 4294967297"), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal number 4294967297 into Go value of type int32"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
			if v["v"] != 0 {
				t.Fatal("failed to decode value")
			}
		})
		t.Run("larger number for int8", func(t *testing.T) {
			var v map[string]int8
			err := Unmarshal([]byte("v: 128"), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal number 128 into Go value of type int8"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
			if v["v"] != 0 {
				t.Fatal("failed to decode value")
			}
		})
	})
	t.Run("type conversion for time", func(t *testing.T) {
		type T struct {
			A time.Time
			B time.Duration
		}
		t.Run("int to time", func(t *testing.T) {
			var v T
			err := Unmarshal([]byte(`a: 123`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal 123 into Go struct field T.a of type time.Time"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
		t.Run("string to duration", func(t *testing.T) {
			var v T
			err := Unmarshal([]byte(`b: str`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal str into Go struct field T.b of type time.Duration"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
		t.Run("int to duration", func(t *testing.T) {
			var v T
			err := Unmarshal([]byte(`b: 10`), &v)
			if err == nil {
				t.Fatal("expected to error")
			}
			msg := "cannot unmarshal 10 into Go struct field T.b of type time.Duration"
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error message: %s to contain: %s", err.Error(), msg)
			}
		})
	})
}

func TestDecoderEmbedded(t *testing.T) {
	t.Run("basic embedding", func(t *testing.T) {
		type Base struct {
			A int
			B string
		}
		yml := `---
  a: 1
  b: hello
  c: true
  `
		var v struct {
			*Base
			C bool
		}
		if err := NewDecoder(strings.NewReader(yml)).Decode(&v); err != nil {
			t.Fatalf("%+v", err)
		}
		if v.A != 1 || v.B != "hello" || !v.C {
			t.Fatal("failed to decode with embedded struct")
		}
	})

	t.Run("nested embedding", func(t *testing.T) {
		type Base struct {
			A int
			B string
		}
		type Base2 struct {
			*Base
		}
		yml := `---
  a: 1
  b: hello
  `
		var v struct {
			*Base2
		}
		if err := NewDecoder(strings.NewReader(yml)).Decode(&v); err != nil {
			t.Fatalf("%+v", err)
		}
		if v.A != 1 || v.B != "hello" {
			t.Fatal("failed to decode with embedded struct")
		}
	})
}

func TestDecoderEmbeddedConflict(t *testing.T) {
	type Base struct {
		A int
		B string
	}
	yml := `---
a: 1
b: hello
c: true
`
	var v struct {
		*Base
		A int
		C bool
	}
	if err := NewDecoder(strings.NewReader(yml)).Decode(&v); err != nil {
		t.Fatalf("%+v", err)
	}
	if v.A != 1 || v.B != "hello" || !v.C || v.Base.A != 0 {
		t.Fatal("failed to decode embedded struct with conflict key")
	}
}

func TestDecoderDisallowUnknownField(t *testing.T) {
	t.Run("allow unknown field", func(t *testing.T) {
		type Base struct {
			A int
			B string
		}
		yml := "a: 1\nunknown: true"
		var v Base
		if err := NewDecoder(strings.NewReader(yml)).Decode(&v); err != nil {
			t.Fatalf("%+v", err)
		}
		if v.A != 1 {
			t.Fatal("failed to decode with allowed unknown field")
		}
	})

	t.Run("different level keys with same name", func(t *testing.T) {
		var v struct {
			C Child
		}
		yml := `---
  b: 1
  c:
    b: 1
  `

		dec := NewDecoder(strings.NewReader(yml))
		dec.DisallowUnknownFields()
		err := dec.Decode(&v)
		if err == nil {
			t.Fatalf("error expected")
		}
	})
	t.Run("embedded", func(t *testing.T) {
		var v struct {
			*Child
			A string
		}
		yml := `---
  a: a
  b: 1
  `

		dec := NewDecoder(strings.NewReader(yml))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			t.Fatalf(`parsing should succeed: %s`, err)
		}
		if v.A != "a" {
			t.Fatalf("v.A should be `a`, got `%s`", v.A)
		}
		if v.B != 1 {
			t.Fatalf("v.B should be 1, got %d", v.B)
		}
		if v.C != 0 {
			t.Fatalf("v.C should be 0, got %d", v.C)
		}
	})
	t.Run("list", func(t *testing.T) {
		type C struct {
			Child
		}

		var v struct {
			Children []C
		}

		yml := `---
children:
- b: 1
- b: 2
`

		dec := NewDecoder(strings.NewReader(yml))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			t.Fatalf(`parsing should succeed: %s`, err)
		}

		if len(v.Children) != 2 {
			t.Errorf("%+v", v.Children)
			t.Fatalf(`len(v.Children) should be 2, got %d`, len(v.Children))
		}

		if v.Children[0].B != 1 {
			t.Fatalf(`v.Children[0].B should be 1, got %d`, v.Children[0].B)
		}

		if v.Children[1].B != 2 {
			t.Fatalf(`v.Children[1].B should be 2, got %d`, v.Children[1].B)
		}
	})
}

type unmarshalableYAMLStringValue string

func (v *unmarshalableYAMLStringValue) UnmarshalYAML(b []byte) error {
	var s string
	if err := Unmarshal(b, &s); err != nil {
		return err
	}
	*v = unmarshalableYAMLStringValue(s)
	return nil
}

type unmarshalableTextStringValue string

func (v *unmarshalableTextStringValue) UnmarshalText(b []byte) error {
	*v = unmarshalableTextStringValue(string(b))
	return nil
}

type unmarshalableStringContainer struct {
	A unmarshalableYAMLStringValue `yaml:"a"`
	B unmarshalableTextStringValue `yaml:"b"`
}

func TestUnmarshalableString(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		yml := `
a: ""
b: ""
`
		var container unmarshalableStringContainer
		if err := Unmarshal([]byte(yml), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.A != "" {
			t.Fatalf("expected empty string, got %q", container.A)
		}
		if container.B != "" {
			t.Fatalf("expected empty string, got %q", container.B)
		}
	})
	t.Run("filled string", func(t *testing.T) {
		t.Parallel()
		yml := `
a: "aaa"
b: "bbb"
`
		var container unmarshalableStringContainer
		if err := Unmarshal([]byte(yml), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.A != "aaa" {
			t.Fatalf("expected \"aaa\", got %q", container.A)
		}
		if container.B != "bbb" {
			t.Fatalf("expected \"bbb\", got %q", container.B)
		}
	})
	t.Run("single-quoted string", func(t *testing.T) {
		t.Parallel()
		yml := `
a: 'aaa'
b: 'bbb'
`
		var container unmarshalableStringContainer
		if err := Unmarshal([]byte(yml), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.A != "aaa" {
			t.Fatalf("expected \"aaa\", got %q", container.A)
		}
		if container.B != "bbb" {
			t.Fatalf("expected \"aaa\", got %q", container.B)
		}
	})
	t.Run("literal", func(t *testing.T) {
		t.Parallel()
		yml := `
a: |
 a
 b
 c
b: |
 a
 b
 c
`
		var container unmarshalableStringContainer
		if err := Unmarshal([]byte(yml), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.A != "a\nb\nc\n" {
			t.Fatalf("expected \"a\nb\nc\n\", got %q", container.A)
		}
		//B does not parse the multiline and returns as is see B's type definition
		if container.B != "|\n a\n b\n c\n" {
			t.Fatalf("expected |\" a\n b\n c\n\", got %q", container.B)
		}
	})
	t.Run("anchor/alias", func(t *testing.T) {
		yml := `
a: &x 1
b: *x
c: &y hello
d: *y
`
		var v struct {
			A, B, C, D unmarshalableTextStringValue
		}
		if err := Unmarshal([]byte(yml), &v); err != nil {
			t.Fatal(err)
		}
		if v.A != "1" || v.B != "1" || v.C != "hello" || v.D != "hello" {
			t.Fatal("failed to unmarshal")
		}
	})
	t.Run("net.IP", func(t *testing.T) {
		yml := `
a: &a 127.0.0.1
b: *a
`
		var v struct {
			A, B net.IP
		}
		if err := Unmarshal([]byte(yml), &v); err != nil {
			t.Fatal(err)
		}
		if v.A.String() != net.IPv4(127, 0, 0, 1).String() {
			t.Fatal("failed to unmarshal")
		}
		if v.B.String() != net.IPv4(127, 0, 0, 1).String() {
			t.Fatal("failed to unmarshal")
		}
	})
	t.Run("quoted map keys", func(t *testing.T) {
		t.Parallel()
		yml := `
a:
  "b"  : 2
  'c': true
`
		var v struct {
			A struct {
				B int
				C bool
			}
		}
		if err := Unmarshal([]byte(yml), &v); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if v.A.B != 2 {
			t.Fatalf("expected a.b == 2, got %d", v.A.B)
		}
		if !v.A.C {
			t.Fatal("expected a.c == true, got false")
		}
	})
}

type unmarshalablePtrStringContainer struct {
	V *string `yaml:"value"`
}

func TestUnmarshalablePtrString(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		var container unmarshalablePtrStringContainer
		if err := Unmarshal([]byte(`value: ""`), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.V == nil || *container.V != "" {
			t.Fatalf("expected empty string, but %q is set", *container.V)
		}
	})

	t.Run("null", func(t *testing.T) {
		t.Parallel()
		var container unmarshalablePtrStringContainer
		if err := Unmarshal([]byte(`value: null`), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.V != (*string)(nil) {
			t.Fatalf("expected nil, but %q is set", *container.V)
		}
	})
}

type unmarshalableIntValue int

func (v *unmarshalableIntValue) UnmarshalYAML(raw []byte) error {
	i, err := strconv.Atoi(string(raw))
	if err != nil {
		return err
	}
	*v = unmarshalableIntValue(i)
	return nil
}

type unmarshalableIntContainer struct {
	V unmarshalableIntValue `yaml:"value"`
}

func TestUnmarshalableInt(t *testing.T) {
	t.Run("empty int", func(t *testing.T) {
		t.Parallel()
		var container unmarshalableIntContainer
		if err := Unmarshal([]byte(``), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.V != 0 {
			t.Fatalf("expected empty int, but %d is set", container.V)
		}
	})
	t.Run("non-empty int", func(t *testing.T) {
		t.Parallel()
		var container unmarshalableIntContainer
		if err := Unmarshal([]byte(`value: 9`), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.V != 9 {
			t.Fatalf("expected 9, but %d is set", container.V)
		}
	})
}

type unmarshalablePtrIntContainer struct {
	V *int `yaml:"value"`
}

func TestUnmarshalablePtrInt(t *testing.T) {
	t.Run("empty int", func(t *testing.T) {
		t.Parallel()
		var container unmarshalablePtrIntContainer
		if err := Unmarshal([]byte(`value: 0`), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.V == nil || *container.V != 0 {
			t.Fatalf("expected 0, but %q is set", *container.V)
		}
	})

	t.Run("null", func(t *testing.T) {
		t.Parallel()
		var container unmarshalablePtrIntContainer
		if err := Unmarshal([]byte(`value: null`), &container); err != nil {
			t.Fatalf("failed to unmarshal %v", err)
		}
		if container.V != (*int)(nil) {
			t.Fatalf("expected nil, but %q is set", *container.V)
		}
	})
}

type literalContainer struct {
	v string
}

func (c *literalContainer) UnmarshalYAML(v []byte) error {
	var lit string
	if err := Unmarshal(v, &lit); err != nil {
		return err
	}
	c.v = lit
	return nil
}

func TestDecodeMultilineLiteral(t *testing.T) {
	yml := `value: |
  {
     "key": "value"
  }
`
	var v map[string]*literalContainer
	if err := Unmarshal([]byte(yml), &v); err != nil {
		t.Fatalf("failed to unmarshal %+v", err)
	}
	if v["value"] == nil || !strings.Contains(v["value"].v, `"key": "value"`) {
		t.Fatal("failed to unmarshal literal with bytes unmarshaler")
	}
}

func TestDecoderStream(t *testing.T) {
	yml := `
---
a: b
c: d
---
e: f
g: h
---
i: j
k: l
`
	dec := NewDecoder(strings.NewReader(yml))
	values := []map[string]string{}
	for {
		var v map[string]string
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("%+v", err)
		}
		values = append(values, v)
	}
	if len(values) != 3 {
		t.Fatal("failed to stream decoding")
	}
	if values[0]["a"] != "b" {
		t.Fatal("failed to stream decoding")
	}
	if values[1]["e"] != "f" {
		t.Fatal("failed to stream decoding")
	}
	if values[2]["i"] != "j" {
		t.Fatal("failed to stream decoding")
	}
}

func TestDecoderWithAnchorAnyValue(t *testing.T) {
	t.Run("[]any to []string", func(t *testing.T) {
		type Config struct {
			Env []string `json:"env"`
		}

		type Schema struct {
			Def    map[string]any `json:"def"`
			Config Config         `json:"config"`
		}

		data := `
def:
  myenv: &my_env
    - VAR1=1
    - VAR2=2
config:
  env: *my_env
`

		var cfg Schema
		if err := NewDecoder(strings.NewReader(data)).Decode(&cfg); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg.Config.Env, []string{"VAR1=1", "VAR2=2"}) {
			t.Fatalf("failed to decode value. actual = %+v", cfg)
		}
	})

	t.Run("map[any]any to map[string]string", func(t *testing.T) {
		type Config struct {
			Env map[string]string
		}

		type Schema struct {
			Def    map[string]any `json:"def"`
			Config Config         `json:"config"`
		}

		data := `
def:
  myenv: &my_env
    a: b
    c: d
config:
  env: *my_env
`

		var cfg Schema
		if err := NewDecoder(strings.NewReader(data)).Decode(&cfg); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg.Config.Env, map[string]string{"a": "b", "c": "d"}) {
			t.Fatalf("failed to decode value. actual = %+v", cfg)
		}
	})
}

func TestDecoderLiteralWithNewLine(t *testing.T) {
	type A struct {
		Node     string `yaml:"b"`
		LastNode string `yaml:"last"`
	}
	tests := []A{
		{
			Node: "hello\nworld",
		},
		{
			Node: "hello\nworld\n",
		},
		{
			LastNode: "hello\nworld",
		},
		{
			LastNode: "hello\nworld\n",
		},
	}
	// struct(want) -> Marshal -> Unmarchal -> struct(got)
	for _, want := range tests {
		bytes, _ := Marshal(want)
		got := A{}
		if err := Unmarshal(bytes, &got); err != nil {
			t.Fatal(err)
		}
		if want.Node != got.Node {
			t.Fatalf("expected:%q but got %q", want.Node, got.Node)
		}
		if want.LastNode != got.LastNode {
			t.Fatalf("expected:%q but got %q", want.LastNode, got.LastNode)
		}
	}
}

func TestDecoderTabCharacterAtRight(t *testing.T) {
	yml := `
- a: [2 , 2]      
  b: [2 , 2]      
  c: [2 , 2]`
	var v []map[string][]int
	if err := Unmarshal([]byte(yml), &v); err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 {
		t.Fatalf("failed to unmarshal %+v", v)
	}
	if len(v[0]) != 3 {
		t.Fatalf("failed to unmarshal %+v", v)
	}
}

func TestDecoderSameAnchor(t *testing.T) {
	yml := `
a: &a 1
b: &a 2
c: &a 3
d: *a
`
	type T struct {
		A int
		B int
		C int
		D int
	}
	var v T
	if err := Unmarshal([]byte(yml), &v); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v, T{A: 1, B: 2, C: 3, D: 3}) {
		t.Fatalf("failed to decode same anchor: %+v", v)
	}
}

func TestDecoderAnchorInterface(t *testing.T) {
	tests := []struct {
		source string
		value  any
	}{
		{
			source: "a: &a b # comment\nb: *a # comment2\n",
			value: map[any]any{
				"a": "b",
				"b": "b",
			},
		},
		//though anchor is int it is converted to string by objectInterface() logic atm, see comments to function
		{
			source: "a: &a 1\n*a: 2\n",
			value: map[any]any{
				"a": "1",
				"1": "2",
			},
		},
		{
			source: "- &a a #comment\n- *a # comment2\n",
			value:  []any{"a", "a"},
		},
		{
			source: "- c\n- &a 1 #comment\n- *a # comment2\n",
			value:  []any{"c", 1, 1},
		},
		//
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			//by decoding into value of type [any] we force the use of object/value/literalInterface functions which have simplified
			//logic compared to ordinary object()/value()/storeLiteral funcs
			typ := reflect.TypeFor[any]()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil {
				t.Fatalf("%s: %+v", test.source, err)
			}
			//comparing string representation makes "1" == 1, but manual go-cmp/cmp.Diff tells all types are correct
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}

func TestDecoderSameName(t *testing.T) {
	type X struct {
		X float64
	}

	type T struct {
		X
	}

	var v T
	if err := Unmarshal([]byte(`x: 0.7`), &v); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(v.X.X) != "0.7" {
		t.Fatalf("failed to decode")
	}
}

type unmarshableMapKey struct {
	Key string
}

func (mk *unmarshableMapKey) UnmarshalYAML(b []byte) error {
	mk.Key = string(b)
	return nil
}

func TestDecoderMapKeyUnmarshaler(t *testing.T) {
	var m map[unmarshableMapKey]string
	if err := Unmarshal([]byte(`key: value`), &m); err != nil {
		t.Fatalf("failed to unmarshal %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 element in map, but got %d", len(m))
	}
	val, ok := m[unmarshableMapKey{Key: "key"}]
	if !ok {
		t.Fatal("expected to have element 'key' in map")
	}
	if val != "value" {
		t.Fatalf("expected to have value \"value\", but got %q", val)
	}
}

func TestDecoderMultilineIndents(t *testing.T) {
	source := "|\n a\n  b\n c"
	expected := "a\n b\nc\n"
	var s string
	if err := Unmarshal([]byte(source), &s); err != nil {
		t.Fatalf("%v", err)
	}
	if expected != s {
		t.Fatalf("expected [%q], got [%q]", expected, s)
	}
}

func TestDecoderRealExamples(t *testing.T) {
	t.Run("sequence", func(t *testing.T) {
		source := `
--- # Favorite movies
- Casablanca
- North by Northwest
- The Man Who Wasn't There
`
		expected := []string{"Casablanca", "North by Northwest", "The Man Who Wasn't There"}
		var v []string
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v, expected) {
			t.Fatalf("expected [%v], got [%v]", expected, v)
		}
	})
	t.Run("flow sequence", func(t *testing.T) {
		source := `
--- # Shopping list
[milk, pumpkin pie, eggs, juice]
`
		expected := []string{"milk", "pumpkin pie", "eggs", "juice"}
		var v []string
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v, expected) {
			t.Fatalf("expected [%v], got [%v]", expected, v)
		}
	})
	t.Run("anchors/aliases", func(t *testing.T) {
		source := `
--- # Sequencer protocols for Laser eye surgery
- step:  &id001                  # defines anchor label &id001
    instrument:      Lasik 2000
    pulseEnergy:     5.4
    pulseDuration:   12
    repetition:      1000
    spotSize:        1mm

- step: &id002
    instrument:      Lasik 2000
    pulseEnergy:     5.0
    pulseDuration:   10
    repetition:      500
    spotSize:        2mm
- Instrument1: *id001   # refers to the first step (with anchor &id001)
- Instrument2: *id002   # refers to the second step
`
		// expected := []string{"milk", "pumpkin pie", "eggs", "juice"}
		var v []map[any]any
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		if len(v) != 4 {
			t.Fatalf("expected slice length 4, got %d", len(v))
		}
		instrum1, ok := v[2]["Instrument1"]
		if !ok {
			t.Fatalf("Instrument1 not found")
		}
		m1, ok := instrum1.(map[string]any)
		if !ok {
			t.Fatalf("expected Instrument1 type map[string]any, got: %T", instrum1)
		}
		if m1["spotSize"] != "1mm" {
			t.Fatalf("expected Instrument1.spotSize [%v], got [%v]", "1mm", m1["spotSize"])
		}
		instrum2, ok := v[3]["Instrument2"]
		if !ok {
			t.Fatalf("Instrument2 not found")
		}
		m2, ok := instrum2.(map[string]any)
		if !ok {
			t.Fatalf("expected Instrument2 type map[string]any, got: %T", instrum2)
		}
		if m2["spotSize"] != "2mm" {
			t.Fatalf("expected Instrument2.spotSize [%v], got [%v]", "2mm", m2["spotSize"])
		}
	})
	t.Run("anchors/aliases", func(t *testing.T) {
		source := `
---
receipt:     Oz-Ware Purchase Invoice
date:        2012-08-06
customer:
    first_name:   Dorothy
    family_name:  Gale

items:
    - part_no:   A4786
      descrip:   Water Bucket (Filled)
      price:     1.47
      quantity:  4

    - part_no:   E1628
      descrip:   High Heeled "Ruby" Slippers
      size:      8
      price:     133.7
      quantity:  1

bill-to:  &id001
    street: |
            123 Tornado Alley
            Suite 16
    city:   East Centerville
    state:  KS

ship-to:  *id001

specialDelivery:  >
    Follow the Yellow Brick
    Road to the Emerald City.
    Pay no attention to the
    man behind the curtain.
...
  `
		var v map[any]any
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		if len(v) != 7 {
			t.Fatalf("expected slice length 7, got %d", len(v))
		}
		shipto, ok := v["ship-to"]
		if !ok {
			t.Fatalf("ship-to not found")
		}
		m, ok := shipto.(map[string]any)
		if !ok {
			t.Fatalf("expected ship-to type map[string]any, got: %T", shipto)
		}
		if m["state"] != "KS" {
			t.Fatalf("expected ship-to.state [%v], got [%v]", "KS", m["state"])
		}
		sd, ok := v["specialDelivery"].(string)
		if !ok {
			t.Fatalf("expected specialDelivery to be string, got %T", v["specialDelivery"])
		}
		if !strings.Contains(sd, "Brick Road") {
			t.Fatalf("expected specialDelivery to contain %q, got %v", "Brick Road", v["specialDelivery"])
		}
	})
	t.Run("anchors/aliases struct", func(t *testing.T) {
		source := `
---
receipt:     Oz-Ware Purchase Invoice
date:        2012-08-06
customer:
    first_name:   Dorothy
    family_name:  Gale

items:
    - part_no:   A4786
      descrip:   Water Bucket (Filled)
      price:     1.47
      quantity:  4

    - part_no:   E1628
      descrip:   High Heeled "Ruby" Slippers
      size:      8
      price:     133.7
      quantity:  1

bill-to:  &id001
    street: |
            123 Tornado Alley
            Suite 16
    city:   East Centerville
    state:  KS

ship-to:  *id001

specialDelivery:  >
    Follow the Yellow Brick
    Road to the Emerald City.
    Pay no attention to the
    man behind the curtain.
...
  `
		type Customer struct {
			FirstName  string `yaml:"first_name"`
			FamilyName string `yaml:"family_name"`
		}
		type Item struct {
			PartNo   string `yaml:"part_no"`
			Descrip  string
			Price    float64
			Quantity int
			Size     float64
		}
		type Address struct {
			Street string
			City   string
			State  string
		}
		type Invoice struct {
			Receipt         string
			Date            time.Time
			Customer        Customer
			Items           []Item
			BillTo          Address `yaml:"bill-to"`
			ShipTo          Address `yaml:"ship-to"`
			SpecialDelivery string  `yaml:"specialDelivery"`
		}
		var v Invoice
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		expected := Invoice{
			Receipt:  "Oz-Ware Purchase Invoice",
			Date:     time.Date(2012, 8, 6, 0, 0, 0, 0, time.UTC),
			Customer: Customer{FirstName: "Dorothy", FamilyName: "Gale"},
			Items: []Item{
				{PartNo: "A4786", Descrip: "Water Bucket (Filled)", Price: 1.47, Quantity: 4},
				{PartNo: "E1628", Descrip: "High Heeled \"Ruby\" Slippers", Price: 133.7, Quantity: 1, Size: 8},
			},
			BillTo: Address{
				Street: "123 Tornado Alley\nSuite 16\n",
				City:   "East Centerville",
				State:  "KS",
			},
			ShipTo: Address{
				Street: "123 Tornado Alley\nSuite 16\n",
				City:   "East Centerville",
				State:  "KS",
			},
			SpecialDelivery: "Follow the Yellow Brick Road to the Emerald City. Pay no attention to the man behind the curtain.\n",
		}
		if !cmp.Equal(expected, v) {
			t.Fatalf("results not equal, diff:\n%s", cmp.Diff(expected, v))
		}
	})
	t.Run("html struct", func(t *testing.T) {
		source := `
---
example: >
        HTML goes into YAML without modification
message: |

        <blockquote style="font: italic 1em serif">
        <p>"Three is always greater than two,
           even for large values of two"</p>
        <p>--Author Unknown</p>
        </blockquote>
date: 2007-06-01
  `
		type Html struct {
			Example string
			Message string
			Date    time.Time
		}
		var v Html
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		expected := Html{
			Example: "HTML goes into YAML without modification\n",
			Message: `
<blockquote style="font: italic 1em serif">
<p>"Three is always greater than two,
   even for large values of two"</p>
<p>--Author Unknown</p>
</blockquote>
`,
			Date: time.Date(2007, 6, 1, 0, 0, 0, 0, time.UTC),
		}
		if !cmp.Equal(expected, v) {
			t.Fatalf("results not equal, diff:\n%s", cmp.Diff(expected, v))
		}
	})
	t.Run("example from benchmark v2", func(t *testing.T) {
		source := `
statisticsentries:
- name: Name 0
  size: 0
  volume: 0.0
  enabled: true
  since: 2026-03-12T15:42:51.486059642+03:00
  codes: [0, 0, 0, 5]
  inf: -.inf
  staff:
    admin: Boris 0
    chief: BulletDodger 0
- name: Name 1
  size: 1
  volume: 1.1
  enabled: false
  since: 2026-03-12T15:42:51.486059642+03:00
  codes: [0, 0, 1, 6]
  inf: .inf
  staff:
    admin: Boris 1
    chief: BulletDodger 1
`
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

		var v TOCV2
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		// if !cmp.Equal(expected, v) {
		// 	t.Fatalf("results not equal, diff:\n%s", cmp.Diff(expected, v))
		// }
	})
	t.Run("example from awesome-home-assistant", func(t *testing.T) {
		source := `
# Project information
site_name: "Awesome Home Assistant"
site_url: "https://www.awesome-ha.com"
site_description: "A curated list of awesome Home Assistant resources for automating every aspect of your home"
site_author: "Franck Nijhof"
copyright: "Copyright 2018 - 2022 - Franck Nijhof. Creative Commons Attribution 4.0."

# Repository
repo_name: "awesome-home-assistant"
repo_url: "https://github.com/frenck/awesome-home-assistant"

# Theme configuration
theme:
  name: "material"
  logo: "https://www.awesome-ha.com/images/icon.svg"
  language: "en"
  palette:
    # Palette toggle for dark mode
    - scheme: slate
      primary: "light-blue"
      accent: "pink"
      toggle:
        icon: material/brightness-4
        name: Switch to light mode

    # Palette toggle for light mode
    - scheme: default
      primary: "light-blue"
      accent: "pink"
      toggle:
        icon: material/brightness-7 
        name: Switch to dark mode

  features:
    - navigation.expand
    - navigation.instant
    - navigation.tabs
    - navigation.tracking
    - search.highlight
    - search.suggest
    - toc.integrate
extra_css:
  - css/extra.css

# Customization
extra:
  social:
    - icon: fontawesome/brands/github-alt
      link: "https://github.com/frenck"
    - icon: fontawesome/brands/twitter
      link: "https://twitter.com/frenck"
    - icon: fontawesome/brands/instagram
      link: "https://instagram.com/frenck"
    - icon: fontawesome/brands/twitch
      link: "https://twitch.tv/frenck"
    - icon: fontawesome/brands/youtube
      link: "https://youtube.com/frenck"
    - icon: fontawesome/brands/linkedin
      link: "https://www.linkedin.com/in/frenck"

# Extensions
markdown_extensions:
  - toc:
      permalink: true
  - pymdownx.betterem:
      smart_enable: all
  - pymdownx.caret
  - pymdownx.critic
  - pymdownx.details
  - pymdownx.emoji:
      emoji_generator: !!python/name:pymdownx.emoji.to_svg
  - pymdownx.inlinehilite
  - pymdownx.magiclink
  - pymdownx.mark
  - pymdownx.smartsymbols
  - pymdownx.superfences
  - pymdownx.tasklist:
      custom_checkbox: true
  - pymdownx.tilde
  - mdx_truly_sane_lists

# The pages to serve
nav:
  - "The Awesome List": "index.md"
  - "Contributing": "contributing.md"

plugins:
  - git-revision-date-localized:
      type: timeago
`
		var v map[string]any
		if err := NewDecoder(strings.NewReader(source)).Decode(&v); err != nil {
			t.Fatal(err)
		}
	})
}
