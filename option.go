package gyaml

const DefaultIndentSize = 2

// EncoderOptions are global encoder options
type EncoderOptions struct {
	SingleQuote           bool
	FlowStyle             bool
	JSONStyle             bool
	OmitZero              bool
	OmitEmpty             bool
	AutoInt               bool
	LiteralStyleMultiline bool
	IndentSequence        bool

	//default indent size in spaces
	IndentSize int
}

func DefaultEncoderOptions() EncoderOptions {
	return EncoderOptions{IndentSize: DefaultIndentSize}
}
