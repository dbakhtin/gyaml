package gyaml

import "testing"

func TestIsNumber(t *testing.T) {
	tests := []struct {
		s      string
		result bool
	}{
		{"1", true},
		{"100_000", true},
		{"100_000.1", true},
		{"100_000e-1", true},
		{"-1", true},
		{"1.1", true},
		{"1.1.1", false},
		{".05", true},
		{"0.05", true},
		{"+0.05", true},
		{"0xffff", true},
		{"-0xffff", true},
		{"0b101", true},
		{"0o123", true},
		{"_1", false},
		{"1:1", false},
		{"a123", false},
		{" 123", false},
		{"/1", false},
	}
	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			got := isNumber(test.s)
			if got != test.result {
				t.Errorf("expected [%v] got [%v] isNumber for %q", test.result, got, test.s)
			}
		})
	}
}

func BenchmarkIsNumber(b *testing.B) {
	data := []string{
		"1",
		"100_000",
		"100_000.1",
		"100_000e-1",
		"-1",
		"1.1",
		"1.1.1",
		".05",
		"0.05",
		"+0.05",
		"0xffff",
		"-0xffff",
		"0b101",
		"0o123",
		"_1",
		"1:1",
		"a123",
		" 123",
		"/1",
	}
	b.Run("isNumber", func(b *testing.B) {
		for b.Loop() {
			for j := range data {
				_ = isNumber(data[j])
			}
		}
	})
}
