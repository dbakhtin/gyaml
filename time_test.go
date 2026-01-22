package gyaml

import "testing"

func TestIsTimestamp(t *testing.T) {
	tests := []struct {
		s      string
		result bool
	}{
		{"2006-01-02T15:04:05.999999999Z", true},
		{"2006-01-02t15:04:05.999999999Z", true},
		{"2006-01-02t15:04:05.999999999ZB", false},
		{"2006-01-02 15:04:05", true},
		{"2006-01-02 15:04:05A", false},
		{"2006-01-02", true},
		{"2006-01-02C", false},
		{"15:4", true},
		{"1:4", true},
		{"1", false},
		{"15:4F", false},
	}
	for _, test := range tests {
		got := isTimestamp(test.s)
		if got != test.result {
			t.Errorf("expected [%v] got [%v] isTimeStamp for %q", test.result, got, test.s)
		}
	}
}
