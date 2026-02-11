// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2

package gyaml

// JSON value parser state machine.
// Just about at the limit of what is reasonable to write by hand.
// Some parts are a bit tedious, but overall it nicely factors out the
// otherwise common code from the multiple scanning functions
// in this package (Compact, Indent, checkValid, etc).
//
// This file starts with two simple examples using the scanner
// before diving into the scanner itself.

import (
	"strconv"
	"sync"
	"unicode"
)

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	scan := newScanner()
	defer freeScanner(scan)
	return checkValid(data, scan) == nil
}

// TODO: remove after debug done
var debu []byte

// checkValid verifies that data is valid JSON-encoded data.
// scan is passed in for use by checkValid to avoid an allocation.
// checkValid returns nil or a SyntaxError.
func checkValid(data []byte, s *scanner) error {
	debu = []byte{}
	s.reset()
	for _, c := range data {
		debu = append(debu, c)
		s.bytes++
		if s.step(s, c) == scanError {
			return s.err
		}
		if !isWhiteSpace(c) {
			s.lastc = c
		}
	}
	if s.eof() == scanError {
		return s.err
	}
	_ = debu
	return nil
}

// A SyntaxError is a description of a JSON syntax error.
// [Unmarshal] will return a SyntaxError if the JSON can't be parsed.
type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// A scanner is a JSON scanning state machine.
// Callers call scan.reset and then pass bytes in one at a time
// by calling scan.step(&scan, c) for each byte.
// The return value, referred to as an opcode, tells the
// caller about significant parsing events like beginning
// and ending literals, objects, and arrays, so that the
// caller can follow along if it wishes.
// The return value scanEnd indicates that a single top-level
// JSON value has been completed, *before* the byte that
// just got passed in.  (The indication must be delayed in order
// to recognize the end of numbers: is 123 a whole value or
// the beginning of 12345e+6?).
type scanner struct {
	// The step is a func to be called to execute the next transition.
	// Also tried using an integer constant and a single func
	// with a switch, but using the func directly was 10% faster
	// on a 64-bit Mac Mini, and it's nicer to read.
	step func(*scanner, byte) int

	// Reached end of top-level value.
	endTop bool

	// Stack of what we're in the middle of - array values, object keys, object values.
	states []int
	//indents per nesting state
	indents []int
	//calculated indent for the current state
	idn int
	//last meaningful (non-whitespace) value of byte input. Though it goes against strict state machine definition it
	//may help in some border cases. For example to distinguish ':' in "a: b" (object key + value) and "a:b" (unquoted string)
	lastc byte

	// Error that happened, if any.
	err error

	// total bytes consumed, updated by decoder.Decode (and deliberately
	// not set to zero by scan.reset)
	bytes int64
}

var scannerPool = sync.Pool{
	New: func() any {
		return &scanner{}
	},
}

func newScanner() *scanner {
	scan := scannerPool.Get().(*scanner)
	// scan.reset by design doesn't set bytes to zero
	scan.bytes = 0
	scan.reset()
	return scan
}

func freeScanner(scan *scanner) {
	// Avoid hanging on to too much memory in extreme cases.
	if len(scan.states) > 1024 {
		scan.states = nil
		scan.indents = nil
	}
	scannerPool.Put(scan)
}

// These values are returned by the state transition functions
// assigned to scanner.state and the method scanner.eof.
// They give details about the current state of the scan that
// callers might be interested to know about.
// It is okay to ignore the return value of any particular
// call to scanner.state: if one call returns scanError,
// every subsequent call will return scanError too.
const (
	// Continue.
	scanContinue     = iota // uninteresting byte
	scanBeginLiteral        // end implied by next result != scanContinue
	scanBeginObject         // begin object
	scanObjectKey           // just finished object key (string)
	scanObjectValue         // just finished non-last object value
	scanEndObject           // end object (implies scanObjectValue if possible)
	scanBeginArray          // begin array
	scanArrayValue          // just finished array value
	scanEndArray            // end array (implies scanArrayValue if possible)
	scanSkipSpace           // space byte; can skip; known to be last "continue" result

	// Stop.
	scanEnd   // top-level value ended *before* this byte; known to be first "stop" result
	scanError // hit an error, scanner.err.
)

// These values are stored in the parseState stack.
// They give the current state of a composite value
// being scanned. If the parser is inside a nested value
// the parseState describes the nested state, outermost at entry 0.
const (
	parseObjectKey = iota // parsing object key (before colon)
	// parseObjectValueEmpty        // parsing object value (after colon) but value is empty yet
	parseObjectValue    // parsing object value (after colon)
	parseArrayValue     // parsing array value
	parseMultiLineValue //parsing multi-line string value (starts with '|' followed by a line-break)
)

// This limits the max nesting depth to prevent stack overflow.
// This is permitted by https://tools.ietf.org/html/rfc7159#section-9
const maxNestingDepth = 10000

// reset prepares the scanner for use.
// It must be called before calling s.step.
func (s *scanner) reset() {
	s.step = stateBeginLine
	s.states = s.states[0:0]
	s.indents = s.indents[0:0]
	s.idn = 0
	s.err = nil
	s.endTop = false
}

// eof tells the scanner that the end of input has been reached.
// It returns a scan status just as s.step does.
func (s *scanner) eof() int {
	if s.err != nil {
		return scanError
	}
	if s.endTop {
		return scanEnd
	}

	s.step(s, '\n')
	if s.endTop || s.err == nil {
		return scanEnd
	}

	return scanError
}

// pushState pushes a new parse state newState onto the parse stack.
// an error state is returned if maxNestingDepth was exceeded, otherwise successState is returned.
func (s *scanner) pushState(c byte, newState int, successState int) int {
	s.states = append(s.states, newState)
	if newState == parseMultiLineValue {
		s.indents = append(s.indents, s.idn+1) //multilines should have a bigger indent compared to parent
	} else {
		s.indents = append(s.indents, s.idn)
	}
	if len(s.states) <= maxNestingDepth {
		return successState
	}
	return s.error(c, "exceeded max depth")
}

// popState pops a parse state (already obtained) off the stack
// and updates s.step accordingly.
func (s *scanner) popState() {
	n := len(s.states) - 1
	s.states = s.states[0:n]
	s.indents = s.indents[0:n]
	if n == 0 {
		s.step = stateEndTop
		s.endTop = true
	} else {
		s.step = stateEndValue
	}
}

// unlike json need to track line breaks separately
func isSpace(c byte) bool {
	//return c <= ' ' && (c == ' ' || c == '\t')
	return c == ' '
}

func isLineBreak(c byte) bool {
	return c <= ' ' && (c == '\r' || c == '\n')
}

func isWhiteSpace(c byte) bool {
	return c <= ' ' && (c == ' ' || c == '\t' || c == '\r' || c == '\n')
}

// indentDiff returns the length difference of current & last indents
func indentDiff(s *scanner) int {
	lastIndent := 0
	if len(s.indents) > 0 {
		lastIndent = s.indents[len(s.indents)-1]
	}
	_ = lastIndent
	// return s.curindent - s.indents[0]
	return s.idn - lastIndent
}

// stateBeginLine is the state after reading `\n` or at the beginning.
func stateBeginLine(s *scanner, c byte) int {
	if isLineBreak(c) {
		s.idn = 0
		return scanSkipSpace
	}
	if isSpace(c) {
		s.idn++
		return scanSkipSpace
	}
	n := len(s.states)
	//if in nested state
	idiff := indentDiff(s)
	if n > 0 {
		// parseObjectKey & parseObjectValue are swapped each time in stateEndValue until indents change
		switch s.states[n-1] {
		case parseObjectKey:
			switch {
			case idiff < 0:
				//TODO: optimize this
				s.popState()
				n--
				if n > 0 && s.states[n-1] == parseObjectValue {
					s.states[n-1] = parseObjectKey
				}
				return stateBeginLine(s, c)
			case idiff > 0:
				return s.error(c, "unexpected increased indent when parsing object key")
			}
		case parseObjectValue:
			// check if not in a multiline string?
			switch {
			case idiff < 0:
				// error now but test later may be just pop?
				// s.popParseState()
				// return stateBeginLine(s, c)
				// should i call endValue?? xD
				s.popState()
				return stateBeginLine(s, c)
			// return s.error(c, "unexpected decrement of indent")
			case idiff == 0:
				//either array or object value was empty and this is the beginning of another key
				if c == '-' {
					s.step = stateBeginArrayValueS
					return scanContinue
				}
				s.states[n-1] = parseObjectKey
			}
		case parseArrayValue:
			// 2 means possible "- "
			switch {
			case idiff < -2:
				//pop & check if inside object. If true swap object state to parseKey, because this array was a value
				//already. Helps with catching border cases.
				s.popState()
				//TODO: move to transitionState method? Also can use it probably in EndValue
				n--
				if n > 0 && s.states[n-1] == parseObjectValue {
					s.states[n-1] = parseObjectKey
				}
				return stateBeginLine(s, c)
			case idiff == -2:
				return stateBeginArrayValue(s, c)
			default:
				//TODO: check if can pop state and (may be) there is an object in stack above this
				return s.error(c, "unexpected end of array")
			}
		case parseMultiLineValue:
			switch {
			case idiff < 0:
				//pop
				s.popState()
				n--
				if n > 0 && s.states[n-1] == parseObjectValue {
					s.states[n-1] = parseObjectKey
				}
				return stateEndValue(s, c)
			}
		}
	}
	return stateBeginValue(s, c)
}

// stateEndLine is a state when c == '\n' in a non-blank line
func stateEndLine(s *scanner, _ byte) int {
	s.idn = 0
	s.step = stateBeginLine
	//either return or set step
	return scanContinue
}

// stateBeginValueOrEmpty is the state when any token or space is expected
func stateBeginValueOrEmpty(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}
	if c == ']' {
		return stateEndValue(s, c)
	}
	if c == '}' {
		n := len(s.states)
		s.states[n-1] = parseObjectValue
		return stateEndValue(s, c)
	}
	return stateBeginValue(s, c)
}

// stateBeginValue is the state at the beginning of any token
func stateBeginValue(s *scanner, c byte) int {
	switch c {
	case '\n':
		return stateEndLine(s, c)
	case '{':
		s.step = stateBeginValueOrEmpty
		return s.pushState(c, parseObjectKey, scanBeginObject)
	case '[':
		s.step = stateBeginValueOrEmpty
		return pushArrayState(s, c)
		// return s.pushState(c, parseArrayValue, scanBeginArray)
	case '"':
		s.step = stateInString
		return scanBeginLiteral
	case '-':
		s.step = stateHyphen
		return scanBeginLiteral
	case '0': // beginning of 0.123 or 0x1f
		s.step = state0Begin
		return scanBeginLiteral
	case '&', '*': // beginning of anchor or alias
		s.step = stateInStringUnq
		return scanBeginLiteral
	case '<': // beginning of anchor or alias
		s.step = stateBeginMerge
		return scanBeginLiteral
	case '|', '>':
		s.step = stateBeginMultilineChomp
		return s.pushState(c, parseMultiLineValue, scanBeginLiteral)
	}
	if '1' <= c && c <= '9' { // beginning of 1234.5
		s.step = state1
		return scanBeginLiteral
	}
	if unicode.IsLetter(rune(c)) {
		// if len(s.states) > 0 && s.states[len(s.states)-1] == parseMultiLineValue
		s.step = stateInStringUnq
		return scanBeginLiteral
	}
	return s.error(c, "looking for beginning of value")
}

// stateEndValue is the state after completing a value,
// such as after reading `{}` or `true` or `["x"`.
func stateEndValue(s *scanner, c byte) int {
	n := len(s.states)
	//TODO: optimize? or can move somewhere?
	if c == ':' {
		if n == 0 || s.states[n-1] == parseObjectValue && indentDiff(s) > 0 {
			s.pushState(c, parseObjectKey, scanObjectKey)
			n++
		}
	}
	if n == 0 {
		// Completed top-level before the current byte.
		s.step = stateEndTop
		s.endTop = true
		return stateEndTop(s, c)
	}
	// if isSpace(c) {
	// 	s.step = stateEndValue
	// 	return scanSkipSpace
	// }

	ps := s.states[n-1]
	switch ps {
	case parseObjectKey:
		//if key is unquoted string, then check space in ": ", otherwise check ":"
		if s.lastc == ':' && isWhiteSpace(c) || c == ':' {
			s.states[n-1] = parseObjectValue
			s.step = stateBeginValueOrEmpty
			return scanObjectKey
		}
		if c == '{' || c == '[' {
			s.states[n-1] = parseObjectValue
			return stateBeginValueOrEmpty(s, c)
		}
		return s.error(c, "after object key")
	case parseObjectValue:
		if isLineBreak(c) {
			s.states[n-1] = parseObjectKey
			return stateEndLine(s, c)
		}
		//TODO: need a flag to track the flow-style state in s???
		if c == ',' {
			s.states[n-1] = parseObjectKey
			s.step = stateBeginValueOrEmpty
			return scanObjectValue
		}
		if c == '}' {
			s.popState()
			return scanEndObject
		}
		return s.error(c, "after object key:value pair")
	case parseArrayValue:
		if isLineBreak(c) {
			return stateEndLine(s, c)
		}
		//TODO: need a flag to track the flow-style state in s???
		if c == ',' {
			s.step = stateBeginValueOrEmpty
			return scanArrayValue
		}
		if c == ']' {
			s.popState()
			return scanEndArray
		}
		return s.error(c, "after array element")
	case parseMultiLineValue:
		if isLineBreak(c) {
			// s.step = stateBeginLine
			// return scanContinue
			return stateEndLine(s, c)
		}
		return s.error(c, "in a multiline string")
	}
	return s.error(c, "")
}

// stateEndTop is the state after finishing the top-level value,
// such as after reading `{}` or `[1,2,3]`.
// Only space characters should be seen now (or the end of document)
func stateEndTop(s *scanner, c byte) int {
	if isSpace(c) {
		s.idn++
		return scanEnd
	}
	if isLineBreak(c) {
		s.idn = 0
		return scanEnd
	}
	//check for document separator
	if c == '-' && s.idn == 0 {
		s.step = stateEndDoc1
		return scanContinue
	}
	s.error(c, "after top-level value")
	return scanEnd
}

// stateKeyOrUnq is the state after reading ':' in an unquoted string
func stateKeyOrUnq(s *scanner, c byte) int {
	n := len(s.states)
	if isSpace(c) {
		// if nested object key
		idiff := indentDiff(s)
		if n == 0 || //if top level object key
			s.states[n-1] == parseObjectValue && idiff > 0 || // if nested object key
			s.states[n-1] == parseArrayValue && idiff >= 0 {
			s.pushState(c, parseObjectKey, scanObjectKey)
		}
		return stateEndValue(s, c)
	}
	switch c {
	case '{', '}', '[', ']', ',':
		return stateEndValue(s, c)
	}
	//TODO: optimize? the idea was to skip stateEndValue but probably it was wrong
	if s.lastc == ':' && isLineBreak(c) {
		s.pushState(c, parseObjectValue, scanObjectValue)
		return stateEndLine(s, c)
	}
	//TODO: check c is printable or a valid string char?
	return stateInStringUnq(s, c)
}

// stateBeginMerge is the state after '<' denoting merge instruction "<<:"
func stateBeginMerge(s *scanner, c byte) int {
	if c == '<' {
		s.step = stateMerge1
		return scanContinue
	}
	return s.error(c, "in merge instruction")
}

// stateMerge1 is the state after "<<" denoting merge instruction "<<:"
func stateMerge1(s *scanner, c byte) int {
	if isSpace(c) {
		return scanContinue
	}
	if c == ':' {
		s.step = stateKeyOrUnq
		return scanContinue
	}
	return s.error(c, "in merge instruction")
}

// stateBeginMultilineChomp is the state after '|' or '>' multiline literal indicator
func stateBeginMultilineChomp(s *scanner, c byte) int {
	if c == '+' || c == '-' {
		s.step = stateBeginMultilineS
		return scanContinue
	}
	return stateBeginMultilineS(s, c)
}

// stateBeginMultilineS is the state after '|' or '>' with optional chomp
func stateBeginMultilineS(s *scanner, c byte) int {
	if isSpace(c) {
		return scanContinue
	}
	if isLineBreak(c) {
		s.step = stateBeginLine
		return scanContinue
	}
	return s.error(c, "at the beginning of multiline string")
}

// // stateBeginMultiline is the state after '|' or '>' followed by a linebreak
// func stateBeginMultiline(s *scanner, c byte) int {
// 	if isSpace(c) {
// 		return scanContinue
// 	}
// 	if isLineBreak(c) {
// 		s.step = stateBeginLine
// 		return scanContinue
// 	}
// 	return s.error(c, "at the beginning of multiline string")
// }

// stateInStringUnq is the state after reading f,t (and not fa, tr following
func stateInStringUnq(s *scanner, c byte) int {
	if isLineBreak(c) {
		return stateEndValue(s, c)
	}
	// Move to stateEndValue?
	if c == ':' {
		s.step = stateKeyOrUnq
		return scanContinue
	}

	switch c {
	case '}', ']', ',':
		return stateEndValue(s, c)
	}

	if c == '\\' {
		s.step = stateInStringEsc
		return scanContinue
	}
	if c < 0x20 {
		return s.error(c, "in unquoted string literal")
	}
	return scanContinue
}

// stateInString is the state after reading `"`.
func stateInString(s *scanner, c byte) int {
	if c == '"' {
		s.step = stateEndValue
		return scanContinue
	}
	if c == '\\' {
		s.step = stateInStringEsc
		return scanContinue
	}
	if c < 0x20 {
		return s.error(c, "in string literal")
	}
	return scanContinue
}

// stateInStringEsc is the state after reading `"\` during a quoted string.
func stateInStringEsc(s *scanner, c byte) int {
	switch c {
	case 'b', 'f', 'n', 'r', 't', '\\', '/', '"':
		s.step = stateInString
		return scanContinue
	case 'u':
		s.step = stateInStringEscU
		return scanContinue
	}
	return s.error(c, "in string escape code")
}

// stateInStringEscU is the state after reading `"\u` during a quoted string.
func stateInStringEscU(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU1
		return scanContinue
	}
	// numbers
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscU1 is the state after reading `"\u1` during a quoted string.
func stateInStringEscU1(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU12
		return scanContinue
	}
	// numbers
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscU12 is the state after reading `"\u12` during a quoted string.
func stateInStringEscU12(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU123
		return scanContinue
	}
	// numbers
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscU123 is the state after reading `"\u123` during a quoted string.
func stateInStringEscU123(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInString
		return scanContinue
	}
	// numbers
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateEndDoc1 is the state after '-' on a new line when document separator "---\n" is expected
func stateEndDoc1(s *scanner, c byte) int {
	if c == '-' {
		s.step = stateEndDoc2
		return scanContinue
	}
	// return stateBeginArrayValueS(s, c)
	return s.error(c, "in document separator")
}

// stateEndDoc2 is the state after "--" on a new line
func stateEndDoc2(s *scanner, c byte) int {
	if c == '-' {
		s.step = stateEndDoc3
		return scanContinue
	}
	return s.error(c, "in document separator")
}

// stateEndDoc3 is the state after "---" on a new line
func stateEndDoc3(s *scanner, c byte) int {
	if isLineBreak(c) {
		//force check of the last document for correctness
		s.step = stateBeginLine
		if s.eof() == scanError {
			return scanError
		}
		s.reset()
		return scanContinue
	}
	return s.error(c, "in document separator")
}

func pushArrayState(s *scanner, c byte) int {
	n := len(s.states)
	s.idn += 2 //"- "
	idiff := indentDiff(s)
	// if top level first element or nested inside object or array of arrays
	if n == 0 ||
		idiff > 0 &&
			(s.states[n-1] == parseArrayValue || s.states[n-1] == parseObjectValue) {
		s.pushState(c, parseArrayValue, scanArrayValue)
		s.step = stateBeginValueOrEmpty
		return scanContinue
	}
	//already in array
	if n > 0 && s.states[n-1] == parseArrayValue && idiff == 0 {
		s.step = stateBeginValueOrEmpty
		return scanContinue
	}
	return s.error(c, "in push array state")
}

// stateBeginArrayValue is the state expecting '-' in "- "
func stateBeginArrayValue(s *scanner, c byte) int {
	if c == '-' {
		s.step = stateBeginArrayValueS
		return scanContinue
	}
	return s.error(c, "in array value prefix")
}

// stateBeginArrayValueS is the state expecting ' ' in "- "
func stateBeginArrayValueS(s *scanner, c byte) int {
	if c == ' ' {
		return pushArrayState(s, c)
	}
	if c == '-' && s.idn == 0 {
		s.step = stateEndDoc2
		return scanContinue
	}
	return s.error(c, "in array value prefix")
}

// stateHyphen is the state after reading `-` during a number or array element
func stateHyphen(s *scanner, c byte) int {
	switch {
	case c == '0':
		s.step = state0Begin
		return scanContinue
	case '1' <= c && c <= '9':
		s.step = state1
		return scanContinue
	case c == ' ':
		return pushArrayState(s, c)
	case c == '-' && s.idn == 0:
		s.step = stateEndDoc2
		return scanContinue
	default:
		return s.error(c, "in numeric literal")
	}
}

// state1 is the state after reading a non-zero integer during a number,
// such as after reading `1` or `100` but not `0`.
func state1(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		s.step = state1
		return scanContinue
	}
	return state0(s, c)
}

// state0 is the state after reading `0` during a number.
func state0(s *scanner, c byte) int {
	if c == '.' {
		s.step = stateDot
		return scanContinue
	}
	if c == 'e' || c == 'E' {
		s.step = stateE
		return scanContinue
	}
	return stateEndValue(s, c)
}

// state0Begin is the state after reading `0` at the beginning of the value. Compared to state0 it can't be triggered inside number
func state0Begin(s *scanner, c byte) int {
	switch c {
	case '.':
		s.step = stateDot
		return scanContinue
	case 'b':
		s.step = stateBin
		return scanContinue
	case 'o':
		s.step = stateOct
		return scanContinue
	case 'x':
		s.step = stateHex
		return scanContinue
	case 'e', 'E':
		s.step = stateE
		return scanContinue
	default:
		return stateEndValue(s, c)
	}
}

// stateBin is the state after reading 0b - start of binary integer
func stateBin(s *scanner, c byte) int {
	if '0' <= c && c <= '1' {
		s.step = stateBin
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateOct is the state after reading 0o - start of octal integer
func stateOct(s *scanner, c byte) int {
	if '0' <= c && c <= '7' {
		s.step = stateOct
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateHex is the state after reading 0o - start of octal integer
func stateHex(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateHex
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateDot is the state after reading the integer and decimal point in a number,
// such as after reading `1.`.
func stateDot(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		s.step = stateDot0
		return scanContinue
	}
	return s.error(c, "after decimal point in numeric literal")
}

// stateDot0 is the state after reading the integer, decimal point, and subsequent
// digits of a number, such as after reading `3.14`.
func stateDot0(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		return scanContinue
	}
	if c == 'e' || c == 'E' {
		s.step = stateE
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateE is the state after reading the mantissa and e in a number,
// such as after reading `314e` or `0.314e`.
func stateE(s *scanner, c byte) int {
	if c == '+' || c == '-' {
		s.step = stateESign
		return scanContinue
	}
	return stateESign(s, c)
}

// stateESign is the state after reading the mantissa, e, and sign in a number,
// such as after reading `314e-` or `0.314e+`.
func stateESign(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		s.step = stateE0
		return scanContinue
	}
	return s.error(c, "in exponent of numeric literal")
}

// stateE0 is the state after reading the mantissa, e, optional sign,
// and at least one digit of the exponent in a number,
// such as after reading `314e-2` or `0.314e+1` or `3.14e0`.
func stateE0(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateError is the state after reaching a syntax error,
// such as after reading `[1}` or `5.1.2`.
func stateError(s *scanner, c byte) int {
	return scanError
}

// error records an error and switches to the error state.
func (s *scanner) error(c byte, context string) int {
	s.step = stateError
	s.err = &SyntaxError{"invalid character " + quoteChar(c) + " " + context, s.bytes}
	return scanError
}

// quoteChar formats c as a quoted character literal.
func quoteChar(c byte) string {
	// special cases - different from quoted strings
	if c == '\'' {
		return `'\''`
	}
	if c == '"' {
		return `'"'`
	}

	// use quoted string with different quotation marks
	s := strconv.Quote(string(c))
	return "'" + s[1:len(s)-1] + "'"
}
