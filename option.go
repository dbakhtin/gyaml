package gyaml

// TODO: consider using a bitmap for bool values (and move map* to encoder)?
type encoderOptions struct {
	singleQuote bool
	isFlowStyle bool
	isJSONStyle bool
	//each time there is an anchor name collision add suffix numbers to anchor name, true by default
	// anchors & aliases
	// aliasRefToName map[uintptr]string
	anchors                    map[uintptr]string
	anchorNames                map[string]uintptr
	omitZero                   bool
	omitEmpty                  bool
	autoInt                    bool
	useLiteralStyleIfMultiline bool
	indentSequence             bool

	//default indent size in spaces
	indentNum int

	//local options
	level int
}

func defaultEncoderOptions() encoderOptions {
	return encoderOptions{
		indentNum: 2,
		//enableSmartAnchor: true,
		anchors:     make(map[uintptr]string),
		anchorNames: make(map[string]uintptr),
	}
}
