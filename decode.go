// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Represents YAML data structure using native Go types: booleans, floats,
// strings, arrays, and maps.

//go:build !goexperiment.yamlv2

package gyaml

import (
	"bytes"
	"encoding"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	_ "unsafe" // for linkname
)

// Unmarshal decodes the first document found within the in byte slice
// and assigns decoded values into the out value.
//
// Struct fields are only unmarshalled if they are exported,
// and are unmarshalled using the field name
// lowercased as the default key. Custom name may be defined via the
// "yaml" field tag: the content preceding the first comma
// is used as the key, and the following comma-separated options are
// used to tweak the marshaling process (see Marshal).
// Conflicting names result in a runtime error.
//
// For example:
//
//	type T struct {
//	    F int `yaml:"a,omitempty"`
//	    B int
//	}
//	var t T
//	yaml.Unmarshal([]byte("a: 1\nb: 2"), &t)
//
// See the documentation of Marshal for the format of tags and a list of
// supported tag options.
func Unmarshal(data []byte, v any) error {
	return UnmarshalWithOptions(data, v, DefaultDecoderOptions())
}

// UnmarshalWithOptions decodes with DecodeOptions the first document found within the in byte slice
// and assigns decoded values into the out value.
func UnmarshalWithOptions(data []byte, v any, opts DecoderOptions) error {
	// Check for well-formedness.
	// Avoids filling out half a data structure
	// before discovering a YAML syntax error.
	var d decodeState
	err := checkValid(data, &d.scan)
	if err != nil {
		return err
	}

	d.init(data)
	return d.unmarshal(v)
}

// Unmarshaler is the interface implemented by types
// that can unmarshal a YAML description of themselves.
// The input can be assumed to be a valid encoding of
// a YAML value. UnmarshalYAML must copy the YAML data
// if it wishes to retain the data after returning.
type Unmarshaler interface {
	UnmarshalYAML([]byte) error
}

// An UnmarshalTypeError describes a YAML value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value  string       // description of YAML value - "bool", "array", "number -5"
	Type   reflect.Type // type of Go value it could not be assigned to
	Offset int64        // error occurred after reading Offset bytes
	Struct string       // name of the struct type containing the field
	Field  string       // the full path from root node to the field, include embedded struct
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return "yaml: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "yaml: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

// An UnmarshalFieldError describes a YAML object key that
// led to an unexported (and therefore unwritable) struct field.
//
// Deprecated: No longer used; kept for compatibility.
type UnmarshalFieldError struct {
	Key   string
	Type  reflect.Type
	Field reflect.StructField
}

func (e *UnmarshalFieldError) Error() string {
	return "yaml: cannot unmarshal object key " + strconv.Quote(e.Key) + " into unexported field " + e.Field.Name + " of type " + e.Type.String()
}

// An InvalidUnmarshalError describes an invalid argument passed to [Unmarshal].
// (The argument to [Unmarshal] must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "yaml: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "yaml: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "yaml: Unmarshal(nil " + e.Type.String() + ")"
}

func (d *decodeState) unmarshal(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	d.scan.reset()
	d.scanWhile(scanSkipSpace)
	// We decode rv not rv.Elem because the Unmarshaler interface
	// test must be applied at the top level of the value.
	err := d.value(rv)
	if err != nil {
		return d.addErrorContext(err)
	}
	return d.savedError
}

// A Number represents a YAML number literal.
type Number string

// String returns the literal text of the number.
func (n Number) String() string { return string(n) }

// Float64 returns the number as a float64.
func (n Number) Float64() (float64, error) {
	return strconv.ParseFloat(string(n), 64)
}

// Int64 returns the number as an int64.
func (n Number) Int64() (int64, error) {
	return strconv.ParseInt(string(n), 10, 64)
}

// An errorContext provides context for type errors during decoding.
type errorContext struct {
	Struct     reflect.Type
	FieldStack []string
}

// decodeState represents the state while decoding a YAML value.
type decodeState struct {
	data                  []byte
	off                   int // next read offset in data
	opcode                int // last read result
	scan                  scanner
	errorContext          *errorContext
	savedError            error
	useNumber             bool
	disallowUnknownFields bool
}

// readIndex returns the position of the last byte read.
func (d *decodeState) readIndex() int {
	return d.off - 1
}

// phasePanicMsg is used as a panic message when we end up with something that
// shouldn't happen. It can indicate a bug in the YAML decoder, or that
// something is editing the data slice while the decoder executes.
const phasePanicMsg = "YAML decoder out of sync - data changing underfoot?"

func (d *decodeState) init(data []byte) *decodeState {
	d.data = data
	d.off = 0
	d.savedError = nil
	if d.errorContext != nil {
		d.errorContext.Struct = nil
		// Reuse the allocated space for the FieldStack slice.
		d.errorContext.FieldStack = d.errorContext.FieldStack[:0]
	}
	return d
}

// saveError saves the first err it is called with,
// for reporting at the end of the unmarshal.
func (d *decodeState) saveError(err error) {
	if d.savedError == nil {
		d.savedError = d.addErrorContext(err)
	}
}

// addErrorContext returns a new error enhanced with information from d.errorContext
func (d *decodeState) addErrorContext(err error) error {
	if d.errorContext != nil && (d.errorContext.Struct != nil || len(d.errorContext.FieldStack) > 0) {
		switch err := err.(type) {
		case *UnmarshalTypeError:
			err.Struct = d.errorContext.Struct.Name()
			fieldStack := d.errorContext.FieldStack
			if err.Field != "" {
				fieldStack = append(fieldStack, err.Field)
			}
			err.Field = strings.Join(fieldStack, ".")
		}
	}
	return err
}

// skip scans to the end of what was started.
func (d *decodeState) skip() {
	s, data, i := &d.scan, d.data, d.off
	depth := len(s.states)
	// for {
	for i < len(data) {
		op := s.step(s, data[i])
		i++
		if len(s.states) < depth {
			d.off = i
			d.opcode = op
			return
		}
	}
}

// scanNext processes the byte at d.data[d.off].
func (d *decodeState) scanNext() {
	if d.off < len(d.data) {
		d.opcode = d.scan.step(&d.scan, d.data[d.off])
		d.off++
	} else {
		d.opcode = d.scan.eof()
		d.off = len(d.data) + 1 // mark processed EOF with len+1
	}
}

// scanWhile processes bytes in d.data[d.off:] until it
// receives a scan code not equal to op.
func (d *decodeState) scanWhile(op int) {
	s, data, i := &d.scan, d.data, d.off
	for i < len(data) {
		newOp := s.step(s, data[i])
		i++
		if newOp != op {
			d.opcode = newOp
			d.off = i
			return
		}
	}

	d.off = len(data) + 1 // mark processed EOF with len+1
	d.opcode = d.scan.eof()
}

// rescanLiteral is similar to scanWhile(scanContinue), but it specialises the
// common case where we're decoding a literal. The decoder scans the input
// twice, once for syntax errors and to check the length of the value, and the
// second to perform the decoding.
//
// Only in the second step do we use decodeState to tokenize literals, so we
// know there aren't any syntax errors. We can take advantage of that knowledge,
// and scan a literal's bytes much more quickly.
func (d *decodeState) rescanLiteral() {
	//TODO: can I use scanner here? Why duplicate the logic?
	data, i := d.data, d.off
Switch:
	switch data[i-1] {
	case '"': // string
		for ; i < len(data); i++ {
			switch data[i] {
			case '\\':
				i++ // escaped char
			case '"':
				i++ // tokenize the closing quote too
				break Switch
			}
		}
	case '\'': // single-quoted string
		for ; i < len(data); i++ {
			switch data[i] {
			case '\'':
				i++ // tokenize the closing quote too
				break Switch
			}
		}
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '+', '.': //number or time or .inf/.nan
		for ; i < len(data); i++ {
			switch data[i] {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
				'.', 'e', 'E', '+', '-', '_', 'b', 'o', 'x', 'a', 'c', 'd', 'f', 'A', 'B', 'C', 'D', 'F', //some non-decimal checks added
				'T', 't', 'Z', ' ', 's', 'h', //time.Time & Duration. A little bit too much for a number yes?
				'i', 'I', 'n', 'N': //.inf, .nan
			case ':':
				if i+1 == len(data) || isSpace(data[i+1]) || isLineBreak(data[i+1]) {
					break Switch
				}
			default:
				break Switch
			}
		}
	default:
		if unicode.IsLetter(rune(data[i-1])) {
			for ; i < len(data); i++ {
				//TODO: add end unquoted scalars checks from scanner
				//TODO: optimize
				if data[i] == '\n' ||
					(data[i] == ',' || data[i] == ']') && d.scan.inArray() || //flow array
					(data[i] == ':' && (i+1 == len(data) || isSpace(data[i+1]) || isLineBreak(data[i+1]) || isUnqTermin(data[i+1]))) {
					break Switch
				}
			}
		}
	}
	if i < len(data) {
		d.opcode = stateEndValue(&d.scan, data[i])
	} else {
		d.opcode = scanEnd
	}
	d.off = i + 1
}

// value consumes a YAML value from d.data[d.off-1:], decoding into v, and
// reads the following byte ahead. If v is invalid, the value is discarded.
// The first byte of the value has been read already.
func (d *decodeState) value(v reflect.Value) error {
	//TODO: add checks for struct xD (and custom unmarshaler)
	if v.IsValid() && isMapOrStruct(v) {
		//TODO: fix if's
		if v.IsValid() {
			if err := d.object(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		d.scanNext()
		return nil
	}
	//can't use json logic here because no starting object/array delimiters in yaml
	switch d.opcode {
	default:
		panic(phasePanicMsg)

	case scanEnd:
		if v.IsValid() && v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
		return nil

	case scanBeginArray:
		if v.IsValid() {
			if err := d.array(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		d.scanNext()

	case scanBeginObject:
		if v.IsValid() {
			if err := d.object(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		d.scanNext()

	case scanBeginLiteral:
		// All bytes inside literal return scanContinue op code.
		start := d.readIndex()
		d.rescanLiteral()

		if v.IsValid() {
			if err := d.storeLiteral(d.data[start:d.readIndex()], v, false); err != nil {
				return err
			}
		}
	}
	// if v.IsValid() {
	// 	if err := d.object(v); err != nil {
	// 		return err
	// 	}
	// } else {
	// 	d.skip()
	// }
	// d.scanNext()
	return nil
}

type unquotedValue struct{}

// valueQuoted is like value but decodes a
// quoted string literal or literal null into an interface value.
// If it finds anything other than a quoted string literal or null,
// valueQuoted returns unquotedValue{}.
func (d *decodeState) valueQuoted() any {
	switch d.opcode {
	default:
		panic(phasePanicMsg)

	case scanBeginArray, scanBeginObject:
		d.skip()
		d.scanNext()

	case scanBeginLiteral:
		v := d.literalInterface()
		switch v.(type) {
		case nil, string:
			return v
		}
	}
	return unquotedValue{}
}

// indirect walks down v allocating pointers as needed,
// until it gets to a non-pointer.
// If it encounters an Unmarshaler, indirect stops and returns that.
// If decodingNull is true, indirect stops at the first settable pointer so it
// can be set to nil.
func indirect(v reflect.Value, decodingNull bool) (Unmarshaler, encoding.TextUnmarshaler, reflect.Value) {
	// Issue #24153 indicates that it is generally not a guaranteed property
	// that you may round-trip a reflect.Value by calling Value.Addr().Elem()
	// and expect the value to still be settable for values derived from
	// unexported embedded struct fields.
	//
	// The logic below effectively does this when it first addresses the value
	// (to satisfy possible pointer methods) and continues to dereference
	// subsequent pointers as necessary.
	//
	// After the first round-trip, we set v back to the original value to
	// preserve the original RW flags contained in reflect.Value.
	v0 := v
	haveAddr := false

	// If v is a named type and is addressable,
	// start with its address, so that if the type has pointer methods,
	// we find them.
	if v.Kind() != reflect.Pointer && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}
	for {
		// Load value from interface, but only if the result will be
		// usefully addressable.
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Pointer && !e.IsNil() && (!decodingNull || e.Elem().Kind() == reflect.Pointer) {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Pointer {
			break
		}

		if decodingNull && v.CanSet() {
			break
		}

		// Prevent infinite loop if v is an interface pointing to its own address:
		//     var v any
		//     v = &v
		if v.Elem().Kind() == reflect.Interface && v.Elem().Elem().Equal(v) {
			v = v.Elem()
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(Unmarshaler); ok {
				return u, nil, reflect.Value{}
			}
			if !decodingNull {
				if u, ok := v.Interface().(encoding.TextUnmarshaler); ok {
					return nil, u, reflect.Value{}
				}
			}
		}

		if haveAddr {
			v = v0 // restore original value after round-trip Value.Addr().Elem()
			haveAddr = false
		} else {
			v = v.Elem()
		}
	}
	return nil, nil, v
}

// array consumes an array from d.data[d.off-1:], decoding into v.
// The first byte of the array ('[') has been read already.
func (d *decodeState) array(v reflect.Value) error {
	// Check for unmarshaler.
	u, ut, pv := indirect(v, false)
	if u != nil {
		start := d.readIndex()
		d.skip()
		return u.UnmarshalYAML(d.data[start:d.off])
	}
	if ut != nil {
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	}
	v = pv

	// Check type of target.
	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() == 0 {
			// Decoding into nil interface? Switch to non-reflect code.
			ai := d.arrayInterface()
			v.Set(reflect.ValueOf(ai))
			return nil
		}
		// Otherwise it's invalid.
		fallthrough
	default:
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	case reflect.Array, reflect.Slice:
		break
	}

	i := 0
	for {
		// Look ahead for ] - can only happen on first iteration.
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndArray {
			break
		}

		// Expand slice length, growing the slice if necessary.
		if v.Kind() == reflect.Slice {
			if i >= v.Cap() {
				v.Grow(1)
			}
			if i >= v.Len() {
				v.SetLen(i + 1)
			}
		}

		if i < v.Len() {
			// Decode into element.
			if err := d.value(v.Index(i)); err != nil {
				return err
			}
		} else {
			// Ran out of fixed array: skip.
			if err := d.value(reflect.Value{}); err != nil {
				return err
			}
		}
		i++

		// Next token must be , or ].
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray {
			break
		}
		if d.opcode != scanArrayValue {
			panic(phasePanicMsg)
		}
	}

	if i < v.Len() {
		if v.Kind() == reflect.Array {
			for ; i < v.Len(); i++ {
				v.Index(i).SetZero() // zero remainder of array
			}
		} else {
			v.SetLen(i) // truncate the slice
		}
	}
	if i == 0 && v.Kind() == reflect.Slice {
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}
	return nil
}

func isMapOrStruct(v reflect.Value) bool {
	_, _, pv := indirect(v, false)
	return pv.Kind() == reflect.Map //|| v.Kind() == reflect.Struct || pv.IsValid() && pv.Kind() == reflect.Struct
}

var nullLiteral = []byte("null")
var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// object consumes an object from d.data[d.off-1:], decoding into v.
// The first byte ('{') of the object has been read already.
func (d *decodeState) object(v reflect.Value) error {
	// Check for unmarshaler.
	u, ut, pv := indirect(v, false)
	if u != nil {
		start := d.readIndex()
		d.skip()
		return u.UnmarshalYAML(d.data[start:d.off])
	}
	if ut != nil {
		d.saveError(&UnmarshalTypeError{Value: "object", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	}
	v = pv
	t := v.Type()

	// Decoding into nil interface? Switch to non-reflect code.
	if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
		oi := d.objectInterface()
		v.Set(reflect.ValueOf(oi))
		return nil
	}

	var fields structFields

	// Check type of target:
	//   struct or
	//   map[T1]T2 where T1 is string, an integer type,
	//             or an encoding.TextUnmarshaler
	switch v.Kind() {
	case reflect.Map:
		// Map key must either have string kind, have an integer kind,
		// or be an encoding.TextUnmarshaler.
		switch t.Key().Kind() {
		case reflect.String,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
			reflect.Float32, reflect.Float64, reflect.Bool:
		default:
			if !reflect.PointerTo(t.Key()).Implements(textUnmarshalerType) {
				d.saveError(&UnmarshalTypeError{Value: "object", Type: t, Offset: int64(d.off)})
				d.skip()
				return nil
			}
		}
		if v.IsNil() {
			v.Set(reflect.MakeMap(t))
		}
	case reflect.Struct:
		fields = cachedTypeFields(t)
		// ok
	default:
		d.saveError(&UnmarshalTypeError{Value: "object", Type: t, Offset: int64(d.off)})
		d.skip()
		return nil
	}

	var mapElem reflect.Value
	var origErrorContext errorContext
	if d.errorContext != nil {
		origErrorContext = *d.errorContext
	}

	for {
		// Read opening " of string key or closing }.
		// d.scanWhile(scanSkipSpace)
		// if d.opcode == scanEndObject {
		// 	// closing } - can only happen on first iteration.
		// 	break
		// }
		// if d.opcode != scanBeginLiteral {
		// 	panic(phasePanicMsg)
		// }

		// Read key.
		start := d.readIndex()
		//TODO: dont just scan literal scan until ": " or ":\n" met
		d.rescanLiteral()
		item := d.data[start:d.readIndex()]
		var key []byte
		if len(item) > 0 && (item[0] == '"' || item[0] == '\'') {
			var ok bool
			key, ok = unquoteBytes(item)
			if !ok {
				panic(phasePanicMsg)
			}
		} else {
			key = item
		}

		// Figure out field corresponding to key.
		var subv reflect.Value
		destring := false // whether the value is wrapped in a string to be decoded first

		if v.Kind() == reflect.Map {
			elemType := t.Elem()
			if !mapElem.IsValid() {
				mapElem = reflect.New(elemType).Elem()
			} else {
				mapElem.SetZero()
			}
			subv = mapElem
		} else {
			f := fields.byExactName[string(key)]
			if f == nil {
				f = fields.byFoldedName[string(foldName(key))]
			}
			if f != nil {
				subv = v
				destring = f.quoted
				if d.errorContext == nil {
					d.errorContext = new(errorContext)
				}
				for i, ind := range f.index {
					if subv.Kind() == reflect.Pointer {
						if subv.IsNil() {
							// If a struct embeds a pointer to an unexported type,
							// it is not possible to set a newly allocated value
							// since the field is unexported.
							//
							// See https://golang.org/issue/21357
							if !subv.CanSet() {
								d.saveError(fmt.Errorf("yaml: cannot set embedded pointer to unexported struct: %v", subv.Type().Elem()))
								// Invalidate subv to ensure d.value(subv) skips over
								// the YAML value without assigning it to subv.
								subv = reflect.Value{}
								destring = false
								break
							}
							subv.Set(reflect.New(subv.Type().Elem()))
						}
						subv = subv.Elem()
					}
					if i < len(f.index)-1 {
						d.errorContext.FieldStack = append(
							d.errorContext.FieldStack,
							subv.Type().Field(ind).Name,
						)
					}
					subv = subv.Field(ind)
				}
				d.errorContext.Struct = t
				d.errorContext.FieldStack = append(d.errorContext.FieldStack, f.name)
			} else if d.disallowUnknownFields {
				d.saveError(fmt.Errorf("yaml: unknown field %q", key))
			}
		}

		// Read : before value.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode != scanObjectKey {
			panic(phasePanicMsg)
		}
		d.scanWhile(scanSkipSpace)

		if destring {
			switch qv := d.valueQuoted().(type) {
			case nil:
				if err := d.storeLiteral(nullLiteral, subv, false); err != nil {
					return err
				}
			case string:
				if err := d.storeLiteral([]byte(qv), subv, true); err != nil {
					return err
				}
			default:
				d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal unquoted value into %v", subv.Type()))
			}
		} else {
			if err := d.value(subv); err != nil {
				return err
			}
		}

		// Write value back to map;
		// if using struct, subv points into struct already.
		if v.Kind() == reflect.Map {
			kt := t.Key()
			var kv reflect.Value
			if reflect.PointerTo(kt).Implements(textUnmarshalerType) {
				kv = reflect.New(kt)
				if err := d.storeLiteral(item, kv, false); err != nil {
					return err
				}
				kv = kv.Elem()
			} else {
				s := string(key)
				switch kt.Kind() {
				case reflect.String:
					kv = reflect.New(kt).Elem()
					kv.SetString(s)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					kv = reflect.New(kt).Elem()
					if isDuration(kv) {
						d.storeDuration(key, kv)
						break
					}
					n, err := strconv.ParseInt(s, 10, 64)
					if err != nil || kt.OverflowInt(n) {
						d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: kt, Offset: int64(start + 1)})
						break
					}
					kv.SetInt(n)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
					n, err := strconv.ParseUint(s, 10, 64)
					if err != nil || kt.OverflowUint(n) {
						d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: kt, Offset: int64(start + 1)})
						break
					}
					kv = reflect.New(kt).Elem()
					kv.SetUint(n)
				case reflect.Float32, reflect.Float64:
					n, err := strconv.ParseFloat(s, 64)
					if err != nil || kt.OverflowFloat(n) {
						d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: kt, Offset: int64(start + 1)})
						break
					}
					kv = reflect.New(kt).Elem()
					kv.SetFloat(n)
				case reflect.Bool:
					n, err := strconv.ParseBool(s)
					if err != nil {
						d.saveError(&UnmarshalTypeError{Value: "bool " + s, Type: kt, Offset: int64(start + 1)})
						break
					}
					kv = reflect.New(kt).Elem()
					kv.SetBool(n)
				default:
					panic("yaml: Unexpected key type") // should never occur
				}
			}
			if kv.IsValid() {
				v.SetMapIndex(kv, subv)
			}
		}

		// Next token must be , or }.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.errorContext != nil {
			// Reset errorContext to its original state.
			// Keep the same underlying array for FieldStack, to reuse the
			// space and avoid unnecessary allocs.
			d.errorContext.FieldStack = d.errorContext.FieldStack[:len(origErrorContext.FieldStack)]
			d.errorContext.Struct = origErrorContext.Struct
		}
		if d.opcode == scanEndObject || d.opcode == scanEnd || d.scan.eof() == scanEnd {
			break
		}
		if d.opcode != scanObjectValue {
			panic(phasePanicMsg)
		}
	}
	return nil
}

// convertNumber converts the number literal s to a float64 or a Number
// depending on the setting of d.useNumber.
func (d *decodeState) convertNumber(s string) (any, error) {
	// if d.useNumber {
	// 	return Number(s), nil
	// }
	// nondecimal := len(s) > 2 &&
	//dig digit index
	dig := 0
	if s[0] == '-' || s[0] == '+' {
		dig++
	}
	if len(s) > 2+dig && s[dig] == '0' && (s[dig+1] == 'b' || s[dig+1] == 'o' || s[dig+1] == 'x') {
		i, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return nil, &UnmarshalTypeError{Value: "number " + s, Type: reflect.TypeFor[int64](), Offset: int64(d.off)}
		}
		return i, nil
	}
	if dig < len(s) && s[dig] == '.' && slices.Contains(reservedInfKeywords, s) {
		if s[0] == '-' {
			return math.Inf(-1), nil
		} else {
			return math.Inf(0), nil
		}
	}
	if dig < len(s) && s[dig] == '.' && slices.Contains(reservedNanKeywords, s) {
		return math.NaN(), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, &UnmarshalTypeError{Value: "number " + s, Type: reflect.TypeFor[float64](), Offset: int64(d.off)}
	}
	return f, nil
}

func isTime(v reflect.Value) bool {
	tt := reflect.TypeFor[time.Time]()
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return v.Kind() == reflect.Struct && v.Type() == tt
}
func isDuration(v reflect.Value) bool {
	return v.Kind() == reflect.Int64 && v.Type() == reflect.TypeFor[time.Duration]()
}

var numberType = reflect.TypeFor[Number]()

func (d *decodeState) storeTime(item []byte, v reflect.Value) error {
	var t time.Time
	var err error
	s := string(item)
	for _, f := range allowedTimestampFormats {
		t, err = time.Parse(f, s)
		if err == nil {
			if v.Kind() == reflect.Pointer {
				v.Elem().Set(reflect.ValueOf(t))
			} else {
				v.Set(reflect.ValueOf(t))
			}
			return nil
		}
	}
	d.saveError(fmt.Errorf("yaml: error while trying to unmarshal %q into %v", item, v.Type()))
	return nil
}

func (d *decodeState) storeDuration(item []byte, v reflect.Value) error {
	s := string(item)
	dur, err := time.ParseDuration(s)
	if err != nil {
		d.saveError(fmt.Errorf("yaml: error while trying to unmarshal %q into %v", item, v.Type()))
		return nil
	}
	if v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.ValueOf(dur))
	} else {
		v.Set(reflect.ValueOf(dur))
	}
	return nil
}

// storeLiteral decodes a literal stored in item into v.
//
// fromQuoted indicates whether this literal came from unwrapping a
// string from the ",string" struct tag option. this is used only to
// produce more helpful error messages.
// TODO: remove fromQuoted? It's not json where it is strictly defined
func (d *decodeState) storeLiteral(item []byte, v reflect.Value, fromQuoted bool) error {
	// Check for unmarshaler.
	if len(item) == 0 {
		// Empty string given.
		d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type()))
		return nil
	}
	if isDuration(v) {
		return d.storeDuration(item, v)
	}
	isNull := (item[0] == 'n' || item[0] == 'N' || item[0] == '~') && slices.Contains(reservedNullKeywords, string(item))
	u, ut, pv := indirect(v, isNull)
	if u != nil {
		return u.UnmarshalYAML(item)
	}
	if ut != nil {
		if item[0] != '"' && item[0] != '\'' {
			if fromQuoted {
				d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type()))
				return nil
			}
			if isTime(v) {
				return d.storeTime(item, v)
			}

			return ut.UnmarshalText(item)
			//TODO: remove?
			val := "number"
			switch item[0] {
			case 'n':
				val = "null"
			case 't', 'f':
				val = "bool"
			}
			d.saveError(&UnmarshalTypeError{Value: val, Type: v.Type(), Offset: int64(d.readIndex())})
			return nil
		}
		s, ok := unquoteBytes(item)
		if !ok {
			if fromQuoted {
				return fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type())
			}
			panic(phasePanicMsg)
		}
		return ut.UnmarshalText(s)
	}

	v = pv

	switch c := item[0]; c {
	// case 'n': // null
	// 	// The main parser checks that only true and false can reach here,
	// 	// but if this was a quoted string input, it could be anything.
	// 	if fromQuoted && string(item) != "null" {
	// 		d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type()))
	// 		break
	// 	}
	// 	switch v.Kind() {
	// 	case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
	// 		v.SetZero()
	// 		// otherwise, ignore null for primitives/string
	// 	}
	// case 't', 'f': // true, false
	// 	value := item[0] == 't'
	// 	// The main parser checks that only true and false can reach here,
	// 	// but if this was a quoted string input, it could be anything.
	// 	if fromQuoted && string(item) != "true" && string(item) != "false" {
	// 		d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type()))
	// 		break
	// 	}
	// 	switch v.Kind() {
	// 	default:
	// 		if fromQuoted {
	// 			d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type()))
	// 		} else {
	// 			d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
	// 		}
	// 	case reflect.Bool:
	// 		v.SetBool(value)
	// 	case reflect.Interface:
	// 		if v.NumMethod() == 0 {
	// 			v.Set(reflect.ValueOf(value))
	// 		} else {
	// 			d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
	// 		}
	// 	}

	case '"', '\'': // string
		s, ok := unquoteBytes(item)
		if !ok {
			if fromQuoted {
				return fmt.Errorf("yaml: invalid use of string struct tag, trying to unmarshal %q into %v", item, v.Type())
			}
			panic(phasePanicMsg)
		}
		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		// case reflect.Slice:
		// 	if v.Type().Elem().Kind() != reflect.Uint8 {
		// 		d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		// 		break
		// 	}
		// 	b := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
		// 	n, err := base64.StdEncoding.Decode(b, s)
		// 	if err != nil {
		// 		d.saveError(err)
		// 		break
		// 	}
		// 	v.SetBytes(b[:n])
		case reflect.String:
			t := string(s)
			if v.Type() == numberType && !isValidNumber(t) {
				return fmt.Errorf("yaml: invalid number literal, trying to unmarshal %q into Number", item)
			}
			v.SetString(t)
		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(string(s)))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		}

	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '+', '.':
		// if c != '-' && (c < '0' || c > '9') {
		// 	if fromQuoted {
		// 		return fmt.Errorf("yaml: invalid use of, string struct tag, trying to unmarshal %q into %v", item, v.Type())
		// 	}
		// 	panic(phasePanicMsg)
		// }
		switch v.Kind() {
		default:
			// if v.Kind() == reflect.String && v.Type() == numberType {
			// 	// s must be a valid number, because it's
			// 	// already been tokenized.
			// 	v.SetString(string(item))
			// 	break
			// }
			if fromQuoted {
				return fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type())
			}
			d.saveError(&UnmarshalTypeError{Value: "number", Type: v.Type(), Offset: int64(d.readIndex())})
		case reflect.String:
			v.SetString(string(item))

		case reflect.Interface:
			n, err := d.convertNumber(string(item))
			if err != nil {
				d.saveError(err)
				break
			}
			if v.NumMethod() != 0 {
				d.saveError(&UnmarshalTypeError{Value: "number", Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.Set(reflect.ValueOf(n))

		// case reflect.String:
		// 	if v.Type() == numberType && !isValidNumber(s) {
		// 		return fmt.Errorf("yaml: invalid number literal, trying to unmarshal %q into Number", item)
		// 	}
		// 	v.SetString(s)

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(string(item), 0, 64)
			if err != nil || v.OverflowInt(n) {
				d.saveError(&UnmarshalTypeError{Value: "number " + string(item), Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetInt(n)

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			n, err := strconv.ParseUint(string(item), 0, 64)
			if err != nil || v.OverflowUint(n) {
				d.saveError(&UnmarshalTypeError{Value: "number " + string(item), Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetUint(n)

		case reflect.Float32, reflect.Float64:
			n, err := strconv.ParseFloat(string(item), v.Type().Bits())
			if err != nil || v.OverflowFloat(n) {
				d.saveError(&UnmarshalTypeError{Value: "number " + string(item), Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetFloat(n)
		}
	default:
		//unquoted string
		s := string(item)
		//is null
		if isNull {
			switch v.Kind() {
			case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
				v.SetZero()
				// otherwise, ignore null for primitives/string
			}
			return nil
		}
		//is bool, I don't check legacy bools because yaml 1.2
		if (c == 't' || c == 'T' || c == 'f' || c == 'F') && slices.Contains(reservedBoolKeywords, s) {
			value := c == 't' || c == 'T'
			switch v.Kind() {
			default:
				if fromQuoted {
					d.saveError(fmt.Errorf("yaml: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type()))
				} else {
					d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
				}
			case reflect.String:
				if v.Type() == numberType && !isValidNumber(s) {
					return fmt.Errorf("yaml: invalid number literal, trying to unmarshal %q into Number", item)
				}
				v.SetString(s)
			case reflect.Bool:
				v.SetBool(value)
			case reflect.Interface:
				if v.NumMethod() == 0 {
					v.Set(reflect.ValueOf(value))
				} else {
					d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
				}
			}
			return nil
		}

		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		case reflect.String:
			if v.Type() == numberType && !isValidNumber(s) {
				return fmt.Errorf("yaml: invalid number literal, trying to unmarshal %q into Number", item)
			}
			v.SetString(s)
		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(s))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		}
	}
	return nil
}

// The xxxInterface routines build up a value to be stored
// in an empty interface. They are not strictly necessary,
// but they avoid the weight of reflection in this common case.

// valueInterface is like value but returns any.
func (d *decodeState) valueInterface() (val any) {
	switch d.opcode {
	default:
		panic(phasePanicMsg)
	case scanBeginArray:
		val = d.arrayInterface()
		d.scanNext()
	case scanBeginObject:
		val = d.objectInterface()
		d.scanNext()
	case scanBeginLiteral:
		val = d.literalInterface()
	}
	return
}

// arrayInterface is like array but returns []any.
func (d *decodeState) arrayInterface() []any {
	var v = make([]any, 0)
	for {
		// Look ahead for ] - can only happen on first iteration.
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndArray {
			break
		}

		v = append(v, d.valueInterface())

		// Next token must be , or ].
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray {
			break
		}
		if d.opcode != scanArrayValue {
			panic(phasePanicMsg)
		}
	}
	return v
}

// objectInterface is like object but returns map[string]any.
func (d *decodeState) objectInterface() map[string]any {
	m := make(map[string]any)
	for {
		// Read opening " of string key or closing }.
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndObject {
			// closing } - can only happen on first iteration.
			break
		}
		if d.opcode != scanBeginLiteral {
			panic(phasePanicMsg)
		}

		// Read string key.
		start := d.readIndex()
		d.rescanLiteral()
		item := d.data[start:d.readIndex()]
		key, ok := unquote(item)
		if !ok {
			panic(phasePanicMsg)
		}

		// Read : before value.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode != scanObjectKey {
			panic(phasePanicMsg)
		}
		d.scanWhile(scanSkipSpace)

		// Read value.
		m[key] = d.valueInterface()

		// Next token must be , or }.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndObject {
			break
		}
		if d.opcode != scanObjectValue {
			panic(phasePanicMsg)
		}
	}
	return m
}

// literalInterface consumes and returns a literal from d.data[d.off-1:] and
// it reads the following byte ahead. The first byte of the literal has been
// read already (that's how the caller knows it's a literal).
func (d *decodeState) literalInterface() any {
	// All bytes inside literal return scanContinue op code.
	start := d.readIndex()
	d.rescanLiteral()

	item := d.data[start:d.readIndex()]

	switch c := item[0]; c {
	//TODO: replace with reserved words contains check
	// case 'n': // null
	// 	return nil
	//
	// case 't', 'f': // true, false
	// 	return c == 't'

	case '"', '\'': // string
		s, ok := unquote(item)
		if !ok {
			panic(phasePanicMsg)
		}
		return s

	default: // number
		if unicode.IsLetter(rune(c)) {
			return string(item)
		}
		if c != '-' && (c < '0' || c > '9') {
			panic(phasePanicMsg)
		}
		n, err := d.convertNumber(string(item))
		if err != nil {
			d.saveError(err)
		}
		return n
	}
}

// getU8 decodes \uXXXXXXXX from the beginning of s, returning the hex value,
// or it returns -1.
func getU8(s []byte) rune {
	if len(s) < 10 || s[0] != '\\' || s[1] != 'U' {
		return -1
	}
	var r rune
	for _, c := range s[2:10] {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			return -1
		}
		r = r*16 + rune(c)
	}
	return r
}

// getu4 decodes \uXXXX from the beginning of s, returning the hex value,
// or it returns -1.
func getu4(s []byte) rune {
	if len(s) < 6 || s[0] != '\\' || s[1] != 'u' {
		return -1
	}
	var r rune
	for _, c := range s[2:6] {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			return -1
		}
		r = r*16 + rune(c)
	}
	return r
}

// getx2 decodes \xXX from the beginning of s, returning the hex value,
// or it returns -1.
func getx2(s []byte) rune {
	if len(s) < 4 || s[0] != '\\' || s[1] != 'x' {
		return -1
	}
	var r rune
	for _, c := range s[2:4] {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			return -1
		}
		r = r*16 + rune(c)
	}
	return r
}

// unquoteBytesSq performs unquoting for a single-quoted string
// \t, \b, \r, \n are escaped (probably the list must be extended, see unquotedBytes switches)
func unquoteBytesSq(s []byte) (t []byte, ok bool) {
	switch len(s) {
	case 0:
		return nil, true
	case 1:
		return s, s[0] != '\''
	default:
		singleQuoted := s[0] == '\'' && s[len(s)-1] == '\''
		if !singleQuoted {
			return t, true
		}
	}

	s = s[1 : len(s)-1]
	//replace '' with ' inside
	s = bytes.ReplaceAll(s, []byte{'\'', '\''}, []byte{'\''})
	b := make([]byte, 2*len(s)) // pessimistic cap
	w := 0
	for _, c := range s {
		switch c {
		case '\b':
			b[w] = '\\'
			w++
			b[w] = 'b'
		case '\t':
			b[w] = '\\'
			w++
			b[w] = 't'
		case '\f':
			b[w] = '\\'
			w++
			b[w] = 'f'
		case '\r':
			b[w] = '\\'
			w++
			b[w] = 'r'
		case '\n':
			b[w] = '\\'
			w++
			b[w] = 'n'
		default:
			b[w] = c
		}
		w++
	}
	return b[0:w], true
}

// unquote converts a quoted YAML string literal s into an actual string t.
// The rules are different than for Go, so cannot use strconv.Unquote.
func unquote(s []byte) (t string, ok bool) {
	s, ok = unquoteBytes(s)
	t = string(s)
	return
}

// unquoteBytes should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/bytedance/sonic
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
// +++ now also checks if already unquoted and returns untouched value
//
//go:linkname unquoteBytes
func unquoteBytes(s []byte) (t []byte, ok bool) {
	switch len(s) {
	case 0:
		return nil, true
	case 1:
		return s, s[0] != '"' && s[0] != '\''
	default:
		doubleQuoted := s[0] == '"' && s[len(s)-1] == '"'
		singleQuoted := s[0] == '\'' && s[len(s)-1] == '\''
		if singleQuoted {
			return unquoteBytesSq(s)
		}
		if !doubleQuoted {
			return s, true //if already unquoted x)
		}
	}
	s = s[1 : len(s)-1]

	// Check for unusual characters. If there are none,
	// then no unquoting is needed, so return a slice of the
	// original bytes.
	r := 0
	for r < len(s) {
		c := s[r]
		//TODO: optimize
		if c == '\\' || c == '"' || c < ' ' {
			break
		}
		if c < utf8.RuneSelf {
			r++
			continue
		}
		rr, size := utf8.DecodeRune(s[r:])
		if rr == utf8.RuneError && size == 1 {
			break
		}
		r += size
	}
	if r == len(s) {
		return s, true
	}

	b := make([]byte, len(s)+2*utf8.UTFMax)
	w := copy(b, s[0:r])
	for r < len(s) {
		// Out of room? Can only happen if s is full of
		// malformed UTF-8 and we're replacing each
		// byte with RuneError.
		if w >= len(b)-2*utf8.UTFMax {
			nb := make([]byte, (len(b)+utf8.UTFMax)*2)
			copy(nb, b[0:w])
			b = nb
		}
		switch c := s[r]; {
		case c == '\\':
			r++
			if r >= len(s) {
				return
			}
			switch s[r] {
			default:
				return
			case '"', '\\', '/', '\'':
				b[w] = s[r]
				r++
				w++
			case 'b':
				b[w] = '\b'
				r++
				w++
			case 'f':
				b[w] = '\f'
				r++
				w++
			case 'n':
				b[w] = '\n'
				r++
				w++
			case 'r':
				b[w] = '\r'
				r++
				w++
			case 't':
				b[w] = '\t'
				r++
				w++
			case 'x':
				r--
				rr := getx2(s[r:])
				if rr < 0 {
					return
				}
				r += 4
				w += utf8.EncodeRune(b[w:], rr)
			case 'u':
				r--
				rr := getu4(s[r:])
				if rr < 0 {
					return
				}
				r += 6
				if utf16.IsSurrogate(rr) {
					rr1 := getu4(s[r:])
					if dec := utf16.DecodeRune(rr, rr1); dec != unicode.ReplacementChar {
						// A valid pair; consume.
						r += 6
						w += utf8.EncodeRune(b[w:], dec)
						break
					}
					// Invalid surrogate; fall back to replacement rune.
					rr = unicode.ReplacementChar
				}
				w += utf8.EncodeRune(b[w:], rr)
			case 'U':
				r--
				rr := getU8(s[r:])
				if rr < 0 {
					return
				}
				r += 10
				if utf16.IsSurrogate(rr) {
					rr1 := getU8(s[r:])
					if dec := utf16.DecodeRune(rr, rr1); dec != unicode.ReplacementChar {
						// A valid pair; consume.
						r += 10
						w += utf8.EncodeRune(b[w:], dec)
						break
					}
					// Invalid surrogate; fall back to replacement rune.
					rr = unicode.ReplacementChar
				}
				w += utf8.EncodeRune(b[w:], rr)
			}

		// Quote, control characters are invalid.
		case c == '"', c < ' ':
			return

		// ASCII
		case c < utf8.RuneSelf:
			b[w] = c
			r++
			w++

		// Coerce to well-formed UTF-8.
		default:
			rr, size := utf8.DecodeRune(s[r:])
			r += size
			w += utf8.EncodeRune(b[w:], rr)
		}
	}
	return b[0:w], true
}

// TODO: do i have this already?
// isValidNumber reports whether s is a valid JSON number literal.
//
// isValidNumber should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/bytedance/sonic
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname isValidNumber
func isValidNumber(s string) bool {
	// This function implements the JSON numbers grammar.
	// See https://tools.ietf.org/html/rfc7159#section-6
	// and https://www.json.org/img/number.png

	if s == "" {
		return false
	}

	// Optional -
	if s[0] == '-' {
		s = s[1:]
		if s == "" {
			return false
		}
	}

	// Digits
	switch {
	default:
		return false

	case s[0] == '0':
		s = s[1:]

	case '1' <= s[0] && s[0] <= '9':
		s = s[1:]
		for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
			s = s[1:]
		}
	}

	// . followed by 1 or more digits.
	if len(s) >= 2 && s[0] == '.' && '0' <= s[1] && s[1] <= '9' {
		s = s[2:]
		for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
			s = s[1:]
		}
	}

	// e or E followed by an optional - or + and
	// 1 or more digits.
	if len(s) >= 2 && (s[0] == 'e' || s[0] == 'E') {
		s = s[1:]
		if s[0] == '+' || s[0] == '-' {
			s = s[1:]
			if s == "" {
				return false
			}
		}
		for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
			s = s[1:]
		}
	}

	// Make sure we are at the end.
	return s == ""
}

// This is a subset of the formats allowed by the regular expression
// defined at http://yaml.org/type/timestamp.html.
var allowedTimestampFormats = []string{
	"2006-1-2T15:4:5.999999999Z07:00", // RCF3339Nano with short date fields.
	"2006-1-2t15:4:5.999999999Z07:00", // RFC3339Nano with short date fields and lower-case "t".
	"2006-1-2 15:4:5.999999999",       // space separated with no time zone
	"2006-1-2",                        // date only
}
