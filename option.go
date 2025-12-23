package gyaml

// EncodeOption functional option type for Encoder
type EncodeOption func(e *Encoder) error

// IndentSequence causes sequence values to be indented the same value as Indent
func IndentSequence(indent bool) EncodeOption {
	return func(e *Encoder) error {
		//e.indentSequence = indent
		return nil
	}
}

// UseLiteralStyleIfMultiline causes encoding multiline strings with a literal syntax,
// no matter what characters they include
func UseLiteralStyleIfMultiline(useLiteralStyleIfMultiline bool) EncodeOption {
	return func(e *Encoder) error {
		//e.useLiteralStyleIfMultiline = useLiteralStyleIfMultiline
		return nil
	}
}

// OmitEmpty behaves in the same way as the interpretation of the omitempty tag in the encoding/json library.
// set on all the fields.
// In the current implementation, the omitempty tag is not implemented in the same way as encoding/json,
// so please specify this option if you expect the same behavior.
func OmitEmpty() EncodeOption {
	return func(e *Encoder) error {
		//e.omitEmpty = true
		return nil
	}
}

// OmitZero forces the encoder to assume an `omitzero` struct tag is
// set on all the fields. See `Marshal` commentary for the `omitzero` tag logic.
func OmitZero() EncodeOption {
	return func(e *Encoder) error {
		//e.omitZero = true
		return nil
	}
}

// UseSingleQuote determines if single or double quotes should be preferred for strings.
func UseSingleQuote(sq bool) EncodeOption {
	return func(e *Encoder) error {
		//e.singleQuote = sq
		return nil
	}
}
