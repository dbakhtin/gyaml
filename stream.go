// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2

package gyaml

import (
	"errors"
	"io"
)

// // A Decoder reads and decodes JSON values from an input stream.
// type Decoder struct {
// 	r       io.Reader
// 	buf     []byte
// 	d       decodeState
// 	scanp   int   // start of unread data in buf
// 	scanned int64 // amount of data already scanned
// 	scan    scanner
// 	err     error
//
// 	tokenState int
// 	tokenStack []int
//
// 	opts DecoderOptions
// }
//
// // NewDecoder returns a new decoder that reads from r.
// //
// // The decoder introduces its own buffering and may
// // read data from r beyond the JSON values requested.
// func NewDecoder(r io.Reader) *Decoder {
// 	return &Decoder{r: r}
// }

// An Encoder writes YAML values to an output stream.
type Encoder struct {
	w   io.Writer
	err error

	options EncoderOptions
	//documentWritten tells if atleast one document has been written by Encode.
	//Each document must be split by a "---\n"
	documentWritten bool
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, options: DefaultEncoderOptions()}
}

func (e *Encoder) WithOptions(opts EncoderOptions) *Encoder {
	if opts.IndentSize < 2 {
		e.err = errors.New("wrong indent size option")
	}
	e.options = opts
	return e
}

// UseSingleQuote determines if single or double quotes should be preferred for strings.
func (e *Encoder) WithSingleQuote(yes bool) *Encoder {
	e.options.SingleQuote = yes
	return e
}

// Flow style for sequences
func (e *Encoder) WithFlowStyle(yes bool) *Encoder {
	e.options.FlowStyle = yes
	return e
}

// WIthJSONStyle uses json style for encoding
func (e *Encoder) WithJSONStyle(yes bool) *Encoder {
	e.options.JSONStyle = yes
	return e
}

// OmitZero forces the encoder to assume an `omitzero` struct tag is
// set on all the fields. See `Marshal` commentary for the `omitzero` tag logic.
func (e *Encoder) WithOmitZero(yes bool) *Encoder {
	e.options.OmitZero = yes
	return e
}

// OmitEmpty behaves in the same way as the interpretation of the omitempty tag in the encoding/json library.
// set on all the fields.
// In the current implementation, the omitempty tag is not implemented in the same way as encoding/json,
// so please specify this option if you expect the same behavior.
func (e *Encoder) WithOmitEmpty(yes bool) *Encoder {
	e.options.OmitEmpty = yes
	return e
}

// WithAutoInt automatically converts floating-point numbers to integers when the fractional part is zero.
// For example, a value of 1.0 will be encoded as 1.
func (e *Encoder) WithAutoInt(yes bool) *Encoder {
	e.options.AutoInt = yes
	return e
}

// WithLiteralMultilineStyle causes encoding multiline strings with a literal syntax,
// no matter what characters they include
func (e *Encoder) WithLiteralMultilineStyle(yes bool) *Encoder {
	e.options.LiteralStyleMultiline = yes
	return e
}

// WithIndent changes default indent
func (e *Encoder) WithIndent(size int) *Encoder {
	if size < 2 {
		e.err = errors.New("wrong indent size option")
	}
	e.options.IndentSize = size
	return e
}

// WithIndentSequence causes sequence values to be indented the same value as Indent
func (e *Encoder) WithIndentSequence(yes bool) *Encoder {
	e.options.IndentSequence = yes
	return e
}

// Encode writes the Yaml encoding of v to the stream,
// followed by a newline character.
//
// If multiple items are encoded to the stream,
// the second and subsequent document will be preceded with a "---" document separator,
// but the first will not.
//
// See the documentation for [Marshal] for details about the
// conversion of Go values to Yaml.
func (enc *Encoder) Encode(v any) error {
	if enc.err != nil {
		return enc.err
	}

	e := newEncodeState()
	defer encodeStatePool.Put(e)

	err := e.marshal(v, enc.options)
	if err != nil {
		return err
	}

	// Terminate each value with a newline.
	e.WriteByte('\n')

	if !enc.documentWritten {
		enc.documentWritten = true
	} else {
		enc.w.Write([]byte("---\n"))
	}

	b := e.Bytes()
	if len(b) != 0 && b[0] == '\n' {
		b = b[1:]
	}
	if _, err = enc.w.Write(b); err != nil {
		enc.err = err
	}

	return err
}
