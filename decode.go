// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Represents YAML data structure using native Go types: booleans, floats,
// strings, arrays, structs and maps.

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
	// Check for well-formedness.
	// Avoids filling out half a data structure
	// before discovering a YAML syntax error.
	var d decodeState
	err := checkValid(data, &d.scan)
	if err != nil {
		return err
	}

	//Unmarshal should support only single document in buffer
	//if there is a new document separator "---" the first occurence is the document beginning
	start := documentStartIndex(data)
	if start > 0 {
		data = data[start:]
		startNext := documentStartIndex(data)
		if startNext > 0 {
			n := max(0, startNext-4)
			data = data[:n]
		}
	}

	//if there is an end document separator "..." ignore data following it
	end := documentEndIndex(data)
	if end > -1 {
		data = data[:end]
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

// An UnmarshalError describes a generic YAML unmarshal error
// Is intended to replace original (coming from json) panics with errors for readability
type UnmarshalError struct {
	Offset  int
	Type    reflect.Type
	Message string
}

func (e *UnmarshalError) Error() string {
	return "yaml: cannot unmarshal into type " + e.Type.String() + ", " + e.Message + ", offset: " + strconv.FormatInt(int64(e.Offset), 10)
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
		return "yaml: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String() + ", offset: " + strconv.Itoa(int(e.Offset))
	}
	return "yaml: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String() + ", offset: " + strconv.Itoa(int(e.Offset))
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

func (d *decodeState) unmarshal(v any) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = d.addErrorContext(fmt.Errorf("recovered panic: %v", p))
		}
	}()
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	d.scan.reset()
	d.scanWhile(scanSkipSpace)
	// We decode rv not rv.Elem because the Unmarshaler interface
	// test must be applied at the top level of the value.
	err = d.value(rv)
	if err != nil {
		return d.addErrorContext(err)
	}
	return d.savedError
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
	disallowUnknownFields bool

	// anchors map[string]uintptr
	anchors map[string]reflect.Value
}

// readIndex returns the position of the last byte read.
func (d *decodeState) readIndex() int {
	return d.off - 1
}

func (d *decodeState) init(data []byte) *decodeState {
	d.data = data
	d.off = 0
	d.anchors = make(map[string]reflect.Value)
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
	s, data, i, op := &d.scan, d.data, d.off, d.opcode
	depth := len(s.states)
	// for {
	for i < len(data) {
		op = s.step(s, data[i])
		s.lastInput(data[i])
		i++
		if len(s.states) < depth {
			d.off = i
			d.opcode = op
			return
		}
	}
	//if end of data reached
	d.off, d.opcode = len(data)-1, op
}

// scanNext processes the byte at d.data[d.off].
func (d *decodeState) scanNext() {
	if d.off < len(d.data) {
		d.opcode = d.scan.step(&d.scan, d.data[d.off])
		d.scan.lastInput(d.data[d.off])
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
		s.lastInput(data[i])
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

// quickScan is checking the literal value if its a block object key / array element or an ordinary string
// if the literal value is an anchor (starting with '&') it advances the scanner to the next value and returns anchor
// name as second value
func (d *decodeState) quickScan() (int, []byte) {
	var anchor []byte
	data, i := d.data, d.off
Switch:
	switch data[i-1] {
	case '"':
		//skip double-quoted string value. We will need what comes after it
		scopy := d.scan
		scopy.reset()
		scopy.step = stateInString
		for ; i < len(data); i++ {
			if scopy.step(&scopy, data[i]) != scanContinue {
				break Switch
			}
		}
	case '\'':
		//skip single quoted string too
		scopy := d.scan
		scopy.reset()
		scopy.step = stateInStringSq
		for ; i < len(data); i++ {
			if scopy.step(&scopy, data[i]) != scanContinue {
				break Switch
			}
		}
	case '-':
		if i == len(data) || isWhiteSpace(data[i]) {
			return scanBeginArray, nil
		}
	case '&':
		d.scanWhile(scanContinue)
		anchor = d.data[i : d.off-1]
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		//if literal again rerun the quickScan
		if d.opcode == scanBeginLiteral {
			opcode, _ := d.quickScan()
			return opcode, anchor
		}
		return d.opcode, anchor
	}
	//because input passed scanner validity check I don't expect some weird behavior here, but..
	for ; i < len(data); i++ {
		//TODO: also check if multiline literal or smth
		if d.scan.isUnqDelim(data[i]) {
			break
		}
		//have to duplicate some scanner logic here
		//basically ": " means object key, but.. when key is a quoted string u can skip that whitespace check (atleast in
		//flow mode for sure), but we let it be for block mode also...
		if data[i] == ':' && (i+1 == len(data) || isWhiteSpace(data[i+1]) || d.scan.lastc == '"' || d.scan.lastc == '\'') {
			return scanBeginObject, nil
		}
	}
	return scanBeginLiteral, nil
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
	data, i := d.data, d.off
Switch:
	switch c := data[i-1]; c {
	case '|', '>':
		if d.scan.inMultilineString() {
			depth := len(d.scan.states)
			for ; i < len(data); i++ {
				d.scan.step(&d.scan, data[i])
				d.scan.lastInput(data[i])
				if len(d.scan.states) < depth {
					break Switch
				}
			}
		}
	case '!': //explicit type
		for ; i < len(data); i++ {
			d.opcode = d.scan.step(&d.scan, data[i])
			d.scan.lastInput(data[i])
			switch d.opcode {
			case scanSkipSpace, scanContinue:
				continue
			case scanBeginLiteral:
				//if value after ex. "!!int " found restart scan
				i++ //so data[i-1] points to the current character
				goto Switch
			default:
				break Switch
			}
		}

	default:
		for ; i < len(data); i++ {
			d.opcode = d.scan.step(&d.scan, data[i])
			d.scan.lastInput(data[i])
			if d.opcode != scanContinue {
				break Switch
			}
		}
	}
	if i == len(data) {
		d.opcode = scanEnd
	}
	if (d.scan.lastc == ':') &&
		(i < len(data) && data[i] != ':' || i == len(data)) &&
		(d.opcode == scanObjectKey || d.opcode == scanEnd) {
		i-- //go back to ":"
	}
	d.off = i + 1
}

// value consumes a YAML value from d.data[d.off-1:], decoding into v, and
// reads the following byte ahead. If v is invalid, the value is discarded.
// The first byte of the value has been read already.
func (d *decodeState) value(v reflect.Value) error {
	//because this no json, if literal opcode, need to make sure it's not a nested block object key "a: "
	//or array element "- a"
	if d.opcode == scanBeginLiteral {
		var anchor []byte
		d.opcode, anchor = d.quickScan()
		if len(anchor) != 0 {
			defer func() {
				if v.IsValid() {
					//create a copy of v to break pointer
					copyv := reflect.New(v.Type()).Elem()
					copyv.Set(v)
					d.anchors[string(anchor)] = copyv
				} else {
					d.saveError(fmt.Errorf("yaml: cannot save anchor %q, no valid unmarshal type specified", anchor))
				}
			}()
		}
	}
	switch d.opcode {
	default:
		return &UnmarshalError{Offset: d.readIndex(), Type: v.Type(), Message: fmt.Sprintf("unexpected parser code %d", d.opcode)}

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
		//Originally was unconditional, but block arrays dont need this skip
		if d.readIndex() < len(d.data) && d.data[d.readIndex()] == ']' {
			d.scanNext()
		}

	// case scanBeginObject:
	case scanBeginObject, scanObjectKey:
		if v.IsValid() {
			if err := d.object(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		//Originally was unconditional, but block objects dont need this skip
		if d.readIndex() < len(d.data) && d.data[d.readIndex()] == '}' {
			d.scanNext()
		}

	case scanObjectValue:
		//value was empty and no expected scanBeginLiteral code.
		return nil

	case scanBeginLiteral:
		start := d.readIndex()
		d.rescanLiteral()

		if v.IsValid() {
			if err := d.storeLiteral(d.data[start:d.readIndex()], v); err != nil {
				return err
			}
		}
	}
	return nil
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
// The first byte of the array ('[') (if flow style) has been read already.
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
			ai, err := d.arrayInterface()
			if err != nil {
				return err
			}
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
	depth := len(d.scan.states)
	if !d.scan.inFlowArray() {
		//block array state is pushed to stack only after first "- " has been parsed, so inc depth preemptively
		depth++
	}
	for {
		//for second value and on in a block array the "- " can have a scanBeginLiteral code sometimes, for ex. after
		//parsing a nested multiline literal, check this
		if d.scan.inBlockArray() && d.opcode == scanBeginLiteral && d.data[d.readIndex()] == '-' {
			d.opcode, _ = d.quickScan()
		}
		if d.opcode == scanBeginArray || d.opcode == scanArrayValue || d.scan.inFlowArray() {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray || d.opcode == scanEnd || len(d.scan.states) < depth {
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

		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray || d.opcode == scanEnd || !d.scan.inArray() {
			break
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

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
var unmarshalerType = reflect.TypeFor[Unmarshaler]()

// object consumes an object from d.data[d.off-1:], decoding into v.
// The first byte ('{') (if flow style) of the object has been read already.
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
		oi, err := d.objectInterface()
		if err != nil {
			return err
		}
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
			reflect.Float32, reflect.Float64, reflect.Bool, reflect.Interface:
		default:
			if !reflect.PointerTo(t.Key()).Implements(textUnmarshalerType) &&
				!reflect.PointerTo(t.Key()).Implements(unmarshalerType) {
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
		c := d.data[d.off-1]
		depth := len(d.scan.states)
		//skip delimiters or white space
		if c == ',' || c == '{' || c == '}' || isWhiteSpace(c) {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndObject || d.opcode == scanEnd || len(d.scan.states) < depth {
			break
		}

		// Read key.
		start := d.readIndex()
		d.rescanLiteral()
		item := d.data[start:d.readIndex()]
		key, ok := unquoteBytes(item)
		if !ok {
			return &UnmarshalError{Offset: d.readIndex(), Type: v.Type(), Message: fmt.Sprintf("cannot unquote string %s", item)}
		}

		// Figure out field corresponding to key.
		var subv reflect.Value

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
		if d.opcode != scanObjectKey && d.opcode != scanEnd {
			return &UnmarshalError{Offset: d.readIndex(), Type: v.Type(), Message: fmt.Sprintf("expected object key, got: %c", d.data[d.readIndex()])}
		}
		depth = len(d.scan.states)
		d.scanWhile(scanSkipSpace)
		//if stack decreased or state value switched back to parseObjectKey then value is empty (==nil)
		if len(d.scan.states) < depth || len(d.scan.states) == depth && d.scan.lastState() == parseObjectKey {
			subv.Set(reflect.Zero(subv.Type()))
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
			if len(key) == 0 {
				if len(item) > 0 && (item[0] == '"' || item[0] == '\'') {
					kv = reflect.ValueOf(string(key))
				} else {
					kv = d.keyInterface(key)
				}
			} else if key[0] == '*' {
				//if key is an alias
				var found bool
				//TODO: check types? xD
				kv, found = d.anchors[string(key[1:])]
				if !found {
					return fmt.Errorf("yaml: anchor not found for alias %q", key)
				}
			} else if reflect.PointerTo(kt).Implements(textUnmarshalerType) {
				kv = reflect.New(kt)
				if err := d.storeLiteral(item, kv); err != nil {
					return err
				}
				kv = kv.Elem()
			} else if reflect.PointerTo(kt).Implements(unmarshalerType) {
				kv = reflect.New(kt)
				if err := d.storeLiteral(item, kv); err != nil {
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
						if err := d.storeDuration(key, kv); err != nil {
							d.saveError(&UnmarshalTypeError{Value: "duration " + s, Type: kt, Offset: int64(start + 1)})
						}
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
					n, err := convertFloat(s, 64)
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
				case reflect.Interface:
					//if was originally quoted then string for sure and evade parsing '123' as int
					if item[0] == '"' || item[0] == '\'' {
						kv = reflect.ValueOf(string(key))
					} else {
						kv = d.keyInterface(key)
					}
				default:
					return &UnmarshalError{Offset: d.readIndex(), Type: v.Type(), Message: "unexpected key type"}
				}
			}
			if kv.IsValid() {
				v.SetMapIndex(kv, subv)
			}
		}
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		//if nested depth decreased while scanning then end of object
		if len(d.scan.states) < depth {
			break
		}
		if d.errorContext != nil {
			// Reset errorContext to its original state.
			// Keep the same underlying array for FieldStack, to reuse the
			// space and avoid unnecessary allocs.
			d.errorContext.FieldStack = d.errorContext.FieldStack[:len(origErrorContext.FieldStack)]
			d.errorContext.Struct = origErrorContext.Struct
		}
		if d.opcode == scanEndObject || d.opcode == scanEnd {
			break
		}
	}
	return nil
}

// convertNumber converts the number literal s to a int64 or float64
// also checks for reserved words .inf, .nan
func (d *decodeState) convertNumber(s string) (any, error) {
	//first digit index
	i := 0
	if s[0] == '-' || s[0] == '+' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		return convertFloat(s, 64)
	}

	base := numberBase([]byte(s))
	n, err := strconv.ParseInt(s, base, 64)
	if err == nil {
		return n, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, &UnmarshalTypeError{Value: "number " + s, Type: reflect.TypeFor[float64](), Offset: int64(d.off)}
	}
	return f, nil
}

// isTime checks if v is a time.Time
func isTime(v reflect.Value) bool {
	tt := reflect.TypeFor[time.Time]()
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return v.Kind() == reflect.Struct && v.Type() == tt
}

// isDuration checks if v is a time.Duration
func isDuration(v reflect.Value) bool {
	return v.Kind() == reflect.Int64 && v.Type() == reflect.TypeFor[time.Duration]()
}

// storeExplicit stores item value in v if explicit yaml tag is specified: !!float, !!str, etc
func (d *decodeState) storeExplicit(item []byte, v reflect.Value) error {
	ty, data, found := bytes.Cut(item, []byte{' '})
	if !found || !bytes.HasPrefix(ty, []byte{'!', '!'}) {
		d.saveError(&UnmarshalTypeError{Value: string(item[:min(len(item), 5)]), Type: v.Type(), Offset: int64(d.readIndex())})
		return nil
	}
	ty, _ = bytes.CutPrefix(ty, []byte{'!', '!'})
	data, ok := unquoteBytes(data)
	if !ok {
		d.saveError(&UnmarshalTypeError{Value: string(item[:min(len(item), 5)]), Type: v.Type(), Offset: int64(d.readIndex())})
		return nil
	}
	if len(data) > 0 && (data[0] == '|' || data[0] == '>') {
		data = d.parseMultiline(data)
	}
	switch sty := string(ty); sty {
	case "str":
		return d.storeString(data, v)
	case "int":
		return d.storeInt(data, v)
	case "float":
		return d.storeFloat(data, v)
	case "bool":
		if slices.Contains(reservedBoolKeywords, string(data)) {
			return d.storeBool(data, v)
		}
		d.saveError(fmt.Errorf("yaml: unsupported bool value with explicit type %q", item))
	case "null":
		return d.storeNull(data, v)
	// case "set":
	// case "map", "omap":
	// case "seq":
	// case "timestamp":
	// 	return d.storeTime(data, v)
	// case "binary":
	// 	return d.storeBase64(data, v)
	default:
		d.saveError(fmt.Errorf("yaml: unsupported explicit type %q", item))
	}

	return nil
}

// storeBase64 decodes base64 value when explicit !!binary type is specified for value
// dropped since yaml 1.2
// func (d *decodeState) storeBase64(item []byte, v reflect.Value) error {
// 	b := make([]byte, base64.StdEncoding.DecodedLen(len(item)))
// 	n, err := base64.StdEncoding.Decode(b, item)
// 	if err != nil {
// 		d.saveError(err)
// 		return nil
// 	}
// 	switch v.Kind() {
// 	case reflect.String:
// 		v.SetString(string(b[:n]))
// 	default:
// 		v.SetBytes(b[:n])
// 	}
//
// 	return nil
// }

// storeInt stores int value when explicit !!int type is specified for value
func (d *decodeState) storeInt(item []byte, v reflect.Value) error {
	switch v.Kind() {
	default:
		d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(d.readIndex())})

	case reflect.Interface:
		n, err := strconv.ParseInt(string(item), 0, 64)
		if err != nil {
			d.saveError(&UnmarshalTypeError{Value: "integer " + string(item), Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.Set(reflect.ValueOf(n))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(string(item), 0, 64)
		if err != nil || v.OverflowInt(n) {
			d.saveError(&UnmarshalTypeError{Value: "integer " + string(item), Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(string(item), 0, 64)
		if err != nil || v.OverflowUint(n) {
			d.saveError(&UnmarshalTypeError{Value: "integer " + string(item), Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.SetUint(n)
	}

	return nil
}

// storeFloat stores float value when explicit !!float type is specified for value
func (d *decodeState) storeFloat(item []byte, v reflect.Value) error {
	s := string(item)
	switch v.Kind() {
	default:
		d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(d.readIndex())})

	case reflect.Interface:
		n, err := convertFloat(s, 64)
		if err != nil {
			d.saveError(&UnmarshalTypeError{Value: "float " + s, Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.Set(reflect.ValueOf(n))

	case reflect.Float32, reflect.Float64:
		n, err := convertFloat(s, v.Type().Bits())
		if err != nil || v.OverflowFloat(n) {
			d.saveError(&UnmarshalTypeError{Value: "float " + s, Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.SetFloat(n)
	}

	return nil
}

func (d *decodeState) storeTime(item []byte, v reflect.Value) error {
	item, ok := unquoteBytes(item)
	if !ok {
		d.saveError(&UnmarshalTypeError{Value: "timestamp", Type: v.Type(), Offset: int64(d.readIndex())})
		return nil
	}
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
	d.saveError(&UnmarshalTypeError{Value: string(item[:min(len(item), 5)]), Type: v.Type(), Offset: int64(d.readIndex())})
	return nil
}

func (d *decodeState) storeDuration(item []byte, v reflect.Value) error {
	s := string(item)
	dur, err := time.ParseDuration(s)
	if err != nil {
		d.saveError(&UnmarshalTypeError{Value: string(item[:min(len(item), 5)]), Type: v.Type(), Offset: int64(d.readIndex())})
		return nil
	}
	if v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.ValueOf(dur))
	} else {
		v.Set(reflect.ValueOf(dur))
	}
	return nil
}

func (d *decodeState) storeBool(item []byte, v reflect.Value) error {
	c := item[0]
	s := string(item)
	value := c == 't' || c == 'T'
	switch v.Kind() {
	default:
		d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
	case reflect.String:
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

func (d *decodeState) storeString(item []byte, v reflect.Value) error {
	item, ok := unquoteBytes(item)
	if !ok {
		d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		return nil
	}
	s := string(item)
	switch v.Kind() {
	default:
		d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
	case reflect.String:
		v.SetString(s)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(s))
		} else {
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		}
	}
	return nil
}

func (d *decodeState) storeNull(_ []byte, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
		v.SetZero()
		// otherwise, ignore null for primitives/string
	}
	return nil
}

// keyInterface parses item value into key value for a map[any]
func (d *decodeState) keyInterface(item []byte) reflect.Value {
	if len(item) == 0 {
		return reflect.ValueOf(nil)
	}
	//item is already unquoted
	s := string(item)
	switch c := item[0]; {
	case c == '-' || c == '+' || c == '.' || '0' <= c && c <= '9':
		n, err := d.convertNumber(s)
		if err == nil {
			return reflect.ValueOf(n)
		} else {
			return reflect.ValueOf(s)
		}
	case (c == '~' || c == 'n' || c == 'N') && slices.Contains(reservedNullKeywords, s):
		return reflect.Zero(reflect.PointerTo(reflect.TypeFor[string]()))
	case (c == 't' || c == 'T' || c == 'f' || c == 'F') && slices.Contains(reservedBoolKeywords, s):
		return reflect.ValueOf(c == 't' || c == 'T')
	default:
		return reflect.ValueOf(s)
	}
}

func numberBase(item []byte) int {
	base := 10
	i := 0
	if item[0] == '-' || item[0] == '+' {
		i++
	}

	if len(item) > 1+i && item[i] == '0' && (item[i+1] == 'b' || item[i+1] == 'o' || item[i+1] == 'x') {
		base = 0 //let parser autodetect
	}
	return base
}

// storeNumber is a generic number parser used by storeLiteral
func (d *decodeState) storeNumber(item []byte, v reflect.Value) error {
	//already unquoted
	item = bytes.TrimRight(item, "\t ") // welcome to yaml, spaces everywhere
	s := string(item)
	switch v.Kind() {
	default:
		d.saveError(&UnmarshalTypeError{Value: "number", Type: v.Type(), Offset: int64(d.readIndex())})
	case reflect.String:
		v.SetString(s)

	case reflect.Interface:
		n, err := d.convertNumber(s)
		if err != nil {
			v.Set(reflect.ValueOf(s))
			break
		}
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "number", Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.Set(reflect.ValueOf(n))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		base := numberBase(item)
		n, err := strconv.ParseInt(s, base, 64)
		if err != nil || v.OverflowInt(n) {
			d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		base := numberBase(item)
		n, err := strconv.ParseUint(s, base, 64)
		if err != nil || v.OverflowUint(n) {
			d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.SetUint(n)

	case reflect.Float32, reflect.Float64:
		n, err := convertFloat(s, v.Type().Bits())
		if err != nil || v.OverflowFloat(n) {
			d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: v.Type(), Offset: int64(d.readIndex())})
			break
		}
		v.SetFloat(n)
	}
	return nil
}

// convertFloat converts string s to float, checking .inf/.nan reserved words
func convertFloat(s string, bitSize int) (float64, error) {
	i := 0
	if s[0] == '-' || s[0] == '+' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		if slices.Contains(reservedInfKeywords, s[i:]) {
			if s[0] == '-' {
				return math.Inf(-1), nil
			} else {
				return math.Inf(0), nil
			}
		}
		if slices.Contains(reservedNanKeywords, s[i:]) {
			return math.NaN(), nil
		}
	}
	return strconv.ParseFloat(s, bitSize)
}

// convertInterface tries to convert v of kind reflect.Interface or []reflect.Interface/map to t
func (d *decodeState) convertInterface(v reflect.Value, t reflect.Type) (reflect.Value, error) {
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Zero(t), nil
		}
		v = v.Elem()
	}

	tk := t.Kind()
	switch v.Kind() {
	case reflect.Slice:
		if t.Kind() != reflect.Slice {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
		ret := reflect.MakeSlice(t, v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			el, err := d.convertInterface(el, t.Elem())
			if err != nil {
				return reflect.Zero(t), err
			}
			ret.Index(i).Set(el)
		}
		return ret, nil
	case reflect.Map:
		if t.Kind() != reflect.Map {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
		ret := reflect.MakeMap(t)
		keys := v.MapKeys()
		for _, k := range keys {
			key, err := d.convertInterface(k, t.Key())
			if err != nil {
				return reflect.Zero(t), err
			}
			value, err := d.convertInterface(v.MapIndex(k), t.Elem())
			if err != nil {
				return reflect.Zero(t), err
			}
			ret.SetMapIndex(key, value)
		}
		return ret, nil
	case reflect.Struct:
		if t != v.Type() {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
		ret := reflect.New(t).Elem()
		ret.Set(v)
		return ret, nil
	case reflect.Bool:
		if t.Kind() != reflect.Bool {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
	case reflect.String:
		if tk != reflect.String {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		if tk != reflect.Uint && tk != reflect.Uint8 && tk != reflect.Uint16 && tk != reflect.Uint32 && tk != reflect.Uint64 {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
		ret := reflect.New(t).Elem()
		if ret.OverflowUint(v.Uint()) {
			return reflect.Zero(t), fmt.Errorf("overflow when converting %v interface value to %v", v.Type(), t)
		}
		ret.SetUint(v.Uint())
		return ret, nil
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8:
		if tk != reflect.Int && tk != reflect.Int8 && tk != reflect.Int16 && tk != reflect.Int32 && tk != reflect.Int64 {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
		ret := reflect.New(t).Elem()
		if ret.OverflowInt(v.Int()) {
			return reflect.Zero(t), fmt.Errorf("overflow when converting %v interface value to %v", v.Type(), t)
		}
		ret.SetInt(v.Int())
		return ret, nil
	case reflect.Float32, reflect.Float64:
		if tk != reflect.Float32 && tk != reflect.Float64 {
			return reflect.Zero(t), fmt.Errorf("error converting %v interface value to %v", v.Type(), t)
		}
		ret := reflect.New(t).Elem()
		if ret.OverflowFloat(v.Float()) {
			return reflect.Zero(t), fmt.Errorf("overflow when converting %v interface value to %v", v.Type(), t)
		}
		ret.SetFloat(v.Float())
		return ret, nil
	}
	return v, nil
}

// storeLiteral decodes a literal stored in item into v.
func (d *decodeState) storeLiteral(item []byte, v reflect.Value) error {
	if len(item) == 0 {
		v.SetZero()
		return nil
	}

	if item[0] == '*' {
		alias := string(item[1:]) //cut '*'
		av, found := d.anchors[alias]
		if !found {
			// Empty string given.
			d.saveError(fmt.Errorf("yaml: anchor %q not found, trying to unmarshal %q into %v", alias, item, v.Type()))
			return nil
		}
		if av.Kind() == reflect.Interface && av.Kind() != v.Kind() {
			var err error
			if av, err = d.convertInterface(av, v.Type()); err != nil {
				d.saveError(fmt.Errorf("yaml: type mismatch for alias %q, trying to unmarshal %v into %v", alias, av.Type(), v.Type()))
				return nil
			}
		}
		if av.Kind() != v.Kind() {
			d.saveError(fmt.Errorf("yaml: type mismatch for alias %q, trying to unmarshal %v into %v", alias, av.Type(), v.Type()))
			return nil
		}
		v.Set(av)
		return nil
	}
	if item[0] == '!' {
		return d.storeExplicit(item, v)
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
			if isTime(v) {
				return d.storeTime(item, v)
			}

			return ut.UnmarshalText(item)
		}
		s, ok := unquoteBytes(item)
		if !ok {
			return &UnmarshalError{Offset: d.readIndex(), Type: v.Type(), Message: "unmatched quotes"}
		}
		return ut.UnmarshalText(s)
	}

	v = pv

	switch c := item[0]; c {
	case '"', '\'': // string
		return d.storeString(item, v)

	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '+', '.':
		return d.storeNumber(item, v)

	default:
		//is null
		if isNull {
			return d.storeNull(item, v)
		}
		//unquoted string
		item = bytes.TrimRight(item, " ")
		if c == '|' || c == '>' {
			item = d.parseMultiline(item)
		}
		s := string(item)
		//if bool, only yaml 1.2 compliant values are checked
		if (c == 't' || c == 'T' || c == 'f' || c == 'F') && slices.Contains(reservedBoolKeywords, s) {
			return d.storeBool(item, v)
		}

		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		case reflect.String:
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

// parseMultiline parses a multiline literal string and returns a formatted string literal
func (d *decodeState) parseMultiline(item []byte) []byte {
	var literalStyle = item[0] == '|' //otherwise folded style '>'
	var chomp = 0                     //chomping indicator
	const (
		clip  int = iota //'|' or '>' keep single trailing newline
		strip            //"|-" or ">-" removes all trailing newlines
		keep             //"|+" or ">+" preserves all trailing newlines and blank lines
	)

	//s non-empty always and first char is '|'
	lbc := detectLineBreakChars(item)
	lines := bytes.Split(item, lbc)
	header := bytes.TrimRight(lines[0], " ")
	//scanner should have caught bad headers
	if len(header) == 2 {
		if header[1] == '-' {
			chomp = strip
		} else {
			chomp = keep
		}
	}

	lines = lines[1:] //first line is "|\n" skip it
	if len(lines) == 0 {
		return nil
	}
	indent := make([]byte, 0, 10)
	for i := range lines {
		emptyLine := !nonSpace(lines[i])
		if len(indent) == 0 && !emptyLine {
			for _, c := range lines[i] {
				if c != ' ' {
					break
				}
				indent = append(indent, ' ') //calc indent for the first non-blank line
			}
		}
		if emptyLine {
			lines[i] = nil //replace blank line with empty
		} else {
			lines[i] = bytes.TrimPrefix(lines[i], indent)
		}
	}
	//for folded style with keep chomp dont replace trailing line-breaks with spaces, count them
	foldedTrailingN := 0
	switch chomp {
	case clip:
		for i := len(lines) - 1; i >= 0; i-- {
			if lines[i] == nil {
				lines = lines[:i]
				continue
			}
			break
		}
		//if not a totally empty string add an empty line to the end
		if len(lines) > 0 {
			lines = append(lines, nil)
		}
	case strip:
		for i := len(lines) - 1; i >= 0; i-- {
			if lines[i] == nil {
				lines = lines[:i]
				continue
			}
			break
		}
	case keep:
		if !literalStyle {
			for i := len(lines) - 1; i >= 0; i-- {
				if lines[i] == nil {
					foldedTrailingN++
					continue
				}
				break
			}
		}
	}
	if literalStyle {
		return bytes.Join(lines, []byte{'\n'}) //here we replace lbc with '\n'
	}
	folded := bytes.Join(lines, []byte{' '}) //here we replace lbc with ' '
	if len(folded) > 0 {
		switch chomp {
		case clip:
			//replace last space (and it is space) with line-break
			folded[len(folded)-1] = '\n'
		case keep:
			//replace last [foldedTrailingN] spaces (and they are) with line-breaks
			for i := 0; i < foldedTrailingN; i++ {
				folded[len(folded)-1-i] = '\n'
			}
		}
	}
	return folded
}

// The xxxInterface routines build up a value to be stored
// in an empty interface. They are not strictly necessary,
// but they avoid the weight of reflection in this common case.

// valueInterface is like value but returns any.
func (d *decodeState) valueInterface() (val any, err error) {
	//because this no json, if literal opcode, need to make sure it's not a nested block object key "a: "
	//or array element "- a"
	if d.opcode == scanBeginLiteral && (d.scan.inBlockArray() || d.scan.inBlockObject()) {
		var anchor []byte
		d.opcode, anchor = d.quickScan()
		if len(anchor) != 0 {
			defer func() {
				d.anchors[string(anchor)] = reflect.ValueOf(val)
			}()
		}
	}
	switch d.opcode {
	default:
		return nil, &UnmarshalError{Type: reflect.TypeFor[any](), Offset: d.off, Message: fmt.Sprintf("unexpected parser code %d", d.opcode)}
	case scanEnd:
		return nil, nil
	case scanBeginArray:
		val, err = d.arrayInterface()
		//Originally was unconditional, but block arrays dont need this skip
		if d.readIndex() < len(d.data) && d.data[d.readIndex()] == ']' {
			d.scanNext()
		}
	case scanBeginObject, scanObjectKey:
		val, err = d.objectInterface()
		//Originally was unconditional, but block arrays dont need this skip
		if d.readIndex() < len(d.data) && d.data[d.readIndex()] == '}' {
			d.scanNext()
		}
	case scanBeginLiteral:
		val, err = d.literalInterface()
	}
	return
}

// arrayInterface is like array but returns []any.
func (d *decodeState) arrayInterface() ([]any, error) {
	var v = make([]any, 0)
	depth := len(d.scan.states)
	if !d.scan.inFlowArray() {
		//block array state is pushed to stack only after first "- " has been parsed, so inc preemptively
		depth++
	}
	for {
		if d.opcode == scanBeginArray || d.opcode == scanArrayValue || d.scan.inFlowArray() {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray || d.opcode == scanEnd || len(d.scan.states) < depth {
			break
		}

		val, err := d.valueInterface()
		if err != nil {
			return nil, err
		}
		v = append(v, val)

		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray || d.opcode == scanEnd || len(d.scan.states) < depth {
			break
		}
	}
	return v, nil
}

// objectInterface is like object but returns map[string]any.
// TODO: probably should upgrade it to map[any]any at some point for usability
func (d *decodeState) objectInterface() (map[string]any, error) {
	m := make(map[string]any)
	for {
		c := d.data[d.off-1]
		depth := len(d.scan.states)
		if c == ',' || c == '{' || c == '}' || isWhiteSpace(c) {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndObject || d.opcode == scanEnd || len(d.scan.states) < depth {
			break
		}
		if d.opcode != scanBeginLiteral && d.opcode != scanBeginObject && d.opcode != scanObjectKey {
			return nil, &UnmarshalError{Offset: d.readIndex(), Type: reflect.TypeFor[any](), Message: fmt.Sprintf("unexpected parser code %d", d.opcode)}
		}

		// Read string key.
		start := d.readIndex()
		d.rescanLiteral()
		item := d.data[start:d.readIndex()]
		var key string
		var ok bool
		if item[0] == '*' {
			anchor, ok := d.anchors[string(item[1:])]
			if !ok {
				return nil, &UnmarshalError{Offset: d.readIndex(), Type: reflect.TypeFor[any](), Message: fmt.Sprintf("anchor %q not found", item)}
			}
			key = fmt.Sprintf("%v", anchor)
		} else {
			key, ok = unquote(item)
			if !ok {
				return nil, &UnmarshalError{Offset: d.readIndex(), Type: reflect.TypeFor[any](), Message: fmt.Sprintf("error unquoting %s", item)}
			}
		}

		// Read : before value.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode != scanObjectKey {
			return nil, &UnmarshalError{Offset: d.readIndex(), Type: reflect.TypeFor[map[string]any](), Message: fmt.Sprintf("expected object key, got: %c", d.data[d.readIndex()])}
		}

		//if depth decreased then value is empty (==nil) and end of object reached
		depth = len(d.scan.states)
		d.scanWhile(scanSkipSpace)
		if len(d.scan.states) < depth {
			m[key] = nil
			break
		}

		// Read value.
		value, err := d.valueInterface()
		if err != nil {
			return nil, err
		}
		m[key] = value

		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndObject || d.opcode == scanEnd || len(d.scan.states) < depth {
			break
		}
	}
	return m, nil
}

// literalInterface consumes and returns a literal from d.data[d.off-1:] and
// it reads the following byte ahead. The first byte of the literal has been
// read already (that's how the caller knows it's a literal).
func (d *decodeState) literalInterface() (any, error) {
	// All bytes inside literal return scanContinue op code.
	start := d.readIndex()
	d.rescanLiteral()

	item := d.data[start:d.readIndex()]

	switch c := item[0]; c {
	case '"', '\'': // string
		s, ok := unquote(item)
		if !ok {
			return nil, fmt.Errorf("cannot unquote string %s", item)
		}
		return s, nil
	case '*':
		v, found := d.anchors[string(item[1:])]
		if !found {
			return nil, fmt.Errorf("anchor %q not found", string(item))
		}
		return v.Interface(), nil
	case '|', '>':
		return string(d.parseMultiline(item)), nil
	default:
		item = bytes.TrimRight(item, " ") //those spaces everywhere...
		if unicode.IsLetter(rune(c)) {
			return string(item), nil
		}
		n, err := d.convertNumber(string(item))
		if err != nil {
			//return string representation
			return string(item), nil
		}
		return n, nil
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
	//replace '' with single ' inside string
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

// unquoteBytes unquotes a byte slice.
// If already unquoted, returns original value with trimmed spaces
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
			// return s, true //if already unquoted x)
			return bytes.TrimSpace(s), true
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
			case 'a':
				r++
				rr := getu4([]byte{'\\', 'u', '0', '0', '0', '7'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
			case 'b':
				b[w] = '\b'
				r++
				w++
			case 'e':
				r++
				rr := getu4([]byte{'\\', 'u', '0', '0', '1', 'b'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
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
			case '\r':
				// \r or \r\n preceded by '\\' means line continuation
				r++
				if r < len(s) && s[r] == '\n' {
					r++
				}
			case '\n':
				r++ // lone \n preceded by '\\' means line continuation
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
			case 'v':
				r++
				rr := getu4([]byte{'\\', 'u', '0', '0', '0', 'b'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
			case '0':
				r++
				rr := getu4([]byte{'\\', 'u', '0', '0', '0', '0'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
			case ' ':
				b[w] = ' '
				r++
				w++
			case '_':
				r++
				rr := getu4([]byte{'\\', 'u', '0', '0', 'a', '0'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
			case 'N':
				r++
				rr := getu4([]byte{'\\', 'u', '0', '0', '8', '5'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
			case 'L':
				r++
				rr := getu4([]byte{'\\', 'u', '2', '0', '2', '8'})
				if rr < 0 {
					return
				}
				w += utf8.EncodeRune(b[w:], rr)
			case 'P':
				r++
				rr := getu4([]byte{'\\', 'u', '2', '0', '2', '9'})
				if rr < 0 {
					return
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

// This is a subset of the formats allowed by the regular expression
// defined at http://yaml.org/type/timestamp.html.
var allowedTimestampFormats = []string{
	"2006-1-2T15:4:5.999999999Z07:00", // RCF3339Nano with short date fields.
	"2006-1-2t15:4:5.999999999Z07:00", // RFC3339Nano with short date fields and lower-case "t".
	"2006-1-2 15:4:5.999999999",       // space separated with no time zone
	"2006-1-2",                        // date only
}
