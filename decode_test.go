package gyaml

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestCustom(t *testing.T) {
	tnil := (*int)(nil)
	tests := []struct {
		source string
		value  any
	}{
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
			actual := value.Elem().Interface()
			expect := test.value
			if actual != expect {
				t.Fatalf("failed to test [%s], actual=[%s], expect=[%s]", test.source, actual, expect)
			}
		})
	}
}
