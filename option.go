package gyaml

// TODO: consider using a bitmap for bool values (and move map* to encoder)?
type encoderOptions struct {
	singleQuote      bool
	isFlowStyle      bool
	isJSONStyle      bool
	useJSONMarshaler bool
	//each time there is an anchor name collision add suffix numbers to anchor name, true by default
	//enableSmartAnchor bool
	// anchors & aliases
	// aliasRefToName map[uintptr]string
	anchors     map[uintptr]string
	anchorNames map[string]uintptr
	//anchorCallback             func(*ast.AnchorNode, interface{}) error
	//customMarshalerMap         map[reflect.Type]func(any) ([]byte, error)
	omitZero                   bool
	omitEmpty                  bool
	autoInt                    bool
	useLiteralStyleIfMultiline bool
	// commentMap                 map[*Path][]*Comment
	indentSequence bool

	//default indent size in spaces
	indentNum int

	//local options
	level       int
	nestedStack []int //replace int with nestedState & push everytime in struct/slice/map and pop in defered
}

func defaultEncoderOptions() encoderOptions {
	return encoderOptions{
		indentNum: 2,
		//enableSmartAnchor: true,
		anchors:     make(map[uintptr]string),
		anchorNames: make(map[string]uintptr),
	}
}

/*
// DecodeOption functional option type for Decoder
type DecodeOption func(d *Decoder) error

// CustomMarshaler overrides any encoding process for the type specified in generics.
//
// NOTE: If type T implements MarshalYAML for pointer receiver, the type specified in CustomMarshaler must be *T.
// If RegisterCustomMarshaler and CustomMarshaler of EncodeOption are specified for the same type,
// the CustomMarshaler specified in EncodeOption takes precedence.
func CustomMarshaler[T any](marshaler func(T) ([]byte, error)) EncodeOption {
	return func(e *Encoder) error {
		var typ T
		e.customMarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}) ([]byte, error) {
			return marshaler(v.(T))
		}
		return nil
	}
}

// CustomMarshalerContext overrides any encoding process for the type specified in generics.
// Similar to CustomMarshaler, but allows passing a context to the marshaler function.
func CustomMarshalerContext[T any](marshaler func(context.Context, T) ([]byte, error)) EncodeOption {
	return func(e *Encoder) error {
		var typ T
		e.customMarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}) ([]byte, error) {
			return marshaler(ctx, v.(T))
		}
		return nil
	}
}

// WithComment add a comment using the location and text information given in the CommentMap.
func WithComment(cm CommentMap) EncodeOption {
	return func(e *Encoder) error {
		commentMap := map[*Path][]*Comment{}
		for k, v := range cm {
			//path, err := PathString(k)
			_ = k
			path, err := &Path{}, error(nil)
			if err != nil {
				return err
			}
			commentMap[path] = v
		}
		e.commentMap = commentMap
		return nil
	}
}

// CommentToMap apply the position and content of comments in a YAML document to a CommentMap.
func CommentToMap(cm CommentMap) DecodeOption {
	return func(d *Decoder) error {
		if cm == nil {
			return ErrInvalidCommentMapValue
		}
		//d.toCommentMap = cm
		return nil
	}
}
*/
