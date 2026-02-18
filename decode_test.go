package gyaml

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestCustom2(t *testing.T) {
	tests := []struct {
		source string
		value  any
		eof    bool
	}{
		// {
		// 	source: "v: [a: b, c: d]",
		// 	value: map[string]any{"v": []any{
		// 		map[string]any{"a": "b"},
		// 		map[string]any{"c": "d"},
		// 	}},
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
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			typ := reflect.ValueOf(test.value).Type()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil {
				if test.eof && err == io.EOF {
					return
				}
				t.Fatalf("%s: %+v", test.source, err)
			}
			if test.eof {
				t.Fatal("expected EOF but got no error")
			}
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}

func TestCustom(t *testing.T) {
	// tnil := (*int)(nil)
	tests := []struct {
		source string
		value  any
	}{
		{`'1\n 2'`, "1\\n 2"},
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

func TestUnmarshalScalar(t *testing.T) {
	tnil := (*int)(nil)
	tests := []struct {
		source string
		value  any
	}{
		//ints
		{"12", 12},
		{"-12", -12},
		{"1_2", 12},
		{"0b01_11", 0b111},
		{"0o07_34", 0o0734},
		{"0x9f_3e", 0x9f3e},
		//floats
		{"1.2", 1.2},
		{"-1.2", -1.2},
		{"1.2_3", 1.23},
		{"1_2.2_3", 12.23},
		{"1.2e2", 1.2e2},
		//strings
		{`"abc"`, "abc"},
		{"abc", "abc"},
		{"'abc'", "abc"},
		{"abc'", "abc'"},
		{"ab'c", "ab'c"},
		{"tru", "tru"},
		{"fals", "fals"},
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

type TestString string

func TestDecoder(t *testing.T) {
	tests := []struct {
		source string
		value  any
		eof    bool
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
		// {
		// 			source: "v: .1",
		// 			value:  map[string]any{"v": 0.1},
		// 		},
		// 		{
		// 			source: "v: -.1",
		// 			value:  map[string]any{"v": -0.1},
		// 		},
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
			source: "v: 685.230_15e+03",
			value:  map[string]any{"v": 685.23015e+03},
		},
		{
			source: "v: 685_230.15",
			value:  map[string]any{"v": 685230.15},
		},
		{
			source: "v: 685_230.15",
			value:  map[string]float64{"v": 685230.15},
		},
		{
			source: "v: 685230",
			value:  map[string]any{"v": 685230},
		},
		{
			source: "v: +685_230",
			value:  map[string]any{"v": 685230},
		},
		// {
		// 	source: "v: 02472256",
		// 	value:  map[string]any{"v": 685230},
		// },
		{
			source: "v: 0b1010_0111_0100_1010_1110",
			value:  map[string]any{"v": 685230},
		},
		{
			source: "v: +685_230",
			value:  map[string]int{"v": 685230},
		},
		{
			source: "v: 0x_0A_74_AE",
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
		// {
		// 	source: "v: 1e3",
		// 	value:  map[string]uint{"v": 1000},
		// },
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
		// {
		// 	source: "v: 1e3",
		// 	value:  map[string]uint64{"v": 1000},
		// },
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
		//need to update the scanner
		// {
		// 	source: `'1': ''''`,
		// 	value:  map[string]any{"1": `'`},
		// },
		// {
		// 	source: `'1': '''2'''`,
		// 	value:  map[string]any{"1": `'2'`},
		// },
		// {
		// 	source: `'1': 'B''z'`,
		// 	value:  map[string]any{"1": `B'z`},
		// },
		{
			source: `'1': '\'`,
			value:  map[string]any{"1": `\`},
		},
		{
			source: `'1': '\\'`,
			value:  map[string]any{"1": `\\`},
		},
		{
			source: `'1': '\"2\"'`,
			value:  map[string]any{"1": `\"2\"`},
		},
		{
			source: `'1': '\\"2\\"'`,
			value:  map[string]any{"1": `\\"2\\"`},
		},
		//here differs from go-yaml but corresponds to https://yaml-online-parser.appspot.com/
		// {
		// 	source: "'1': '   1\n    2\n    3'",
		// 	value:  map[string]any{"1": "   1\\n    2\\n    3"},
		// },
		// {
		// 	source: "'1': '\n    2\n    3'",
		// 	value:  map[string]any{"1": "\\n    2\\n    3"},
		// },
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
		// {
		// 	source: "'1': \"a\\\nb\\\nc\"",
		// 	value:  map[string]any{"1": "abc"},
		// },
		// {
		// 	source: "'1': \"a\\\r\nb\\\r\nc\"",
		// 	value:  map[string]any{"1": "abc"},
		// },
		// {
		// 	source: "'1': \"a\\\rb\\\rc\"",
		// 	value:  map[string]any{"1": "abc"},
		// },

		// {
		// 	source: "a: -b_c",
		// 	value:  map[string]any{"a": "-b_c"},
		// },
		// {
		// 	source: "a: +b_c",
		// 	value:  map[string]any{"a": "+b_c"},
		// },
		// {
		// 	source: "a: 50cent_of_dollar",
		// 	value:  map[string]any{"a": "50cent_of_dollar"},
		// },

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
		// {
		// 	source: "~: null key",
		// 	value:  map[any]string{nil: "null key"},
		// },
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
		// 		{
		// 			source: "v: !!float '1.1'",
		// 			value:  map[string]any{"v": 1.1},
		// 		},
		// 		{
		// 			source: "v: !!float 0",
		// 			value:  map[string]any{"v": float64(0)},
		// 		},
		// 		{
		// 			source: "v: !!float -1",
		// 			value:  map[string]any{"v": float64(-1)},
		// 		},
		// 		{
		// 			source: "v: !!null ''",
		// 			value:  map[string]any{"v": nil},
		// 		},
		// 		{
		// 			source: "v: !!timestamp \"2015-01-01\"",
		// 			value:  map[string]time.Time{"v": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
		// 		},
		// 		{
		// 			source: "v: !!timestamp 2015-01-01",
		// 			value:  map[string]time.Time{"v": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
		// 		},
		// 		{
		// 			source: "v: !!bool yes",
		// 			value:  map[string]bool{"v": true},
		// 		},
		// 		{
		// 			source: "v: !!bool False",
		// 			value:  map[string]bool{"v": false},
		// 		},
		// 		{
		// 			source: `
		// !!merge <<: { a: 1, b: 2 }
		// c: 3
		// `,
		// 			value: map[string]any{"a": 1, "b": 2, "c": 3},
		// 		},

		// merge
		// 		{
		// 			source: `
		// a: &a
		//  foo: 1
		// b: &b
		//  bar: 2
		// merge:
		//  <<: [*a, *b]
		// `,
		// 			value: map[string]map[string]any{
		// 				"a":     {"foo": 1},
		// 				"b":     {"bar": 2},
		// 				"merge": {"foo": 1, "bar": 2},
		// 			},
		// 		},
		// 		{
		// 			source: `
		// a: &a
		//  foo: 1
		// b: &b
		//  bar: 2
		// merge:
		//  <<: [*a, *b]
		// `,
		// 			value: map[string]yaml.MapSlice{
		// 				"a":     {{Key: "foo", Value: 1}},
		// 				"b":     {{Key: "bar", Value: 2}},
		// 				"merge": {{Key: "foo", Value: 1}, {Key: "bar", Value: 2}},
		// 			},
		// 		},

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
		// see notes, need to update the scanner
		// {
		// 	source: "v: [a: b, c: d]",
		// 	value: map[string]any{"v": []any{
		// 		map[string]any{"a": "b"},
		// 		map[string]any{"c": "d"},
		// 	}},
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
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			buf := bytes.NewBufferString(test.source)
			dec := NewDecoder(buf)
			typ := reflect.ValueOf(test.value).Type()
			value := reflect.New(typ)
			if err := dec.Decode(value.Interface()); err != nil {
				if test.eof && err == io.EOF {
					return
				}
				t.Fatalf("%s: %+v", test.source, err)
			}
			if test.eof {
				t.Fatal("expected EOF but got no error")
			}
			actual := fmt.Sprintf("%+v", value.Elem().Interface())
			expect := fmt.Sprintf("%+v", test.value)
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}
