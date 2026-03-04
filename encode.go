// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package yaml implements encoding and decoding of YAML as defined in RFC 7159.
// The mapping between YAML and Go values is described in the documentation for
// the Marshal and Unmarshal functions.
//
// See "YAML and Go" for an introduction to this package:
// https://golang.org/doc/articles/yaml_and_go.html
//
// # Security Considerations
//
// The YAML standard (RFC 7159) is lax in its definition of a number of parser
// behaviors. As such, many YAML parsers behave differently in various
// scenarios. These differences in parsers mean that systems that use multiple
// independent YAML parser implementations may parse the same YAML object in
// differing ways.
//
// Systems that rely on a YAML object being parsed consistently for security
// purposes should be careful to understand the behaviors of this parser, as
// well as how these behaviors may cause interoperability issues with other
// parser implementations.
//
// Due to the Go Backwards Compatibility promise (https://go.dev/doc/go1compat)
// there are a number of behaviors this package exhibits that may cause
// interopability issues, but cannot be changed. In particular the following
// parsing behaviors may cause issues:
//
//   - If a YAML object contains duplicate keys, keys are processed in the order
//     they are observed, meaning later values will replace or be merged into
//     prior values, depending on the field type (in particular maps and structs
//     will have values merged, while other types have values replaced).
//   - When parsing a YAML object into a Go struct, keys are considered in a
//     case-insensitive fashion.
//   - When parsing a YAML object into a Go struct, unknown keys in the YAML
//     object are ignored (unless a [Decoder] is used and
//     [Decoder.DisallowUnknownFields] has been called).
//   - Invalid UTF-8 bytes in YAML strings are replaced by the Unicode
//     replacement character.
//   - Large YAML number integers will lose precision when unmarshaled into
//     floating-point types.
package gyaml

import (
	"bytes"
	"cmp"
	"encoding"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	_ "unsafe" // for linkname
)

// Marshal returns the YAML encoding of v.
//
// Marshal traverses the value v recursively.
// If an encountered value implements [Marshaler]
// and is not a nil pointer, Marshal calls [Marshaler.MarshalYAML]
// to produce YAML. If no [Marshaler.MarshalYAML] method is present but the
// value implements [encoding.TextMarshaler] instead, Marshal calls
// [encoding.TextMarshaler.MarshalText] and encodes the result as a YAML string.
// The nil pointer exception is not strictly necessary
// but mimics a similar, necessary exception in the behavior of
// [Unmarshaler.UnmarshalYAML].
//
// Otherwise, Marshal uses the following type-dependent default encodings:
//
// Boolean values encode as YAML booleans.
//
// Floating point, integer, and [Number] values encode as YAML numbers.
// NaN and +/-Inf values will return an [UnsupportedValueError].
//
// String values encode as YAML strings coerced to valid UTF-8,
// replacing invalid bytes with the Unicode replacement rune.
// So that the YAML will be safe to embed inside HTML <script> tags,
// the string is encoded using [HTMLEscape],
// which replaces "<", ">", "&", U+2028, and U+2029 are escaped
// to "\u003c","\u003e", "\u0026", "\u2028", and "\u2029".
// This replacement can be disabled when using an [Encoder],
// by calling [Encoder.SetEscapeHTML](false).
//
// Array and slice values encode as YAML arrays, except that
// []byte encodes as a base64-encoded string, and a nil slice
// encodes as the null YAML value.
//
// Struct values encode as YAML objects.
// Each exported struct field becomes a member of the object, using the
// field name as the object key, unless the field is omitted for one of the
// reasons given below.
//
// The encoding of each struct field can be customized by the format string
// stored under the "yaml" key in the struct field's tag.
// The format string gives the name of the field, possibly followed by a
// comma-separated list of options. The name may be empty in order to
// specify options without overriding the default field name.
//
// The "omitempty" option specifies that the field should be omitted
// from the encoding if the field has an empty value, defined as
// false, 0, a nil pointer, a nil interface value, and any array,
// slice, map, or string of length zero.
//
// As a special case, if the field tag is "-", the field is always omitted.
// Note that a field with name "-" can still be generated using the tag "-,".
//
// Examples of struct field tags and their meanings:
//
//	// Field appears in YAML as key "myName".
//	Field int `yaml:"myName"`
//
//	// Field appears in YAML as key "myName" and
//	// the field is omitted from the object if its value is empty,
//	// as defined above.
//	Field int `yaml:"myName,omitempty"`
//
//	// Field appears in YAML as key "Field" (the default), but
//	// the field is skipped if empty.
//	// Note the leading comma.
//	Field int `yaml:",omitempty"`
//
//	// Field is ignored by this package.
//	Field int `yaml:"-"`
//
//	// Field appears in YAML as key "-".
//	Field int `yaml:"-,"`
//
// The "omitzero" option specifies that the field should be omitted
// from the encoding if the field has a zero value, according to rules:
//
// 1) If the field type has an "IsZero() bool" method, that will be used to
// determine whether the value is zero.
//
// 2) Otherwise, the value is zero if it is the zero value for its type.
//
// If both "omitempty" and "omitzero" are specified, the field will be omitted
// if the value is either empty or zero (or both).
//
// The "string" option signals that a field is stored as YAML inside a
// YAML-encoded string. It applies only to fields of string, floating point,
// integer, or boolean types. This extra level of encoding is sometimes used
// when communicating with JavaScript programs:
//
//	Int64String int64 `yaml:",string"`
//
// The key name will be used if it's a non-empty string consisting of
// only Unicode letters, digits, and ASCII punctuation except quotation
// marks, backslash, and comma.
//
// Embedded struct fields are usually marshaled as if their inner exported fields
// were fields in the outer struct, subject to the usual Go visibility rules amended
// as described in the next paragraph.
// An anonymous struct field with a name given in its YAML tag is treated as
// having that name, rather than being anonymous.
// An anonymous struct field of interface type is treated the same as having
// that type as its name, rather than being anonymous.
//
// The Go visibility rules for struct fields are amended for YAML when
// deciding which field to marshal or unmarshal. If there are
// multiple fields at the same level, and that level is the least
// nested (and would therefore be the nesting level selected by the
// usual Go rules), the following extra rules apply:
//
// 1) Of those fields, if any are YAML-tagged, only tagged fields are considered,
// even if there are multiple untagged fields that would otherwise conflict.
//
// 2) If there is exactly one field (tagged or not according to the first rule), that is selected.
//
// 3) Otherwise there are multiple fields, and all are ignored; no error occurs.
//
// Handling of anonymous struct fields is new in Go 1.1.
// Prior to Go 1.1, anonymous struct fields were ignored. To force ignoring of
// an anonymous struct field in both current and earlier versions, give the field
// a YAML tag of "-".
//
// Map values encode as YAML objects. The map's key type must either be a
// string, an integer type, or implement [encoding.TextMarshaler]. The map keys
// are sorted and used as YAML object keys by applying the following rules,
// subject to the UTF-8 coercion described for string values above:
//   - keys of any string type are used directly
//   - keys that implement [encoding.TextMarshaler] are marshaled
//   - integer keys are converted to strings
//
// Pointer values encode as the value pointed to.
// A nil pointer encodes as the null YAML value.
//
// Interface values encode as the value contained in the interface.
// A nil interface value encodes as the null YAML value.
//
// Channel, complex, and function values cannot be encoded in YAML.
// Attempting to encode such a value causes Marshal to return
// an [UnsupportedTypeError].
//
// YAML cannot represent cyclic data structures and Marshal does not
// handle them. Passing cyclic structures to Marshal will result in
// an error.
func Marshal(v any) ([]byte, error) {
	return MarshalWithOptions(v, DefaultEncoderOptions())
}

func MarshalWithOptions(v any, opts EncoderOptions) ([]byte, error) {
	e := newEncodeState()
	defer encodeStatePool.Put(e)

	err := e.marshal(v, opts)
	if err != nil {
		return nil, err
	}
	//Marshal does not terminate with \n like Encoder does
	// e.WriteByte('\n')

	b := append([]byte(nil), e.Bytes()...)
	//strip \n prefix
	if len(b) != 0 && b[0] == '\n' {
		b = b[1:]
	}

	return b, nil
}

// Marshaler is the interface implemented by types that
// can marshal themselves into valid YAML.
type Marshaler interface {
	MarshalYAML() ([]byte, error)
}

// An UnsupportedTypeError is returned by [Marshal] when attempting
// to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "unknown value type " + e.Type.String()
}

// An UnsupportedValueError is returned by [Marshal] when attempting
// to encode an unsupported value.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "yaml: unsupported value: " + e.Str
}

// Before Go 1.2, an InvalidUTF8Error was returned by [Marshal] when
// attempting to encode a string value with invalid UTF-8 sequences.
// As of Go 1.2, [Marshal] instead coerces the string to valid UTF-8 by
// replacing invalid bytes with the Unicode replacement rune U+FFFD.
//
// Deprecated: No longer used; kept for compatibility.
type InvalidUTF8Error struct {
	S string // the whole string value that caused the error
}

func (e *InvalidUTF8Error) Error() string {
	return "yaml: invalid UTF-8 in string: " + strconv.Quote(e.S)
}

// A MarshalerError represents an error from calling a
// [Marshaler.MarshalYAML] or [encoding.TextMarshaler.MarshalText] method.
type MarshalerError struct {
	Type       reflect.Type
	Err        error
	sourceFunc string
}

func (e *MarshalerError) Error() string {
	srcFunc := e.sourceFunc
	if srcFunc == "" {
		srcFunc = "MarshalYAML"
	}
	return "yaml: error calling " + srcFunc +
		" for type " + e.Type.String() +
		": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *MarshalerError) Unwrap() error { return e.Err }

// An encodeState encodes YAML into a bytes.Buffer.
type encodeState struct {
	bytes.Buffer // accumulated output

	// Keep track of what pointers we've seen in the current recursive call
	// path, to avoid cycles that could lead to a stack overflow. Only do
	// the relatively expensive map operations if ptrLevel is larger than
	// startDetectingCyclesAfter, so that we skip the work if we're within a
	// reasonable amount of nested pointers deep.
	ptrLevel uint
	ptrSeen  map[any]struct{}

	//anchor cache
	anchors     map[uintptr]string
	anchorNames map[string]uintptr
	//nested level
	level int

	opts EncoderOptions
}

const startDetectingCyclesAfter = 1000

var encodeStatePool sync.Pool

func newEncodeState() *encodeState {
	if v := encodeStatePool.Get(); v != nil {
		e := v.(*encodeState)
		e.Reset()
		if len(e.ptrSeen) > 0 {
			panic("ptrEncoder.encode should have emptied ptrSeen via defers")
		}
		e.ptrLevel = 0
		e.level = 0
		clear(e.anchors)
		clear(e.anchorNames)
		// e.opts  is set by marshal
		return e
	}
	return &encodeState{
		ptrSeen:     make(map[any]struct{}),
		anchors:     make(map[uintptr]string),
		anchorNames: make(map[string]uintptr),
	}
}

// yamlError is an error wrapper type for internal use only.
// Panics with errors are wrapped in yamlError so that the top-level recover
// can distinguish intentional panics from this package.
type yamlError struct{ error }

func (e *encodeState) marshal(v any, opts EncoderOptions) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if je, ok := r.(yamlError); ok {
				err = je.error
			} else {
				panic(r)
			}
		}
	}()
	e.opts = opts
	e.reflectValue(reflect.ValueOf(v), 0)
	return nil
}

// error aborts the encoding by panicking with err wrapped in yamlError.
func (e *encodeState) error(err error) {
	panic(yamlError{err})
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		//reflect.Struct,
		reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}

func (e *encodeState) reflectValue(v reflect.Value, ns nestedState) {
	valueEncoder(v)(e, v, ns)
}

// nestedState keeps a bitmap for the parent type: struct, map or slice/array
type nestedState int

const (
	//indentPrinted is used mostly for printing slices of slices, so there is no linebreak for the first element
	indentPrinted nestedState = 1 << iota
	inSlice
	inStruct
	inMap
)

type encoderFunc func(e *encodeState, v reflect.Value, ns nestedState)

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func valueEncoder(v reflect.Value) encoderFunc {
	if !v.IsValid() {
		return invalidValueEncoder
	}
	return typeEncoder(v.Type())
}

func typeEncoder(t reflect.Type) encoderFunc {
	if fi, ok := encoderCache.Load(t); ok {
		return fi.(encoderFunc)
	}

	// To deal with recursive types, populate the map with an
	// indirect func before we build it. If the type is recursive,
	// the second lookup for the type will return the indirect func.
	//
	// This indirect func is only used for recursive types,
	// and briefly during racing calls to typeEncoder.
	indirect := sync.OnceValue(func() encoderFunc {
		return newTypeEncoder(t, true)
	})
	fi, loaded := encoderCache.LoadOrStore(t, encoderFunc(func(e *encodeState, v reflect.Value, ns nestedState) {
		indirect()(e, v, ns)
	}))
	if loaded {
		return fi.(encoderFunc)
	}

	f := indirect()
	encoderCache.Store(t, f)
	return f
}

var (
	marshalerType     = reflect.TypeFor[Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// newTypeEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeEncoder(t reflect.Type, allowAddr bool) encoderFunc {
	// If we have a non-pointer value whose type implements
	// Marshaler with a value receiver, then we're better off taking
	// the address of the value - otherwise we end up with an
	// allocation as we cast the value to an interface.
	if t.Kind() != reflect.Pointer && allowAddr && reflect.PointerTo(t).Implements(marshalerType) {
		return newCondAddrEncoder(addrMarshalerEncoder, newTypeEncoder(t, false))
	}
	if t.Implements(marshalerType) {
		return marshalerEncoder
	}
	if t.Kind() != reflect.Pointer && allowAddr && reflect.PointerTo(t).Implements(textMarshalerType) {
		return newCondAddrEncoder(addrTextMarshalerEncoder, newTypeEncoder(t, false))
	}
	if t.Implements(textMarshalerType) {
		return textMarshalerEncoder
	}

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64: //, reflect.Uintptr:
		return uintEncoder
	case reflect.Float32:
		return float32Encoder
	case reflect.Float64:
		return float64Encoder
	case reflect.String:
		return stringEncoder
	case reflect.Interface:
		return interfaceEncoder
	case reflect.Struct:
		return newStructEncoder(t)
	case reflect.Map:
		return newMapEncoder(t)
	case reflect.Slice, reflect.Array:
		return newArrayEncoder(t)
	case reflect.Pointer:
		return newPtrEncoder(t)
	default:
		return unsupportedTypeEncoder
	}
}

func invalidValueEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	b := appendIndent(e, true, ns)
	e.Write(appendString(b, "null"))
}

func marshalerEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	m, ok := v.Interface().(Marshaler)
	if !ok {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	my, err := m.MarshalYAML()
	if err == nil {
		b := appendIndent(e, true, ns)
		e.Write(append(b, my...))
	}
	if err != nil {
		e.error(&MarshalerError{v.Type(), err, "MarshalYAML"})
	}
}

func addrMarshalerEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	va := v.Addr()
	if va.IsNil() {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	m := va.Interface().(Marshaler)
	my, err := m.MarshalYAML()
	if err == nil {
		b := appendIndent(e, true, ns)
		e.Write(append(b, my...))
	}
	if err != nil {
		e.error(&MarshalerError{v.Type(), err, "MarshalYAML"})
	}
}

func textMarshalerEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	m, ok := v.Interface().(encoding.TextMarshaler)
	if !ok {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	mt, err := m.MarshalText()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err, "MarshalText"})
	}
	s := string(mt)
	b := appendIndent(e, true, ns)
	if isNeedQuoted(s) {
		e.Write(strconv.AppendQuote(b, s))
		return
	}
	e.Write(append(b, mt...))
}

func addrTextMarshalerEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	va := v.Addr()
	if va.IsNil() {
		e.WriteString("null")
		return
	}
	m := va.Interface().(encoding.TextMarshaler)
	b, err := m.MarshalText()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err, "MarshalText"})
	}
	e.Write(b)
}

// a helper function to create slice indents with one '-' and spaces
func indent(level, indentNum int, isSlice bool) []byte {
	if level <= 0 {
		return nil
	}
	indent := bytes.Repeat([]byte{' '}, level*indentNum)
	if isSlice {
		indent[len(indent)-2] = '-'
	}
	return indent
}

// appendIndent appends a calculated indent to dst []byte and returns it
func appendIndentBuf(e *encodeState, dst []byte, inline bool, ns nestedState) []byte {
	if ns&indentPrinted != 0 {
		return dst
	}
	//for slices: elements always start with line break unless flow styled
	if ns&inSlice != 0 {
		if e.opts.FlowStyle {
			return dst
		}
		dst = append(dst, '\n')
		return append(dst, indent(e.level, e.opts.IndentSize, true)...)
	}
	//for structs & maps
	if ns != 0 {
		if inline {
			return append(dst, ' ')
		}
		dst = append(dst, '\n')
		return append(dst, indent(e.level, e.opts.IndentSize, ns&inSlice != 0)...)
	}
	//top-level non-inline
	if ns == 0 && !inline {
		dst = append(dst, '\n')
	}
	return dst
}

// appendIndent appends a calculated indent to e.AvailableBuffer() and returns a []byte
func appendIndent(e *encodeState, inline bool, ns nestedState) []byte {
	return appendIndentBuf(e, e.AvailableBuffer(), inline, ns)
}

func appendString(dst []byte, str string) []byte {
	return append(dst, []byte(str)...)
}

func boolEncoder(e *encodeState, v reflect.Value, ns nestedState) {

	b := appendIndent(e, true, ns)
	b = strconv.AppendBool(b, v.Bool())
	e.Write(b)
}

func intEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	if v.CanInterface() {
		if t, ok := v.Interface().(time.Duration); ok {
			encodeDuration(e, t, ns)
			return
		}
	}
	b := appendIndent(e, true, ns)
	b = strconv.AppendInt(b, v.Int(), 10)
	e.Write(b)
}

func encodeDuration(e *encodeState, d time.Duration, ns nestedState) {
	b := appendIndent(e, true, ns)
	var value []byte
	if e.opts.JSONStyle {
		value = []byte(quoteString(e, d.String()))
	} else {
		value = []byte(d.String())
	}
	b = append(b, value...)
	e.Write(b)
}

func uintEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	b := appendIndent(e, true, ns)
	b = strconv.AppendUint(b, v.Uint(), 10)
	e.Write(b)
}

type floatEncoder int // number of bits

func (bits floatEncoder) encode(e *encodeState, v reflect.Value, ns nestedState) {
	f := v.Float()
	b := appendIndent(e, true, ns)
	if f == math.Inf(0) {
		e.Write(appendString(b, ".inf"))
		return
	} else if f == math.Inf(-1) {
		e.Write(appendString(b, "-.inf"))
		return
	} else if math.IsNaN(f) {
		e.Write(appendString(b, ".nan"))
		return
	}

	b = strconv.AppendFloat(b, f, byte('g'), -1, int(bits))
	if !bytes.Contains(b, []byte{'e'}) && !bytes.Contains(b, []byte{'.'}) && !e.opts.AutoInt {
		// append x.0 suffix to keep float value context
		b = append(b, '.', '0')
	}
	e.Write(b)
}

var (
	float32Encoder = (floatEncoder(32)).encode
	float64Encoder = (floatEncoder(64)).encode
)

// isNeedQuotedOpts checks whether the value needs quote for passed string or not
func isNeedQuotedOpts(value string, opts *EncoderOptions) bool {
	if opts.JSONStyle {
		return true
	}
	if opts.LiteralStyleMultiline && strings.ContainsAny(value, "\n\r") {
		return false
	}
	if opts.FlowStyle && strings.ContainsAny(value, `]},'"`) {
		return true
	}
	if opts.FlowStyle {
		for i := 0; i < len(value); i++ {
			if value[i] != ':' {
				continue
			}
			if i+1 < len(value) && value[i+1] == '/' {
				continue
			}
			return true
		}
	}

	return isNeedQuoted(value)
}

func quoteString(e *encodeState, s string) string {
	if e.opts.SingleQuote {
		return quoteWith(s, '\'')
	}
	return strconv.Quote(s)
}

func stringEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	s := v.String()
	b := appendIndent(e, true, ns)

	if isNeedQuotedOpts(s, &e.opts) {
		e.Write(appendString(b, quoteString(e, s)))
		return
	}

	lbc := detectLineBreakCharacter(s)
	if strings.Contains(s, lbc) {
		// This block assumes that the line breaks in this inside scalar content and the Outside scalar content are the same.
		// It works mostly, but inconsistencies occur if line break characters are mixed.
		header := literalBlockHeader(s)
		level := e.level
		if ns&inSlice == 0 {
			level++
		}
		indent := strings.Repeat(" ", level*e.opts.IndentSize)
		values := []string{}
		for v := range strings.SplitSeq(s, lbc) {
			values = append(values, indent+v)
		}
		block := strings.TrimSuffix(strings.TrimSuffix(strings.Join(values, lbc), lbc+indent), indent)
		b = appendString(b, header)
		b = appendString(b, lbc)
		e.Write(appendString(b, block))
		return
	}
	e.Write(appendString(b, s))
}

func interfaceEncoder(e *encodeState, v reflect.Value, ns nestedState) {
	if v.IsNil() {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	e.reflectValue(v.Elem(), ns)
}

func unsupportedTypeEncoder(e *encodeState, v reflect.Value, _ nestedState) {
	e.error(&UnsupportedTypeError{v.Type()})
}

type mapEncoder struct {
	elemEnc encoderFunc
}

// if flow style enabled, use this encode, just to make code more clear
// the map here is nonempty, all checks passed
func (me mapEncoder) encodeFlow(e *encodeState, sv []reflectWithString, ns nestedState) {
	b := appendIndent(e, true, ns)
	e.Write(append(b, '{'))
	next := []byte{}
	for _, kv := range sv {
		b := append(e.AvailableBuffer(), next...)
		next = []byte{',', ' '}

		if e.opts.JSONStyle {
			kv.ks = quoteString(e, kv.ks)
		}
		b = appendString(b, kv.ks)

		b = append(b, ':')
		e.Write(b)
		me.elemEnc(e, kv.v, inMap)
	}
	if e.opts.FlowStyle {
		e.WriteByte('}')
	}
}

func (me mapEncoder) encode(e *encodeState, v reflect.Value, ns nestedState) {
	if ns != 0 && ns&inSlice == 0 {
		e.level++
		defer func() { e.level-- }()
	}
	if v.IsNil() {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	if v.Len() == 0 {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "{}"))
		return
	}
	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.UnsafePointer()
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
		defer func() { e.ptrLevel-- }()
	}

	// Extract and sort the keys.
	var (
		sv  = make([]reflectWithString, v.Len())
		mi  = v.MapRange()
		err error
	)
	for i := 0; mi.Next(); i++ {
		if sv[i].ks, err = resolveKeyName(mi.Key()); err != nil {
			e.error(fmt.Errorf("yaml: encoding error for type %q: %q", v.Type().String(), err.Error()))
		}
		sv[i].v = mi.Value()
	}
	slices.SortFunc(sv, func(i, j reflectWithString) int {
		return strings.Compare(i.ks, j.ks)
	})

	if e.opts.FlowStyle {
		me.encodeFlow(e, sv, ns)
		return
	}

	for _, kv := range sv {
		b := appendIndent(e, false, ns)

		if e.opts.JSONStyle {
			kv.ks = quoteString(e, kv.ks)
		}
		b = appendString(b, kv.ks)

		b = append(b, ':')
		e.Write(b)
		me.elemEnc(e, kv.v, inMap)
		//after first field printed:
		//remove inSlice + indentPrinted bit, so it does not print '-'
		//set ns = inStruct just to preserve the indent size
		ns = inStruct
	}
}

func newMapEncoder(t reflect.Type) encoderFunc {
	switch t.Key().Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Bool:
	default:
		if !t.Key().Implements(textMarshalerType) {
			return unsupportedTypeEncoder
		}
	}
	me := mapEncoder{typeEncoder(t.Elem())}
	return me.encode
}

// mergeAlias merges an alias if found in anchor's map
func mergeAlias(e *encodeState, ptr uintptr) bool {
	anchor, ok := getAnchor(e, ptr)
	if ok {
		e.WriteString("<<: *" + anchor)
	}
	return ok
}

// encodeAlias encodes alias if found in anchor's map, returns true is successful
func encodeAlias(e *encodeState, ptr uintptr, alias string) bool {
	anchor, ok := getAnchor(e, ptr)
	if ok {
		//alias validity check
		if alias != "" && anchor != alias {
			e.error(fmt.Errorf("expected alias name is %q but got %q", alias, anchor))
		}
		e.WriteString(" *" + anchor)
	}
	return ok
}

func encodeAnchor(e *encodeState, ptr uintptr, anchor string) {
	if anchor != "" {
		anch := storeAnchor(e, ptr, anchor)
		if anch == "" {
			e.error(fmt.Errorf("unexpected empty anchor name"))
		}
		b := append(e.AvailableBuffer(), ' ', '&')
		e.Write(appendString(b, anch))
	}
}

type structEncoder struct {
	fields structFields
}

type structFields struct {
	list         []field
	byExactName  map[string]*field
	byFoldedName map[string]*field
}

type exportedField struct {
	f  *field
	fv reflect.Value
}

func (se structEncoder) exportedFields(e *encodeState, v reflect.Value) []exportedField {
	eFields := make([]exportedField, 0, len(se.fields.list))

FieldLoop:
	for i := range se.fields.list {
		f := &se.fields.list[i]
		// Find the nested struct field by following f.index.
		fv := v
		for _, i := range f.index {
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					continue FieldLoop
				}
				fv = fv.Elem()
			}
			fv = fv.Field(i)
		}

		if ((f.omitEmpty || e.opts.OmitEmpty) && isEmptyValue(fv)) ||
			((f.omitZero || e.opts.OmitZero) && (f.isZero == nil && fv.IsZero() || (f.isZero != nil && f.isZero(fv)))) {
			continue
		}
		eFields = append(eFields, exportedField{f, fv})
	}

	return eFields
}

func (se structEncoder) encode(e *encodeState, v reflect.Value, ns nestedState) {
	if ns != 0 && ns&inSlice == 0 {
		e.level++
		defer func() { e.level-- }()
	}
	next := []byte{} //field separator
	if e.opts.FlowStyle {
		b := appendIndent(e, true, ns)
		e.Write(append(b, '{'))
	}

	//each time isAnonymousRef field is processed and anchor pointer found
	//add its type to anchorsFound. So all fields with anonymousParentTyp
	//in this map should be skipped without encoding, because there is an *anchor
	//for the whole struct
	anchorsFound := make(map[reflect.Type]struct{})
	//check this when printing a slice of structs, or empty/with private fields only/ structs
	empty := true
	fields := se.exportedFields(e, v)
	for i := range fields {
		f := fields[i].f
		fv := fields[i].fv
		if !empty && ns&indentPrinted != 0 {
			ns = ns &^ indentPrinted
		}

		// if its a reference to embedded struct & no anchors found, ignore the ref
		// this is an artificial field with only purpose to track anchors
		if f.isAnonymousRef {
			if _, ok := getAnchor(e, fv.Pointer()); !ok {
				continue
			}
		}

		//se.fields.list is a plain slice of all struct fields mixed with embedded ones.
		//f.anonymousParentTyp != nil if a parent struct has an embedded pointer struct
		//if anchorsFound[f.anonymousParentTyp] found it means an alias has already been printed
		//and I should skip these nested fields
		if f.anonymousParentTyp != nil {
			if _, ok := anchorsFound[f.anonymousParentTyp]; ok {
				continue
			}
		}

		b := append(e.AvailableBuffer(), next...)
		if e.opts.FlowStyle {
			next = []byte{',', ' '}
		}

		if !e.opts.FlowStyle {
			if !empty && ns&inSlice != 0 {
				//for second field and on remove inSlice bit so it does not print '-'
				//add inStruct bit just to preserve the indent size
				ns = ns&^inSlice | inStruct
			}
			b = appendIndentBuf(e, b, false, ns)
		}
		empty = false
		//anonymous self embed with pointer, like struct Person{*Person...}, f->*Person field
		if f.isAnonymousRef && fv.Elem().Type() == v.Type() {
			if found := mergeAlias(e, fv.Pointer()); found {
				continue
			}
		}

		if e.opts.JSONStyle {
			//quote struct fields
			b = appendString(b, quoteString(e, f.name))
		} else {
			b = append(b, f.nameBytes...)
		}
		e.Write(append(b, ':'))
		//types of possible anchors/aliases
		isReference := (fv.Kind() == reflect.Pointer || fv.Kind() == reflect.Map || fv.Kind() == reflect.Slice)
		if isReference {
			if found := encodeAlias(e, fv.Pointer(), f.alias); found {
				if fv.Kind() == reflect.Pointer {
					anchorsFound[fv.Elem().Type()] = struct{}{}
				}
				continue
			}

			encodeAnchor(e, fv.Pointer(), f.anchor)
		}
		oldFlow := e.opts.FlowStyle
		//merge global flow with field flow
		e.opts.FlowStyle = e.opts.FlowStyle || f.flow
		f.encoder(e, fv, inStruct)
		//and revert back ;)
		e.opts.FlowStyle = oldFlow
	}
	//if no fields printed
	if empty && !e.opts.FlowStyle {
		b := appendIndent(e, true, ns)
		e.Write(append(b, '{', '}'))
	} else if e.opts.FlowStyle {
		e.WriteByte('}')
	}
}

func newStructEncoder(t reflect.Type) encoderFunc {
	se := structEncoder{fields: cachedTypeFields(t)}
	return se.encode
}

type arrayEncoder struct {
	elemEnc encoderFunc
}

// flow style encoder, made it separate for readability
func (ae arrayEncoder) encodeFlow(e *encodeState, v reflect.Value, ns nestedState) {
	n := v.Len()
	b := appendIndent(e, true, ns)
	e.Write(append(b, '['))
	for i := range n {
		if i > 0 {
			e.Write([]byte{',', ' '})
		}
		ae.elemEnc(e, v.Index(i), inSlice)
	}
	e.WriteByte(']')
}

func (ae arrayEncoder) encode(e *encodeState, v reflect.Value, ns nestedState) {
	if ns != 0 || e.level == 0 && !e.opts.FlowStyle {
		e.level++
		defer func() { e.level-- }()
		//just add one more indent? or check if ns&inMap != 0 || ns&inStruct != 0
		if e.opts.IndentSequence {
			e.level++
			defer func() { e.level-- }()
		}
	}
	n := v.Len()
	if n == 0 {
		b := appendIndent(e, true, ns)
		e.Write(append(b, '[', ']'))
		return
	}

	if e.opts.FlowStyle {
		ae.encodeFlow(e, v, ns)
		return
	}

	/*
		Here, an array differs from other types in that it prints the required indentation
		itself, rather than delegating it to the element. If the element is a structure
		or a map, this applies only to it's first field;
		the others print the indentation themselves. This is controlled by the bit indentPrinted
		Alittle bit confusing logic, but this is the only way to encode slices of slices I have found.
	*/
	for i := range n {
		nsChild := inSlice | indentPrinted
		if ns&indentPrinted == 0 {
			e.Write(appendIndent(e, false, inSlice))
		} else {
			//if slice of slices and this is the second slice, just print "- "
			e.Write(indent(1, e.opts.IndentSize, true))
		}
		ae.elemEnc(e, v.Index(i), nsChild)
		ns = ns &^ indentPrinted //reset the indentPrinted bit, coming from parent ns
	}
}

func newArrayEncoder(t reflect.Type) encoderFunc {
	enc := arrayEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type ptrEncoder struct {
	elemEnc encoderFunc
}

func (pe ptrEncoder) encode(e *encodeState, v reflect.Value, ns nestedState) {
	if v.IsNil() {
		b := appendIndent(e, true, ns)
		e.Write(appendString(b, "null"))
		return
	}
	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.Interface()
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}
	pe.elemEnc(e, v.Elem(), ns)
	e.ptrLevel--
}

func newPtrEncoder(t reflect.Type) encoderFunc {
	enc := ptrEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type condAddrEncoder struct {
	canAddrEnc, elseEnc encoderFunc
}

func (ce condAddrEncoder) encode(e *encodeState, v reflect.Value, ns nestedState) {
	if v.CanAddr() {
		ce.canAddrEnc(e, v, ns)
	} else {
		ce.elseEnc(e, v, ns)
	}
}

// newCondAddrEncoder returns an encoder that checks whether its value
// CanAddr and delegates to canAddrEnc if so, else to elseEnc.
func newCondAddrEncoder(canAddrEnc, elseEnc encoderFunc) encoderFunc {
	enc := condAddrEncoder{canAddrEnc: canAddrEnc, elseEnc: elseEnc}
	return enc.encode
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

func typeByIndex(t reflect.Type, index []int) reflect.Type {
	for _, i := range index {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		t = t.Field(i).Type
	}
	return t
}

type reflectWithString struct {
	v  reflect.Value
	ks string
}

func resolveKeyName(k reflect.Value) (string, error) {
	if tm, ok := k.Interface().(encoding.TextMarshaler); ok {
		if k.Kind() == reflect.Pointer && k.IsNil() {
			return "", nil
		}
		buf, err := tm.MarshalText()
		return string(buf), err
	}

	switch k.Kind() {
	case reflect.String:
		return k.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(k.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(k.Uint(), 10), nil
	case reflect.Float32:
		return strconv.FormatFloat(k.Float(), 'g', -1, 32), nil
	case reflect.Float64:
		return strconv.FormatFloat(k.Float(), 'g', -1, 64), nil
	case reflect.Bool:
		return strconv.FormatBool(k.Bool()), nil
	}
	panic("unexpected map key type")
}

// A field represents a single field found in a struct.
type field struct {
	name      string
	nameBytes []byte // []byte(name)

	// namePrint string // `"` + name + `":`

	tag       bool
	index     []int
	typ       reflect.Type
	omitEmpty bool
	omitZero  bool
	isZero    func(reflect.Value) bool
	quoted    bool
	inline    bool
	flow      bool

	//yaml anchors & aliases
	anchor string
	alias  string
	//if a field is a pointer to self struct (embedded or not)
	//if true, structEncoder should check pointer value for containment in encoder's anchorRefToName map
	isAnonymousRef bool
	//a field inside embedded struct pointer.
	//this field should not be encoded to yaml if and only if an anchor for the parent embedded struct exists in cache
	isAnonymousRefField bool
	//anonymousParentTyp contains parent struct type if isAnonymousRefField is true
	anonymousParentTyp reflect.Type

	encoder encoderFunc
}

type isZeroer interface {
	IsZero() bool
}

var isZeroerType = reflect.TypeFor[isZeroer]()

// typeFields returns a list of fields that YAML should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
//
// typeFields should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/bytedance/sonic
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname typeFields
func typeFields(t reflect.Type) structFields {
	// Anonymous fields to explore at the current level and the next.
	current := []field{}
	next := []field{{typ: t}}

	// Count of queued names for current level and the next.
	var count, nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			// Scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Pointer {
						t = t.Elem()
					}
					if !sf.IsExported() && t.Kind() != reflect.Struct {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}
					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if !sf.IsExported() {
					// Ignore unexported non-embedded fields.
					continue
				}
				tag := sf.Tag.Get("yaml")
				if tag == "-" {
					continue
				}
				name, opts := parseTag(tag)
				if !isValidTag(name) {
					name = ""
				}
				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i

				isAnonymousStructRef := false
				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					// Follow pointer.
					ft = ft.Elem()

					//embedded reference, potentially anchorable, ex: type Person struct {*Person ...}
					if sf.Anonymous && ft.Kind() == reflect.Struct {
						isAnonymousStructRef = true
					}
				}

				// Record found field and index sequence.
				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct || isAnonymousStructRef {
					tagged := name != ""
					if name == "" {
						//default lowercase struct fields, go-yaml compatibility
						//name = sf.Name
						name = strings.ToLower(sf.Name)
					}
					//yaml anchor & alias
					hasAnchor := opts.ContainsWithPrefix("anchor")
					anchor := ""
					if hasAnchor {
						anchor = opts.NamedValue("anchor")
						if anchor == "" {
							anchor = name
						}
					}
					hasAlias := opts.ContainsWithPrefix("alias")
					alias := ""
					if hasAlias {
						alias = opts.NamedValue("alias")
					}

					field := field{
						name:                name,
						tag:                 tagged,
						index:               index,
						typ:                 ft,
						omitEmpty:           opts.Contains("omitempty"),
						omitZero:            opts.Contains("omitzero"),
						inline:              opts.Contains("inline"),
						flow:                opts.Contains("flow"),
						anchor:              anchor,
						alias:               alias,
						isAnonymousRef:      isAnonymousStructRef,
						isAnonymousRefField: f.isAnonymousRefField,
						//quoted:    quoted,
					}
					field.nameBytes = []byte(field.name)
					// field.namePrint = field.name + `: `

					if field.isAnonymousRefField {
						field.anonymousParentTyp = f.typ
					}

					if field.omitZero {
						t := sf.Type
						// Provide a function that uses a type's IsZero method.
						switch {
						case t.Kind() == reflect.Interface && t.Implements(isZeroerType):
							field.isZero = func(v reflect.Value) bool {
								// Avoid panics calling IsZero on a nil interface or
								// non-nil interface with nil pointer.
								return v.IsNil() ||
									(v.Elem().Kind() == reflect.Pointer && v.Elem().IsNil()) ||
									v.Interface().(isZeroer).IsZero()
							}
						case t.Kind() == reflect.Pointer && t.Implements(isZeroerType):
							field.isZero = func(v reflect.Value) bool {
								// Avoid panics calling IsZero on nil pointer.
								return v.IsNil() || v.Interface().(isZeroer).IsZero()
							}
						case t.Implements(isZeroerType):
							field.isZero = func(v reflect.Value) bool {
								return v.Interface().(isZeroer).IsZero()
							}
						case reflect.PointerTo(t).Implements(isZeroerType):
							field.isZero = func(v reflect.Value) bool {
								if !v.CanAddr() {
									// Temporarily box v so we can take the address.
									v2 := reflect.New(v.Type()).Elem()
									v2.Set(v)
									v = v2
								}
								return v.Addr().Interface().(isZeroer).IsZero()
							}
						}
					}

					fields = append(fields, field)
					if count[f.typ] > 1 {
						// If there were multiple instances, add a second,
						// so that the annihilation code will see a duplicate.
						// It only cares about the distinction between 1 and 2,
						// so don't bother generating any more copies.
						fields = append(fields, fields[len(fields)-1])
					}
					//dont skip anonymous struct exploration (below)
					// continue
					if !isAnonymousStructRef {
						continue
					}
				}

				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, field{name: ft.Name(), index: index, typ: ft, isAnonymousRefField: isAnonymousStructRef})
				}
			}
		}
	}

	slices.SortFunc(fields, func(a, b field) int {
		// sort field by name, breaking ties with depth, then
		// breaking ties with "name came from yaml tag", then
		// breaking ties with index sequence.
		if c := strings.Compare(a.name, b.name); c != 0 {
			return c
		}
		if c := cmp.Compare(len(a.index), len(b.index)); c != 0 {
			return c
		}
		if a.tag != b.tag {
			if a.tag {
				return -1
			}
			return +1
		}
		return slices.Compare(a.index, b.index)
	})

	// Delete all fields that are hidden by the Go rules for embedded fields,
	// except that fields with YAML tags are promoted.

	// The fields are sorted in primary order of name, secondary order
	// of field index length. Loop over names; for each name, delete
	// hidden fields by choosing the one dominant field that survives.
	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		// One iteration per name.
		// Find the sequence of fields with the name of this first field.
		fi := fields[i]
		name := fi.name
		for advance = 1; i+advance < len(fields); advance++ {
			fj := fields[i+advance]
			if fj.name != name {
				break
			}
		}
		if advance == 1 { // Only one field with this name
			out = append(out, fi)
			continue
		}
		dominant, ok := dominantField(fields[i : i+advance])
		if ok {
			out = append(out, dominant)
		}
	}

	fields = out
	slices.SortFunc(fields, func(i, j field) int {
		return slices.Compare(i.index, j.index)
	})

	for i := range fields {
		f := &fields[i]
		f.encoder = typeEncoder(typeByIndex(t, f.index))
	}
	exactNameIndex := make(map[string]*field, len(fields))
	foldedNameIndex := make(map[string]*field, len(fields))
	for i, field := range fields {
		exactNameIndex[field.name] = &fields[i]
		// For historical reasons, first folded match takes precedence.
		if _, ok := foldedNameIndex[string(foldName(field.nameBytes))]; !ok {
			foldedNameIndex[string(foldName(field.nameBytes))] = &fields[i]
		}
	}
	return structFields{fields, exactNameIndex, foldedNameIndex}
}

// dominantField looks through the fields, all of which are known to
// have the same name, to find the single field that dominates the
// others using Go's embedding rules, modified by the presence of
// YAML tags. If there are multiple top-level fields, the boolean
// will be false: This condition is an error in Go and we skip all
// the fields.
func dominantField(fields []field) (field, bool) {
	// The fields are sorted in increasing index-length order, then by presence of tag.
	// That means that the first field is the dominant one. We need only check
	// for error cases: two fields at top level, either both tagged or neither tagged.
	if len(fields) > 1 && len(fields[0].index) == len(fields[1].index) && fields[0].tag == fields[1].tag {
		return field{}, false
	}
	return fields[0], true
}

var fieldCache sync.Map // map[reflect.Type]structFields

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type) structFields {
	if f, ok := fieldCache.Load(t); ok {
		return f.(structFields)
	}
	f, _ := fieldCache.LoadOrStore(t, typeFields(t))
	return f.(structFields)
}

// storeAnchor stores anchor ptr & name in map cache. If cache already
// has an anchor with the same name but different ptr (name collision)
// then it makes a unique name by appending number suffix, stores
// and returns it
func storeAnchor(e *encodeState, ptr uintptr, name string) string {
	if ptr == 0 || name == "" {
		return ""
	}
	if cachedPtr, ok := e.anchorNames[name]; ok {
		if cachedPtr == ptr {
			return name
		}
		name = uniqueAnchorName(e, name)
	}
	e.anchors[ptr] = name
	e.anchorNames[name] = ptr
	return name
}

func uniqueAnchorName(e *encodeState, base string) string {
	//anchor name already exists in cache no need to check again
	for i := 1; i < 100; i++ {
		name := base + strconv.Itoa(i)
		if _, exists := e.anchorNames[name]; exists {
			continue
		}
		return name
	}
	return ""
}

func getAnchor(e *encodeState, ref uintptr) (string, bool) {
	name, exists := e.anchors[ref]
	return name, exists
}
