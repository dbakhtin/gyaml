// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gyaml

// YAML value parser state machine.
// Just about at the limit of what is reasonable to write by hand.
// Some parts are a bit tedious, but overall it nicely factors out the
// otherwise common code from the multiple scanning functions
//
// This file starts with two simple examples using the scanner
// before diving into the scanner itself.

import (
	"fmt"
	"strconv"
	"sync"
	"unicode"
)

// Valid reports whether data is a valid YAML encoding.
func Valid(data []byte) bool {
	scan := newScanner()
	defer freeScanner(scan)
	return checkValid(data, scan) == nil
}

// checkValid verifies that data is valid YAML-encoded data.
// scan is passed in for use by checkValid to avoid an allocation.
// checkValid returns nil or a SyntaxError.
func checkValid(data []byte, s *scanner) error {
	s.reset()
	for _, c := range data {
		s.bytes++
		if s.step(s, c) == scanError {
			return s.err
		}
		s.lastInput(c)
	}
	if s.eof() == scanError {
		return s.err
	}
	return nil
}

// A SyntaxError is a description of a YAML syntax error.
// [Unmarshal] will return a SyntaxError if the YAML can't be parsed.
type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("%s, offset: %d", e.msg, e.Offset) }

// A scanner is a YAML scanning state machine.
// Callers call scan.reset and then pass bytes in one at a time
// by calling scan.step(&scan, c) for each byte.
// The return value, referred to as an opcode, tells the
// caller about significant parsing events like beginning
// and ending literals, objects, and arrays, so that the
// caller can follow along if it wishes.
// The return value scanEnd indicates that a single top-level
// YAML value has been completed, *before* the byte that
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
	//last meaningful (non-whitespace) value of byte input. Though this goes against strict state machine definition it
	//may help in some border cases. For example to distinguish ':' in "a: b" (object key + value) and "a:b" (a single unquoted string)
	lastc byte
	//sameLine is a flag, set to true after a block object key found and reset on new line
	//the only purpose atm is to catch "a: b: c" and "a: - b" error cases
	sameLine bool

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
	parseObjectKey      = iota // parsing object key (before colon)
	parseObjectValue           // parsing object value (after colon)
	parseArrayValue            // parsing array value
	parseMultiLineValue        //parsing multi-line string value (starts with '|' followed by a line-break)
	//separate states for flow style
	parseFlowObjectKey
	parseFlowObjectValue
	parseFlowArrayValue
)

// This limits the max nesting depth to prevent stack overflow.
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
	s.lastc = 0
	s.sameLine = false
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
	//if by this time stack contains any flow state then no matching ] or } were found, report error
	//unlike with full flow style or even json the stack can be non-empty at the end of the scan
	for _, state := range s.states {
		if state == parseFlowArrayValue || state == parseFlowObjectKey || state == parseFlowObjectValue {
			s.err = &SyntaxError{"unexpected end of input", s.bytes}
			return scanError
		}
	}

	s.step(s, '\n')
	if s.endTop || s.err == nil {
		return scanEnd
	}

	s.err = &SyntaxError{"unexpected end of input", s.bytes}
	return scanError
}

// lastInput saves last input char c if not a white-space. Helps to treat some ambibuities when parsing unquoted strings
// in object keys, etc
func (s *scanner) lastInput(c byte) {
	if !isWhiteSpace(c) {
		s.lastc = c
	}
}

// pushState pushes a new parse state newState onto the parse stack.
// an error state is returned if maxNestingDepth was exceeded, otherwise successState is returned.
func (s *scanner) pushState(c byte, newState int, successState int) int {
	s.states = append(s.states, newState)
	if newState == parseMultiLineValue {
		if len(s.states) > 1 && s.states[len(s.states)-2] == parseArrayValue {
			//if inside block array then store the calculated indent
			s.indents = append(s.indents, s.idn)
		} else {
			//if not then atleast +1 it
			s.indents = append(s.indents, s.idn+1)
		}
	} else {
		s.indents = append(s.indents, s.idn)
	}
	if len(s.states) <= maxNestingDepth {
		return successState
	}
	return s.error(c, "exceeded max depth")
}

// a more specialized pushState for a block object with indent checks
func (s *scanner) pushObjectState(c byte) int {
	n := len(s.states)
	diff := indentDiff(s)
	s.step = stateBeginValueOrEmpty
	if n == 0 ||
		s.states[n-1] == parseObjectValue && diff > 0 ||
		s.states[n-1] == parseArrayValue && diff >= 0 {
		s.sameLine = true
		s.pushState(c, parseObjectValue, scanObjectValue)
		if isLineBreak(c) {
			return endLine(s, scanObjectKey)
		}
		return scanObjectKey
	}
	//if we are here then input looks like "a: b:" which is invalid
	if n > 0 && s.states[n-1] == parseObjectValue && s.sameLine {
		return s.error(c, "expected object value, got key")
	}

	s.toggleObjectState()
	//if ":{" without space between ...
	if c == '{' {
		return s.pushState(c, parseFlowObjectKey, scanBeginObject)
	}
	//if ":[" without space between ...
	if c == '[' {
		return s.pushState(c, parseFlowArrayValue, scanBeginArray)
	}
	if isLineBreak(c) {
		return endLine(s, scanObjectKey)
	}
	return scanObjectKey
}

// a more specialized pushState for a block array with indent checks
func (s *scanner) pushArrayState(c byte) int {
	//if we are here then input looks like "a: - b" which is invalid
	if s.sameLine {
		return s.error(c, "unexpected array value")
	}
	n := len(s.states)
	s.idn += 2 //"- "
	idiff := indentDiff(s)
	// if top level first element or nested inside object or array of arrays
	if n == 0 ||
		idiff > 0 && (s.states[n-1] == parseArrayValue || s.states[n-1] == parseObjectValue) {
		s.pushState(c, parseArrayValue, scanArrayValue)
		s.step = stateBeginValueOrEmpty
		return scanSkipSpace
	}
	//already in array
	if n > 0 && s.states[n-1] == parseArrayValue && idiff == 0 {
		s.step = stateBeginValueOrEmpty
		return scanSkipSpace
	}
	return s.error(c, "in push array state")
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

// lastState returns the top value in states stack or -1
func (s *scanner) lastState() int {
	n := len(s.states)
	if n == 0 {
		return -1
	}
	return s.states[n-1]
}

// inArray tells if scanner is parsing a block or flow array
func (s *scanner) inArray() bool {
	n := len(s.states)
	return n > 0 && (s.states[n-1] == parseArrayValue || s.states[n-1] == parseFlowArrayValue)
}

// inBlockArray tells if scanner is parsing a block sequence (array) "- anything".
// its true only after first "- " has been scanned
func (s *scanner) inBlockArray() bool {
	n := len(s.states)
	return n > 0 && s.states[n-1] == parseArrayValue
}

// inArrayFlow tells if scanner is parsing array.
// For flow style this is true after '[' has been met
func (s *scanner) inFlowArray() bool {
	n := len(s.states)
	return n > 0 && s.states[n-1] == parseFlowArrayValue
}

// inBlockObject tells if scanner is parsing object.
// its true only after first legit ": " or ":\n" has been scanned
func (s *scanner) inBlockObject() bool {
	n := len(s.states)
	return n > 0 && (s.states[n-1] == parseObjectKey || s.states[n-1] == parseObjectValue)
}

// inFlowObject tells if scanner is parsing object.
// For flow style this is true after '{' has been met
func (s *scanner) inFlowObject() bool {
	n := len(s.states)
	return n > 0 && (s.states[n-1] == parseFlowObjectKey || s.states[n-1] == parseFlowObjectValue)
}

// inMultilineString tells if scanner is parsing a multiline literal, starting with '|' (or '>').
func (s *scanner) inMultilineString() bool {
	n := len(s.states)
	return n > 0 && s.states[n-1] == parseMultiLineValue
}

// toggleObjectState switches between parseObjectValue and parseObjectKey states
func (s *scanner) toggleObjectState() {
	n := len(s.states)
	if n > 0 {
		switch s.states[n-1] {
		case parseObjectKey:
			s.states[n-1] = parseObjectValue
		case parseObjectValue:
			s.states[n-1] = parseObjectKey
		case parseFlowObjectKey:
			s.states[n-1] = parseFlowObjectValue
		case parseFlowObjectValue:
			s.states[n-1] = parseFlowObjectKey
		}
	}
}

// isUnqDelim checks if c terminates an unquoted string, context agnostic
func (s *scanner) isUnqDelim(c byte) bool {
	if s.inFlowArray() || s.inFlowObject() {
		switch c {
		case '{', '}', '[', ']', ',', '\n', '\r':
			return true
		}
		return false
	}
	return isLineBreak(c)
}

// unlike json need to track line breaks separately
func isSpace(c byte) bool {
	return c <= ' ' && (c == ' ' || c == '\t')
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
	return s.idn - lastIndent
}

// stateBeginLine is the state after reading `\n` or at the beginning.
// because YAML is indent dependent, perform some checks each time on a new line
func stateBeginLine(s *scanner, c byte) int {
	if isLineBreak(c) {
		s.idn = 0
		return scanSkipSpace
	}
	if isSpace(c) {
		s.idn++
		return scanSkipSpace
	}
	//comments can have any indent
	if c == '#' {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	n, idiff := len(s.states), indentDiff(s)
	//if in nested state
	if n > 0 {
		switch s.states[n-1] {
		case parseObjectKey:
			switch {
			case idiff < 0:
				//pop & rerun
				s.popState()
				s.toggleObjectState()
				return stateBeginLine(s, c)
			case idiff > 0:
				return s.error(c, "unexpected increased indent when parsing object key")
			}
		case parseObjectValue:
			switch {
			case idiff < 0:
				s.popState()
				s.toggleObjectState()
				return stateBeginLine(s, c)
			case idiff == 0:
				//either array or object value was empty and this is the beginning of another key
				if c == '-' {
					s.step = stateBeginArrayValueS
					return scanBeginLiteral
				}
				s.toggleObjectState()
			}
		case parseArrayValue:
			// 2 means possible "- "
			switch {
			case idiff < -2:
				//pop & check if inside object. If true toggle object state (value -> key)
				//because this array was a value already and we expect another key
				s.popState()
				s.toggleObjectState()
				return stateBeginLine(s, c)
			case idiff == -2:
				if c == '-' {
					s.step = stateBeginArrayValueS
					return scanSkipSpace
				} else if c == '.' && s.idn == 0 {
					s.step = stateEndYaml1
					return scanContinue
				}
				s.popState()
				s.toggleObjectState()
				//this value can be inside the wrapping array of any or object. Cannot call endvalue here obviously, so
				//check this manually and report the error
				if len(s.states) == 0 {
					return s.error(c, "unexpected value after end of array")
				}
				return stateBeginLine(s, c)
			default:
				return s.error(c, "unexpected end of array")
			}
		case parseMultiLineValue:
			switch {
			case idiff < 0:
				s.popState()
				s.toggleObjectState()
				return stateBeginLine(s, c)
			default:
				s.step = stateInMultiline
				return scanContinue
			}
		}
	}
	return stateBeginValue(s, c)
}

// endLine should be called each time when isLineBreak(c) == true on a non-blank line
func endLine(s *scanner, opcode int) int {
	s.idn = 0
	s.step = stateBeginLine
	s.sameLine = false
	return opcode
}

// stateBeginValueOrEmpty is the state when any token or space is expected
func stateBeginValueOrEmpty(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}
	if isLineBreak(c) {
		return endLine(s, scanSkipSpace)
	}
	//end of flow array or delimiter
	if c == ']' || c == ',' {
		return stateEndValue(s, c)
	}
	if c == '}' {
		n := len(s.states)
		if n == 0 || !s.inFlowObject() {
			return s.error(c, "while not in object")
		}
		s.states[n-1] = parseFlowObjectValue
		return stateEndValue(s, c)
	}
	return stateBeginValue(s, c)
}

// stateBeginValue is the state at the beginning of any token
func stateBeginValue(s *scanner, c byte) int {
	if s.endTop {
		// Completed top-level before the current byte.
		s.step = stateEndTop
		return stateEndTop(s, c)
	}
	switch c {
	case '\r', '\n':
		return endLine(s, scanSkipSpace)
	case '{':
		s.step = stateBeginValueOrEmpty
		return s.pushState(c, parseFlowObjectKey, scanBeginObject)
	case '[':
		s.step = stateBeginValueOrEmpty
		return s.pushState(c, parseFlowArrayValue, scanBeginArray)
	case '"':
		s.step = stateInString
		return scanBeginLiteral
	case '\'':
		s.step = stateInStringSq
		return scanBeginLiteral
	case '-':
		s.step = stateHyphen
		return scanBeginLiteral
	case '~': //null
		s.step = stateInStringUnq
		return scanBeginLiteral
	case '&':
		s.step = stateBeginAnchor
		return scanBeginLiteral
	case '*':
		s.step = stateBeginAlias
		return scanBeginLiteral
	case '!':
		s.step = stateExplicitType1
		return scanBeginLiteral
	case '.':
		s.step = stateDotBegin
		return scanBeginLiteral
	case '\\', '/', '_', '(', ')', '+':
		s.step = stateInStringUnq
		return scanBeginLiteral
	case '<':
		s.step = stateBeginMerge
		return scanBeginLiteral
	case '|', '>':
		s.step = stateBeginMultilineChomp
		return s.pushState(c, parseMultiLineValue, scanBeginLiteral)
	case '#':
		s.step = stateBeginComment
		return scanSkipSpace
	default:
		if unicode.IsLetter(rune(c)) || '0' <= c && c <= '9' {
			s.step = stateInStringUnq
			return scanBeginLiteral
		}
	}
	return s.error(c, "looking for beginning of value")
}

// stateEndValue is the state after completing a value,
// such as after reading `{}` or `true` or `["x"`.
func stateEndValue(s *scanner, c byte) int {
	if isSpace(c) {
		s.step = stateEndValue
		return scanSkipSpace
	}
	if c == '#' {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	//for quoted keys need to check here... for unquoted & numbers this is done in a stateKeyOrUnq
	if c == ':' {
		return s.pushObjectState(c)
	}
	n := len(s.states)
	if n == 0 {
		// Completed top-level before the current byte.
		s.step = stateEndTop
		s.endTop = true
		return stateEndTop(s, c)
	}

	ps := s.states[n-1]
	switch ps {
	case parseObjectKey:
		//if key is unquoted string, then check space in ": ", otherwise check ":"
		if c == ':' || s.lastc == ':' && isWhiteSpace(c) {
			s.toggleObjectState()
			if isLineBreak(c) {
				return endLine(s, scanObjectKey)
			}
			s.step = stateBeginValueOrEmpty
			return scanObjectKey
		}
		return s.error(c, "after object key")
	case parseObjectValue:
		if isLineBreak(c) {
			s.toggleObjectState()
			return endLine(s, scanObjectValue)
		}
		return s.error(c, "after object key:value pair")
	case parseFlowObjectKey:
		//if key is unquoted string, then check space in ": ", otherwise check ":"
		if c == ':' || s.lastc == ':' && isWhiteSpace(c) {
			s.toggleObjectState()
			s.step = stateBeginValueOrEmpty
			return scanObjectKey
		}
		if s.lastc == ':' && (c == '{' || c == '[') {
			s.toggleObjectState()
			return stateBeginValueOrEmpty(s, c)
		}
		return s.error(c, "after object key")
	case parseFlowObjectValue:
		if c == ',' {
			s.toggleObjectState()
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
			return endLine(s, scanArrayValue)
		}
		return s.error(c, "after array element")
	case parseFlowArrayValue:
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
			return endLine(s, scanSkipSpace)
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
		s.step = stateNewDoc1
		return scanContinue
	}
	//check for document separator
	if c == '.' && s.idn == 0 {
		s.step = stateEndYaml1
		return scanContinue
	}
	s.error(c, "after top-level value")
	return scanEnd
}

// stateEndTop is the state after finishing the "\n...\n"
// all parsing will be stopped
func stateEndYaml(s *scanner, c byte) int {
	return scanEnd
}

// stateKeyOrUnq is the state after reading ':' in an unquoted string
func stateKeyOrUnq(s *scanner, c byte) int {
	if isSpace(c) || s.isUnqDelim(c) {
		return s.pushObjectState(c)
	}
	s.step = stateInStringUnq
	return stateInStringUnq(s, c)
}

// stateBeginComment is the state after '#' denoting comment till the end of line
func stateBeginComment(s *scanner, c byte) int {
	if isLineBreak(c) {
		return endLine(s, scanSkipSpace)
	}
	return scanSkipSpace
}

// stateBeginMerge is the state after '<' denoting merge instruction "<<:"
func stateBeginMerge(s *scanner, c byte) int {
	if c == '<' {
		s.step = stateMerge1
		return scanContinue
	}
	s.step = stateInStringUnq
	return scanContinue
}

// stateMerge1 is the state after "<<" denoting merge instruction "<<:"
func stateMerge1(s *scanner, c byte) int {
	return s.error(c, "in merge instruction which is not supported since yaml 1.2")
}

// stateEmptyLine is the state awaiting only white-spaces or comments till the end of line
func stateEmptyLine(s *scanner, c byte) int {
	if isSpace(c) {
		return scanContinue
	}
	if isLineBreak(c) {
		return endLine(s, scanContinue) //not sure about code
	}
	if c == '#' {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	return s.error(c, "on empty line")
}

// stateBeginMultilineChomp is the state after '|' or '>' multiline literal indicator
func stateBeginMultilineChomp(s *scanner, c byte) int {
	if c == '+' || c == '-' {
		s.step = stateBeginMultiline
		return scanContinue
	}
	return stateBeginMultiline(s, c)
}

// stateBeginMultiline is the state after '|' or '>' with optional chomp
func stateBeginMultiline(s *scanner, c byte) int {
	if isSpace(c) {
		s.step = stateEmptyLine
		return scanContinue //or scanSkipSpace?
	}
	if isLineBreak(c) {
		return endLine(s, scanContinue) //not sure about code
	}
	return s.error(c, "at the beginning of multiline string")
}

// stateInMultiline is the state in a multiline string: all characters are accepted and only indent will judge us
func stateInMultiline(s *scanner, c byte) int {
	if isLineBreak(c) {
		return endLine(s, scanSkipSpace)
	}
	return scanContinue
}

func stateInStringUnqOrComment(s *scanner, c byte) int {
	if isSpace(c) {
		return scanContinue //can't use scanSkipSpace because not sure if string continues
	}
	if c == '#' {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	s.step = stateInStringUnq
	return stateInStringUnq(s, c)
}

// stateInStringUnq is the state after reading f,t (and not fa, tr following
func stateInStringUnq(s *scanner, c byte) int {
	if isLineBreak(c) {
		return stateEndValue(s, c)
	}
	if c == ':' {
		s.step = stateKeyOrUnq
		return scanContinue
	}

	if s.isUnqDelim(c) {
		return stateEndValue(s, c)
	}

	if isSpace(c) {
		s.step = stateInStringUnqOrComment
		return scanContinue
	}

	if c < 0x20 {
		return s.error(c, "in unquoted string literal")
	}
	return scanContinue
}

// stateInString is the state after reading `"`.
func stateInString(s *scanner, c byte) int {
	if c == '"' || c == '\'' {
		s.step = stateEndValue
		return scanContinue
	}
	if c == '\\' {
		s.step = stateInStringEsc
		return scanContinue
	}
	if s.lastc == '\\' && c == '\n' {
		//to catch "\\\r\n" backslash with \r\n - line continuation
		return stateInStringEsc(s, c)
	}
	if c < 0x20 {
		return s.error(c, "in string literal")
	}
	return scanContinue
}

// stateInStringSq is the state after reading `'`.
func stateInStringSq(s *scanner, c byte) int {
	if c == '\'' {
		// s.step = stateEndValue
		s.step = stateInStringSq2
		return scanContinue
	}
	// c < 0x20 is ok in a single quoted string they are preserved as is
	return scanContinue
}

// stateInStringSq is the state after reading `'` inside a single-quoted string.
// it may be a terminating `'` then end state or a duplicate `”` then check and return back to string
func stateInStringSq2(s *scanner, c byte) int {
	if c == '\'' {
		s.step = stateInStringSq
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateBeginAnchor is the state after reading '&' at the beginning of the value
func stateBeginAnchor(s *scanner, c byte) int {
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' {
		s.step = stateInAnchor
		return scanContinue
	}
	return s.error(c, "in anchor literal")
}

// stateInAnchor is the state after reading '&' and atleast one legit character
func stateInAnchor(s *scanner, c byte) int {
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' {
		s.step = stateInAnchor
		return scanContinue
	}
	if isSpace(c) {
		s.step = stateBeginValueOrEmpty
		return scanSkipSpace
	}
	if isLineBreak(c) {
		// return endLine(s, scanContinue)
		return endLine(s, scanSkipSpace)
	}

	return s.error(c, "in anchor literal")
}

// stateBeginAnchor is the state after reading '&' at the beginning of the value
func stateBeginAlias(s *scanner, c byte) int {
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' {
		s.step = stateInAlias
		return scanContinue
	}
	return s.error(c, "in alias literal")
}

// stateInAlias is the state after reading '*' at the beginning of the value
func stateInAlias(s *scanner, c byte) int {
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' {
		s.step = stateInAlias
		return scanContinue
	}
	if isSpace(c) {
		s.step = stateEndValue
		return scanSkipSpace
	}
	if isLineBreak(c) || c == ':' || s.isUnqDelim(c) {
		return stateEndValue(s, c)
	}

	return s.error(c, "in alias literal")
}

// stateInStringEsc is the state after reading `"\` during a quoted string.
func stateInStringEsc(s *scanner, c byte) int {
	switch c {
	case 'a', 'b', 'e', 'f', 'n', 'r', 't', 'v', '0', '\\', '/', '"', ' ', '_', 'N', 'L', 'P':
		s.step = stateInString
		return scanContinue
	case 'x':
		s.step = stateInStringEscx
		return scanContinue
	case 'u':
		s.step = stateInStringEscu
		return scanContinue
	case 'U':
		s.step = stateInStringEscU
		return scanContinue
	case '\r', '\n':
		// backslash followed by a line break in a double-quoted string acts as a line continuation char
		if s.lastc == '\\' {
			s.step = stateInString
			return scanContinue
		}
	}
	return s.error(c, "in string escape code")
}

// stateInStringEscx is the state after reading `"\x` during a double-quoted string.
func stateInStringEscx(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscx1
		return scanContinue
	}
	return s.error(c, "in \\x hexadecimal character escape")
}

// stateInStringEscx1 is the state after reading `"\x1` during a double-quoted string.
func stateInStringEscx1(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInString
		return scanContinue
	}
	return s.error(c, "in \\x hexadecimal character escape")
}

// stateInStringEscu is the state after reading `"\u` during a quoted string.
func stateInStringEscu(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscu1
		return scanContinue
	}
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscu1 is the state after reading `"\u1` during a quoted string.
func stateInStringEscu1(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscu12
		return scanContinue
	}
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscu12 is the state after reading `"\u12` during a quoted string.
func stateInStringEscu12(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscu123
		return scanContinue
	}
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscu123 is the state after reading `"\u123` during a quoted string.
func stateInStringEscu123(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInString
		return scanContinue
	}
	return s.error(c, "in \\u hexadecimal character escape")
}

// stateInStringEscU is the state after reading `"\U` during a quoted string.
func stateInStringEscU(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU1
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU1 is the state after reading `"\U1` during a quoted string.
func stateInStringEscU1(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU12
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU12 is the state after reading `"\U12` during a quoted string.
func stateInStringEscU12(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU123
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU123 is the state after reading `"\U123` during a quoted string.
func stateInStringEscU123(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU1234 //sigh
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU1234 is the state after reading `"\U1234` during a quoted string.
func stateInStringEscU1234(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU12345
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU12345 is the state after reading `"\U12345` during a quoted string.
func stateInStringEscU12345(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU123456
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU123456 is the state after reading `"\U123456` during a quoted string.
func stateInStringEscU123456(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInStringEscU1234567
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateInStringEscU1234567 is the state after reading `"\U1234567` during a quoted string.
func stateInStringEscU1234567(s *scanner, c byte) int {
	if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
		s.step = stateInString
		return scanContinue
	}
	return s.error(c, "in \\U hexadecimal character escape")
}

// stateNewDoc1 is the state after '-' on a new line when document separator "---\n" is expected
func stateNewDoc1(s *scanner, c byte) int {
	if c == '-' {
		s.step = stateNewDoc2
		return scanContinue
	}
	return s.error(c, "in new document --- separator")
}

// stateNewDoc2 is the state after "--" on a new line
func stateNewDoc2(s *scanner, c byte) int {
	if c == '-' {
		s.step = stateNewDoc3
		return scanContinue
	}
	return s.error(c, "in new document --- separator")
}

// stateNewDoc3 is the state after "---" on a new line
func stateNewDoc3(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}
	if c == '#' {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	if isLineBreak(c) {
		//force check of the last document for correctness
		s.step = stateBeginLine
		if s.eof() == scanError {
			return scanError
		}
		s.reset()
		return scanContinue
	}
	return s.error(c, "in a new document separator")
}

// stateEndYaml is the state after '.' on a new line when end document separator "...\n" is expected
// parsing will be stopped after all dots met.
func stateEndYaml1(s *scanner, c byte) int {
	if c == '.' {
		s.step = stateEndYaml2
		return scanContinue
	}
	return s.error(c, "in end document ... separator")
}

// stateEndYaml2 is the state after ".." on a new line
func stateEndYaml2(s *scanner, c byte) int {
	if c == '.' {
		s.step = stateEndYaml3
		return scanContinue
	}
	return s.error(c, "in end document ... separator")
}

// stateEndYaml3 is the state after "..." on a new line
func stateEndYaml3(s *scanner, c byte) int {
	if isSpace(c) {
		return scanSkipSpace
	}
	if c == '#' {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	if isLineBreak(c) {
		//force check of the last document for correctness
		s.step = stateEndYaml
		if s.eof() == scanError {
			return scanError
		}
		return scanEnd
	}
	return s.error(c, "in an end document separator")
}

// stateBeginArrayValueS is the state expecting ' ' in "- "
func stateBeginArrayValueS(s *scanner, c byte) int {
	if isSpace(c) {
		return s.pushArrayState(c)
	}
	if c == '-' && s.idn == 0 {
		s.step = stateNewDoc2
		return scanContinue
	}
	return s.error(c, "in array value prefix")
}

// stateHyphen is the state after reading `-` during a number or array element
func stateHyphen(s *scanner, c byte) int {
	switch {
	case c == ' ' || c == '\t':
		return s.pushArrayState(c)
	case c == '-' && s.idn == 0:
		s.step = stateNewDoc2
		return scanContinue
	case c == '\r' || c == '\n':
		return stateEndValue(s, c)
	default:
		s.step = stateInStringUnq
		return scanContinue
	}
}

// stateDotBegin is the state after reading the '.' at the beginning of a value (unlike stateDot)
func stateDotBegin(s *scanner, c byte) int {
	if c == '.' && s.idn == 0 {
		s.step = stateEndYaml2
		return scanContinue
	}
	return stateInStringUnq(s, c)
}

// stateExplicitType1 is the state after reading '!' at the value beginning
func stateExplicitType1(s *scanner, c byte) int {
	if c == '!' {
		s.step = stateExplicitType2
		return scanContinue
	}
	return s.error(c, "in explicit type")
}

// stateExplicitType2 is the state after reading "!!" at the value beginning
func stateExplicitType2(s *scanner, c byte) int {
	if 'a' <= c && c <= 'z' {
		s.step = stateExplicitType2
		return scanContinue
	}
	if isWhiteSpace(c) {
		s.step = stateBeginValueOrEmpty
		return scanSkipSpace
	}
	return s.error(c, "in explicit type")
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
