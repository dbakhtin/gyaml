// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Represents JSON data structure using native Go types: booleans, floats,
// strings, arrays, and maps.

//go:build !goexperiment.jsonv2

package gyaml

import (
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
// func Unmarshal(data []byte, v interface{}) error {
// 	return UnmarshalWithOptions(data, v, DefaultDecoderOptions())
// }
//
// // UnmarshalWithOptions decodes with DecodeOptions the first document found within the in byte slice
// // and assigns decoded values into the out value.
// func UnmarshalWithOptions(data []byte, v interface{}, opts DecoderOptions) error {
// 	// Check for well-formedness.
// 	// Avoids filling out half a data structure
// 	// before discovering a YAML syntax error.
// 	var d decodeState
// 	err := checkValid(data, &d.scan)
// 	if err != nil {
// 		return err
// 	}
//
// 	d.init(data)
// 	return d.unmarshal(v)
// }
//
// // Unmarshaler is the interface implemented by types
// // that can unmarshal a JSON description of themselves.
// // The input can be assumed to be a valid encoding of
// // a JSON value. UnmarshalJSON must copy the JSON data
// // if it wishes to retain the data after returning.
// type Unmarshaler interface {
// 	UnmarshalJSON([]byte) error
// }
//
// // An UnmarshalTypeError describes a JSON value that was
// // not appropriate for a value of a specific Go type.
// type UnmarshalTypeError struct {
// 	Value  string       // description of JSON value - "bool", "array", "number -5"
// 	Type   reflect.Type // type of Go value it could not be assigned to
// 	Offset int64        // error occurred after reading Offset bytes
// 	Struct string       // name of the struct type containing the field
// 	Field  string       // the full path from root node to the field, include embedded struct
// }
//
// func (e *UnmarshalTypeError) Error() string {
// 	if e.Struct != "" || e.Field != "" {
// 		return "json: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
// 	}
// 	return "json: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
// }
//
// // An UnmarshalFieldError describes a JSON object key that
// // led to an unexported (and therefore unwritable) struct field.
// //
// // Deprecated: No longer used; kept for compatibility.
// type UnmarshalFieldError struct {
// 	Key   string
// 	Type  reflect.Type
// 	Field reflect.StructField
// }
//
// func (e *UnmarshalFieldError) Error() string {
// 	return "json: cannot unmarshal object key " + strconv.Quote(e.Key) + " into unexported field " + e.Field.Name + " of type " + e.Type.String()
// }
//
// // An InvalidUnmarshalError describes an invalid argument passed to [Unmarshal].
// // (The argument to [Unmarshal] must be a non-nil pointer.)
// type InvalidUnmarshalError struct {
// 	Type reflect.Type
// }
//
// func (e *InvalidUnmarshalError) Error() string {
// 	if e.Type == nil {
// 		return "json: Unmarshal(nil)"
// 	}
//
// 	if e.Type.Kind() != reflect.Pointer {
// 		return "json: Unmarshal(non-pointer " + e.Type.String() + ")"
// 	}
// 	return "json: Unmarshal(nil " + e.Type.String() + ")"
// }
//
// func (d *decodeState) unmarshal(v any) error {
// 	rv := reflect.ValueOf(v)
// 	if rv.Kind() != reflect.Pointer || rv.IsNil() {
// 		return &InvalidUnmarshalError{reflect.TypeOf(v)}
// 	}
//
// 	d.scan.reset()
// 	d.scanWhile(scanSkipSpace)
// 	// We decode rv not rv.Elem because the Unmarshaler interface
// 	// test must be applied at the top level of the value.
// 	err := d.value(rv)
// 	if err != nil {
// 		return d.addErrorContext(err)
// 	}
// 	return d.savedError
// }
//
// // A Number represents a JSON number literal.
// type Number string
//
// // String returns the literal text of the number.
// func (n Number) String() string { return string(n) }
//
// // Float64 returns the number as a float64.
// func (n Number) Float64() (float64, error) {
// 	return strconv.ParseFloat(string(n), 64)
// }
//
// // Int64 returns the number as an int64.
// func (n Number) Int64() (int64, error) {
// 	return strconv.ParseInt(string(n), 10, 64)
// }
//
// // An errorContext provides context for type errors during decoding.
// type errorContext struct {
// 	Struct     reflect.Type
// 	FieldStack []string
// }
//
// // An errorContext provides context for type errors during decoding.
// type errorContext struct {
// 	Struct     reflect.Type
// 	FieldStack []string
// }
//
// // decodeState represents the state while decoding a JSON value.
// type decodeState struct {
// 	data                  []byte
// 	off                   int // next read offset in data
// 	opcode                int // last read result
// 	scan                  scanner
// 	errorContext          *errorContext
// 	savedError            error
// 	useNumber             bool
// 	disallowUnknownFields bool
// }
