package gyaml

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	yml := `
# head comment
key: value # line comment
`
	var v struct {
		Key string
	}

	if err := Unmarshal([]byte(yml), &v); err != nil {
		t.Fatal(err)
	}
	out, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	expected := "key: value"
	got := string(out)
	if expected != got {
		t.Fatalf("failed to get round tripped yaml: %s", cmp.Diff(expected, got))
	}
}

func TestStreamDecoding(t *testing.T) {
	yml := `
# comment
---
a:
  b:
    c: # comment
---
foo: bar # comment
---
- a
- b
- c # comment
`
	dec := NewDecoder(strings.NewReader(yml))
	var vals [3]any
	for i := range vals {
		if err := dec.Decode(&vals[i]); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
	}
	for i := range vals {
		if vals[i] == nil {
			t.Fatalf("unexpected nil parsed value, number: %d", i+1)
		}
	}
	expected := map[string]any{"foo": "bar"}
	if !reflect.DeepEqual(vals[1], expected) {
		t.Fatalf("expected 2nd value: [%+v](%[1]T), got [%+v](%[2]T)", expected, vals[1])
	}
}

func TestDecodeKeepAddress(t *testing.T) {
	data := `
a: &a [_]
b: &b [*a,*a]
c: &c [*b,*b]
d: &d [*c,*c]
`
	var v map[string]any
	if err := Unmarshal([]byte(data), &v); err != nil {
		t.Fatal(err)
	}
	a := v["a"]
	b := v["b"]
	for _, vv := range v["b"].([]any) {
		if fmt.Sprintf("%p", a) != fmt.Sprintf("%p", vv) {
			t.Fatalf("failed to decode b element as keep address")
		}
	}
	for _, vv := range v["c"].([]any) {
		if fmt.Sprintf("%p", b) != fmt.Sprintf("%p", vv) {
			t.Fatalf("failed to decode c element as keep address")
		}
	}
}
