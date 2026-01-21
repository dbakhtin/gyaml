// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2

package gyaml

import (
	"errors"
	"io"
)

//TODO: Decoder

// TODO: make private?
// An Encoder writes JSON values to an output stream.
type Encoder struct {
	w   io.Writer
	err error

	options encoderOptions
	//documentWritten indicates if atleas one document has been written by Encode. Each document must be split by "---\n"
	documentWritten bool
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, options: defaultEncoderOptions()}
}

func (e *Encoder) WithOptions(opts *encoderOptions) *Encoder {
	if opts != nil {
		e.options = *opts
	}
	return e
}

// UseSingleQuote determines if single or double quotes should be preferred for strings.
func (e *Encoder) WithSingleQuote(yes bool) *Encoder {
	e.options.singleQuote = yes
	return e
}

// Flow style for sequences
func (e *Encoder) WithFlowStyle(yes bool) *Encoder {
	e.options.isFlowStyle = yes
	return e
}

// WIthJSONStyle uses json style for encoding
func (e *Encoder) WithJSONStyle(yes bool) *Encoder {
	e.options.isJSONStyle = yes
	return e
}

// OmitZero forces the encoder to assume an `omitzero` struct tag is
// set on all the fields. See `Marshal` commentary for the `omitzero` tag logic.
func (e *Encoder) WithOmitZero(yes bool) *Encoder {
	e.options.omitZero = yes
	return e
}

// OmitEmpty behaves in the same way as the interpretation of the omitempty tag in the encoding/json library.
// set on all the fields.
// In the current implementation, the omitempty tag is not implemented in the same way as encoding/json,
// so please specify this option if you expect the same behavior.
func (e *Encoder) WithOmitEmpty(yes bool) *Encoder {
	e.options.omitEmpty = yes
	return e
}

// WithAutoInt automatically converts floating-point numbers to integers when the fractional part is zero.
// For example, a value of 1.0 will be encoded as 1.
func (e *Encoder) WithAutoInt(yes bool) *Encoder {
	e.options.autoInt = yes
	return e
}

// WithLiteralMultilineStyle causes encoding multiline strings with a literal syntax,
// no matter what characters they include
func (e *Encoder) WithLiteralMultilineStyle(yes bool) *Encoder {
	e.options.useLiteralStyleIfMultiline = yes
	return e
}

// WithIndent changes default indent
func (e *Encoder) WithIndent(size int) *Encoder {
	if size < 2 {
		e.err = errors.New("wrong indent size option")
	}
	e.options.indentNum = size
	return e
}

// WithIndentSequence causes sequence values to be indented the same value as Indent
func (e *Encoder) WithIndentSequence(yes bool) *Encoder {
	e.options.indentSequence = yes
	return e
}

func (e *Encoder) clearAnchorCache() {
	clear(e.options.anchors)
	clear(e.options.anchorNames)
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

	//TODO: indent is a crucial part of yaml format, so move indents to e.marshal

	// Terminate each value with a newline.
	// This makes the output look a little nicer
	// when debugging, and some kind of space
	// is required if the encoded value was a number,
	// so that the reader knows there aren't more
	// digits coming.
	e.WriteByte('\n')

	enc.clearAnchorCache()

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
