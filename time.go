package gyaml

import (
	"strings"
	"time"
)

// This is a subset of the formats permitted by the regular expression
// defined at http://yaml.org/type/timestamp.html. Note that time.Parse
// cannot handle: "2001-12-14 21:59:43.10 -5" from the examples.
var timestampFormats = []string{
	time.RFC3339Nano,
	"2006-01-02t15:04:05.999999999Z07:00", // RFC3339Nano with lower-case "t".
	time.DateTime,
	time.DateOnly,

	// Not in examples, but to preserve backward compatibility by quoting time values.
	"15:4",
}

const legalTimeChars = "0123456789-: tTZ."

func isTimestamp(value string) bool {
	//rough check if not a timestamp, saves a looot of cpu
	n := len(value)
	if n < 3 || n > len(time.RFC3339Nano) {
		return false
	}
	for i := range n {
		if !strings.ContainsRune(legalTimeChars, rune(value[i])) {
			return false
		}
	}

	for _, format := range timestampFormats {
		if _, err := time.Parse(format, value); err == nil {
			return true
		}
	}
	return false
}
