// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gyaml

import (
	"bytes"
	"errors"
	"io"
)

// A Decoder reads and decodes YAML values from an input stream.
type Decoder struct {
	r       io.Reader
	buf     []byte
	d       decodeState
	scanp   int   // start of unread data in buf
	scanned int64 // amount of data already scanned
	scan    scanner
	err     error

	tokenState int
	tokenStack []int
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the YAML values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// DisallowUnknownFields causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (dec *Decoder) DisallowUnknownFields() { dec.d.disallowUnknownFields = true }

var (
	newDocumentSeparator = []byte{'-', '-', '-'}
	endDocumentSeparator = []byte{'.', '.', '.'}
)

// documentStartIndex looks for a [newDocumentSeparator] sequence in a bytes buffer
// and returns the index of the first byte after this separator
func documentStartIndex(buf []byte) int {
	if bytes.HasPrefix(buf, newDocumentSeparator) {
		return 3
	}
	idx := bytes.Index(buf, append([]byte{'\n'}, newDocumentSeparator...))
	if idx > -1 {
		return idx + 4
	} else {
		idxr := bytes.Index(buf, append([]byte{'\r'}, newDocumentSeparator...))
		if idxr > -1 {
			return idxr + 4
		}
	}
	return 0
}

// documentEndIndex looks for a [endDocumentSeparator] sequence in a bytes buffer
// returns index of the first byte preceding it
// separator should either be the first value (then document is empty) or start from a new line
func documentEndIndex(buf []byte) int {
	idx := bytes.Index(buf, append([]byte{'\n'}, endDocumentSeparator...))
	if idx == -1 {
		if bytes.HasPrefix(buf, endDocumentSeparator) {
			//document empty
			return 0
		}
		return bytes.Index(buf, append([]byte{'\r'}, endDocumentSeparator...))
	}
	return idx
}

// Decode reads the next YAML-encoded value from its
// input and stores it in the value pointed to by v.
//
// See the documentation for [Unmarshal] for details about
// the conversion of YAML into a Go value.
func (dec *Decoder) Decode(v any) error {
	if dec.err != nil {
		return dec.err
	}

	if err := dec.tokenPrepareForDecode(); err != nil {
		return err
	}

	if !dec.tokenValueAllowed() {
		return &SyntaxError{msg: "not at beginning of value", Offset: dec.InputOffset()}
	}

	// Read whole value into buffer.
	n, err := dec.readValue()
	if err != nil {
		return err
	}
	buf := dec.buf[dec.scanp : dec.scanp+n]
	//Look for new document separator "---" starting from a new line or buffer beginning
	start := documentStartIndex(buf)
	if start > 0 {
		dec.scan.reset()
		dec.scanp = start
		n -= start
		buf = buf[start:]
		startNext := documentStartIndex(buf)
		if startNext > 0 {
			n = max(0, startNext-4)
			buf = buf[:n]
		}
	}
	//Look for a document end separator
	end := documentEndIndex(buf)
	if end > -1 {
		buf = buf[:end]
		n = end + 4 //skip "\n..." bytes
	}
	dec.d.init(buf)
	dec.scanp += n

	if len(buf) == 0 {
		return io.EOF
	}
	// Don't save err from unmarshal into dec.err:
	// the connection is still usable since we read a complete YAML
	// object from it before the error happened.
	err = dec.d.unmarshal(v)

	// fixup token streaming state
	dec.tokenValueEnd()

	return err
}

// Buffered returns a reader of the data remaining in the Decoder's
// buffer. The reader is valid until the next call to [Decoder.Decode].
func (dec *Decoder) Buffered() io.Reader {
	return bytes.NewReader(dec.buf[dec.scanp:])
}

// readValue reads a YAML value into dec.buf.
// It returns the length of the encoding.
func (dec *Decoder) readValue() (int, error) {
	dec.scan.reset()

	scanp := dec.scanp
	var err error
Input:
	// help the compiler see that scanp is never negative, so it can remove
	// some bounds checks below.
	for scanp >= 0 {

		// Look in the buffer for a new value.
		for ; scanp < len(dec.buf); scanp++ {
			c := dec.buf[scanp]
			dec.scan.bytes++
			switch dec.scan.step(&dec.scan, c) {
			case scanEnd:
				// scanEnd is delayed one byte so we decrement
				// the scanner bytes count by 1 to ensure that
				// this value is correct in the next call of Decode.
				dec.scan.bytes--
				break Input
			// case scanEndObject, scanEndArray:
			// 	// scanEnd is delayed one byte.
			// 	// We might block trying to get that byte from src,
			// 	// so instead invent a space byte.
			// 	if stateEndValue(&dec.scan, '\n') == scanEnd {
			// 		scanp++
			// 		break Input
			// 	}
			case scanError:
				dec.err = dec.scan.err
				return 0, dec.scan.err
			}
			dec.scan.lastInput(c)
		}

		// Did the last read have an error?
		// Delayed until now to allow buffer scan.
		if err != nil {
			if err == io.EOF {
				// if dec.scan.step(&dec.scan, '\n') == scanEnd {
				if dec.scan.eof() == scanEnd {
					break Input
				}
				if nonSpace(dec.buf) {
					err = io.ErrUnexpectedEOF
				}
			}
			dec.err = err
			return 0, err
		}

		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n
	}
	return scanp - dec.scanp, nil
}

func (dec *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		dec.scanned += int64(dec.scanp)
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	// Grow buffer if not large enough.
	const minRead = 512
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}

	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0 : len(dec.buf)+n]

	return err
}

// nonSpace checks is a byte slice contains a non-space character
func nonSpace(b []byte) bool {
	for _, c := range b {
		if !isSpace(c) {
			return true
		}
	}
	return false
}

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

// WithSingleQuote determines if single or double quotes should be preferred for strings.
func (e *Encoder) WithSingleQuote(yes bool) *Encoder {
	e.options.SingleQuote = yes
	return e
}

// WithFlowStyle style for sequences
func (e *Encoder) WithFlowStyle(yes bool) *Encoder {
	e.options.FlowStyle = yes
	return e
}

// WithJSONStyle uses json style for encoding
func (e *Encoder) WithJSONStyle(yes bool) *Encoder {
	e.options.JSONStyle = yes
	return e
}

// WithOmitZero forces the encoder to assume an `omitzero` struct tag is
// set on all the fields. See `Marshal` commentary for the `omitzero` tag logic.
func (e *Encoder) WithOmitZero(yes bool) *Encoder {
	e.options.OmitZero = yes
	return e
}

// WithOmitEmpty behaves in the same way as the interpretation of the omitempty tag in the encoding/json library.
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
func (e *Encoder) Encode(v any) error {
	if e.err != nil {
		return e.err
	}

	es := newEncodeState()
	defer encodeStatePool.Put(es)

	err := es.marshal(v, e.options)
	if err != nil {
		return err
	}

	// Terminate each value with a newline.
	es.WriteByte('\n')

	if !e.documentWritten {
		e.documentWritten = true
	} else {
		if _, err := e.w.Write([]byte("---\n")); err != nil {
			e.err = err
		}
	}

	b := es.Bytes()
	if len(b) != 0 && b[0] == '\n' {
		b = b[1:]
	}
	if _, err = e.w.Write(b); err != nil {
		e.err = err
	}

	return err
}

// RawMessage is a raw encoded YAML value.
// It implements [Marshaler] and [Unmarshaler] and can
// be used to delay YAML decoding or precompute a YAML encoding.
type RawMessage []byte

// MarshalYAML returns m as the YAML encoding of m.
func (m RawMessage) MarshalYAML() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalYAML sets *m to a copy of data.
func (m *RawMessage) UnmarshalYAML(data []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalYAML on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}

var _ Marshaler = (*RawMessage)(nil)
var _ Unmarshaler = (*RawMessage)(nil)

// A Token holds a value of one of these types:
//
//   - [Delim], for the four YAML delimiters [ ] { }
//   - bool, for YAML booleans
//   - float64, for YAML numbers
//   - [Number], for YAML numbers
//   - string, for YAML string literals
//   - nil, for YAML null
type Token any

const (
	tokenTopValue = iota
	tokenArrayStart
	tokenArrayValue
	tokenArrayComma
	tokenObjectStart
	tokenObjectKey
	tokenObjectColon
	tokenObjectValue
	tokenObjectComma
)

// advance tokenstate from a separator state to a value state
func (dec *Decoder) tokenPrepareForDecode() error {
	// Note: Not calling peek before switch, to avoid
	// putting peek into the standard Decode path.
	// peek is only called when using the Token API.
	switch dec.tokenState {
	case tokenArrayComma:
		c, err := dec.peek()
		if err != nil {
			return err
		}
		if c != ',' {
			return &SyntaxError{"expected comma after array element", dec.InputOffset()}
		}
		dec.scanp++
		dec.tokenState = tokenArrayValue
	case tokenObjectColon:
		c, err := dec.peek()
		if err != nil {
			return err
		}
		if c != ':' {
			return &SyntaxError{"expected colon after object key", dec.InputOffset()}
		}
		dec.scanp++
		dec.tokenState = tokenObjectValue
	}
	return nil
}

func (dec *Decoder) tokenValueAllowed() bool {
	switch dec.tokenState {
	case tokenTopValue, tokenArrayStart, tokenArrayValue, tokenObjectValue:
		return true
	}
	return false
}

func (dec *Decoder) tokenValueEnd() {
	switch dec.tokenState {
	case tokenArrayStart, tokenArrayValue:
		dec.tokenState = tokenArrayComma
	case tokenObjectValue:
		dec.tokenState = tokenObjectComma
	}
}

// A Delim is a YAML array or object delimiter, one of [ ] { or }.
type Delim rune

func (d Delim) String() string {
	return string(d)
}

// Token returns the next YAML token in the input stream.
// At the end of the input stream, Token returns nil, [io.EOF].
//
// Token guarantees that the delimiters [ ] { } it returns are
// properly nested and matched: if Token encounters an unexpected
// delimiter in the input, it will return an error.
//
// The input stream consists of basic YAML values—bool, string,
// number, and null—along with delimiters [ ] { } of type [Delim]
// to mark the start and end of arrays and objects.
// Commas and colons are elided.
func (dec *Decoder) Token() (Token, error) {
	for {
		c, err := dec.peek()
		if err != nil {
			return nil, err
		}
		switch c {
		case '[':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenArrayStart
			return Delim('['), nil

		case ']':
			if dec.tokenState != tokenArrayStart && dec.tokenState != tokenArrayComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim(']'), nil

		case '{':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenObjectStart
			return Delim('{'), nil

		case '}':
			if dec.tokenState != tokenObjectStart && dec.tokenState != tokenObjectComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim('}'), nil

		case ':':
			if dec.tokenState != tokenObjectColon {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = tokenObjectValue
			continue

		case ',':
			if dec.tokenState == tokenArrayComma {
				dec.scanp++
				dec.tokenState = tokenArrayValue
				continue
			}
			if dec.tokenState == tokenObjectComma {
				dec.scanp++
				dec.tokenState = tokenObjectKey
				continue
			}
			return dec.tokenError(c)

		case '"':
			if dec.tokenState == tokenObjectStart || dec.tokenState == tokenObjectKey {
				var x string
				old := dec.tokenState
				dec.tokenState = tokenTopValue
				err := dec.Decode(&x)
				dec.tokenState = old
				if err != nil {
					return nil, err
				}
				dec.tokenState = tokenObjectColon
				return x, nil
			}
			fallthrough

		default:
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			var x any
			if err := dec.Decode(&x); err != nil {
				return nil, err
			}
			return x, nil
		}
	}
}

func (dec *Decoder) tokenError(c byte) (Token, error) {
	var context string
	switch dec.tokenState {
	case tokenTopValue:
		context = " looking for beginning of value"
	case tokenArrayStart, tokenArrayValue, tokenObjectValue:
		context = " looking for beginning of value"
	case tokenArrayComma:
		context = " after array element"
	case tokenObjectKey:
		context = " looking for beginning of object key string"
	case tokenObjectColon:
		context = " after object key"
	case tokenObjectComma:
		context = " after object key:value pair"
	}
	return nil, &SyntaxError{"invalid character " + quoteChar(c) + context, dec.InputOffset()}
}

// More reports whether there is another element in the
// current array or object being parsed.
func (dec *Decoder) More() bool {
	c, err := dec.peek()
	return err == nil && c != ']' && c != '}'
}

func (dec *Decoder) peek() (byte, error) {
	var err error
	for {
		for i := dec.scanp; i < len(dec.buf); i++ {
			c := dec.buf[i]
			if isSpace(c) {
				continue
			}
			dec.scanp = i
			return c, nil
		}
		// buffer has been scanned, now report any error
		if err != nil {
			return 0, err
		}
		err = dec.refill()
	}
}

// InputOffset returns the input stream byte offset of the current decoder position.
// The offset gives the location of the end of the most recently returned token
// and the beginning of the next token.
func (dec *Decoder) InputOffset() int64 {
	return dec.scanned + int64(dec.scanp)
}
