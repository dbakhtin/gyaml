package gyaml

import (
	"regexp"
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

var legalTimeChars = regexp.MustCompile(`^[0-9]+[-:0-9\stTzZ.]*$`)

func isTimestamp(value string) bool {
	//rude check if not a timestamp, saves a looot of cpu
	if !legalTimeChars.MatchString(value) {
		return false
	}
	for _, format := range timestampFormats {
		if _, err := time.Parse(format, value); err == nil {
			return true
		}
	}
	return false
}
