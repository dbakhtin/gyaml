package yamloptions

// EncOptions is a bitmap of encoder options
type EncOptions int32

const (
	SingleQuote EncOptions = 2 << iota
	FlowStyle
	JSONStyle
	UseJSONMarshaler
	OmitZero
	OmitEmpty
	AutoInt
	UseLiteralStyleIfMultiline
	IndentSequence
)

func Options(opts ...EncOptions) EncOptions {
	var options EncOptions
	for _, o := range opts {
		options |= o
	}
	return options
}
