package gyaml

// EncodeOption functional option type for Encoder
type EncodeOption func(e *Encoder) error

// Indent change indent number
func Indent(spaces int) EncodeOption {
	return func(e *Encoder) error {
		e.indentNum = spaces
		return nil
	}
}

// IndentSequence causes sequence values to be indented the same value as Indent
func IndentSequence(indent bool) EncodeOption {
	return func(e *Encoder) error {
		e.indentSequence = indent
		return nil
	}
}
