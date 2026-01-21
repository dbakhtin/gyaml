package gyaml

const DefaultIndentSize = 2

// encoderOptions are global encoder options
type encoderOptions struct {
	singleQuote           bool
	flowStyle             bool
	JSONStyle             bool
	omitZero              bool
	omitEmpty             bool
	autoInt               bool
	literalStyleMultiline bool
	indentSequence        bool

	//default indent size in spaces
	indentSize int
}

func defaultEncoderOptions() encoderOptions {
	return encoderOptions{indentSize: DefaultIndentSize}
}
