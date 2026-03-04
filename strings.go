// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2

package gyaml

import (
	"bytes"
	"strings"
)

// maximum length of the reserved word
var maxReservedLength int
var reservedMap map[string]struct{}

func init() {
	reservedMap = make(map[string]struct{})
	allReservedSlice := [][]string{
		reservedBoolKeywords,
		reservedInfKeywords,
		reservedMiscKeywords,
		reservedNanKeywords,
		reservedNullKeywords,
	}
	for i := range allReservedSlice {
		for _, v := range allReservedSlice[i] {
			if len(v) > maxReservedLength {
				maxReservedLength = len(v)
			}
			reservedMap[v] = struct{}{}
		}
	}
}

var (
	// taken from go-yaml:
	reservedNullKeywords = []string{
		"null",
		"Null",
		"NULL",
		"~",
	}
	reservedBoolKeywords = []string{
		"true",
		"True",
		"TRUE",
		"false",
		"False",
		"FALSE",
	}

	reservedInfKeywords = []string{
		".inf",
		".Inf",
		".INF",
		"-.inf",
		"-.Inf",
		"-.INF",
	}
	reservedNanKeywords = []string{
		".nan",
		".NaN",
		".NAN",
	}
	reservedMiscKeywords = []string{
		"-",
	}
)

func isReserved(s string) bool {
	//it can't be reserved for sure
	if len(s) > maxReservedLength {
		return false
	}
	_, ok := reservedMap[s]
	return ok
}

func isNeedQuoted(value string) bool {
	if value == "" || value == "-" {
		return true
	}
	if isReserved(value) {
		return true
	}
	if isNumber(value) {
		return true
	}
	first := value[0]
	switch first {
	case '*', '&', '[', '{', '}', ']', ',', '!', '|', '>', '%', '\'', '"', '@', ' ', '`':
		return true
	}
	last := value[len(value)-1]
	switch last {
	case ':', ' ':
		return true
	}
	for i, c := range value {
		switch c {
		case '#', '\\':
			return true
		case ':', '-':
			if i+1 < len(value) && value[i+1] == ' ' {
				return true
			}
		}
	}
	return isTimestamp(value)
}

// detectLineBreakCharacter detect line break character in only one inside scalar content scope.
func detectLineBreakChars(src []byte) []byte {
	n := []byte{'\n'}
	r := []byte{'\r'}
	rn := []byte{'\r', '\n'}

	nc := bytes.Count(src, n)
	rc := bytes.Count(src, r)
	rnc := bytes.Count(src, rn)
	switch {
	case nc == rnc && rc == rnc:
		return rn
	case rc > nc:
		return r
	default:
		return n
	}
}

// detectLineBreakCharacter detect line break character in only one inside scalar content scope.
func detectLineBreakCharacter(src string) string {
	nc := strings.Count(src, "\n")
	rc := strings.Count(src, "\r")
	rnc := strings.Count(src, "\r\n")
	switch {
	case nc == rnc && rc == rnc:
		return "\r\n"
	case rc > nc:
		return "\r"
	default:
		return "\n"
	}
}

// literalBlockHeader detect literal block scalar header
func literalBlockHeader(value string) string {
	lbc := detectLineBreakCharacter(value)

	switch {
	case !strings.Contains(value, lbc):
		return ""
	case strings.HasSuffix(value, lbc+lbc):
		return "|+"
	case strings.HasSuffix(value, lbc):
		return "|"
	default:
		return "|-"
	}
}
