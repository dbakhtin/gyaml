package gyaml

import (
	"encoding/json"
	"testing"
)

type Child struct {
	B int
	C int `yaml:"-"`
}

type TestString string

func TestUnmarshalScalar(t *testing.T) {
	// tests := []struct {
	// 	source string
	// 	value  interface{}
	// 	eof    bool
	// }{}
	// for _, test := range tests {
	// 	_ = test
	// }

	// var num int
	// if err := Unmarshal([]byte("123"), &num); err != nil {
	// 	t.Error(err)
	// }
	// expected := 123
	// if expected != num {
	// 	t.Errorf("expected [%v] got [%v]", expected, num)
	// }
}

func TestDecoderJSON(t *testing.T) {
	var num int
	if err := json.Unmarshal([]byte("123"), &num); err != nil {
		t.Error(err)
	}
	expected := 123
	if expected != num {
		t.Errorf("expected [%v] got [%v]", expected, num)
	}
}
