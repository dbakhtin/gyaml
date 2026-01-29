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

type DecoderOptions struct {
	ReferenceFiles       []string
	ReferenceDirs        []string
	IsRecursiveDir       bool
	UseOrderedMap        bool
	AllowDuplicateMapKey bool
	AllowedFieldPrefixes []string
	DisallowUnknownField bool
}

func DefaultDecoderOptions() DecoderOptions {
	return DecoderOptions{}
}
