package gyaml

import (
	"strings"
	"testing"
)

func FuzzUnmarshalToMap(f *testing.F) {
	const validYAML = `
id: 1
message: Hello, World
verified: true
`

	invalidYAML := []string{
		"0::",
		"{0",
		"*-0",
		">\n>",
		"&{0",
		"0_",
		"0\n:",
		"0\n-",
		"0\n0",
		"0\n0\n",
		"0\n0\n0",
		"0\n0\n0\n",
		"0\n0\n0\n0",
		"0\n0\n0\n0\n",
		"0\n0\n0\n0\n0",
		"0\n0\n0\n0\n0\n",
		"0\n0\n0\n0\n0\n0",
		"0\n0\n0\n0\n0\n0\n",
		"",
		"00A: 0000A",
		"{\"000\":0000A,",
	}

	f.Add([]byte(validYAML))
	for _, s := range invalidYAML {
		f.Add([]byte(s))
		f.Add([]byte(validYAML + s))
		f.Add([]byte(s + validYAML))
		f.Add([]byte(s + validYAML + s))
		f.Add([]byte(strings.Repeat(s, 3)))
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		v := map[string]any{}
		t.Logf("fuzz testing: [%s]\n", src)
		if err := Unmarshal(src, &v); err != nil {
			t.Log(err.Error())
		}
	})
}

func TestFuzzHanged(t *testing.T) {
	table := []struct {
		source string
	}{
		{"0: 0:"},
	}
	for _, test := range table {
		t.Run(test.source, func(t *testing.T) {
			v := map[string]any{}
			if err := Unmarshal([]byte(test.source), &v); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
