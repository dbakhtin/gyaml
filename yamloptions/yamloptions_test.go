package yamloptions

import "testing"

func TestOptions(t *testing.T) {
	type dataStruct struct {
		input    []EncOptions
		expected EncOptions
	}
	data := []dataStruct{
		{nil, 0},
		{[]EncOptions{}, 0},
		{[]EncOptions{SingleQuote}, SingleQuote},
		{[]EncOptions{FlowStyle}, FlowStyle},
		{[]EncOptions{AutoInt}, AutoInt},
		{[]EncOptions{SingleQuote, FlowStyle, AutoInt}, SingleQuote | AutoInt | FlowStyle},
		{[]EncOptions{SingleQuote, OmitZero, FlowStyle, AutoInt, UseLiteralStyleIfMultiline}, SingleQuote | OmitZero | AutoInt | UseLiteralStyleIfMultiline | FlowStyle},
	}

	for _, d := range data {
		t.Run("bitmap option", func(t *testing.T) {
			got := Options(d.input...)
			if d.expected != got {
				t.Errorf("Expected %v got %v", d.expected, got)
			}
		})
	}
}
