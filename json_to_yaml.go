package gyaml

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"

	"github.com/denisbakhtin/gyaml/token"
)

// an attemp to convert json string, produced by IndendMarshal, into a correct yaml string with proper indents
func JsonToYaml(src []byte) ([]byte, error) {
	if bytes.Equal(src, []byte{'{', '}'}) {
		return []byte{'{', '}', '\n'}, nil
	}
	res := make([]byte, len(src))
	copy(res, src)
	res = bytes.TrimLeft(res, "{\n")
	res = bytes.TrimRight(res, "}")
	res = bytes.ReplaceAll(res, []byte{':', ' ', '{', '\n'}, []byte{':', '\n'})
	//unquote keys
	reKey := regexp.MustCompile(`(?m)^([\s]*)(")([a-zA-Z0-9\-:]+)(":)`)
	res = reKey.ReplaceAll(res, []byte("$1$3:"))

	//unquote datetimes
	//TODO: see strict RFC3339 Json encoding algorithm
	reDT := regexp.MustCompile(`(:\s)(")(\d{4}-\d{2}-\d{2}[tT]\d{2}:\d{2}:\d{2}Z)(")`)
	res = reDT.ReplaceAll(res, []byte("$1$3"))

	//unquote strings
	reUq := regexp.MustCompile(`(?m)(:\s)(")(.)+("\n)`)
	res = reUq.ReplaceAllFunc(res, func(b []byte) []byte {
		start := bytes.Index(b, []byte{'"'})
		end := bytes.LastIndex(b, []byte{'"'})
		//TODO: remove datetime check from IsNeedQuoted
		if token.IsNeedQuoted(string(b[start+1 : end])) {
			//leave as is
			return b
		}
		result := make([]byte, 0, len(b))
		result = append(result, []byte{':', ' '}...)
		result = append(result, b[start+1:end]...)
		result = append(result, '\n')
		return result
	})

	//move empty {} object to same line as key
	reEmpty := regexp.MustCompile(`:(\s)*({})`)
	res = reEmpty.ReplaceAll(res, []byte(": {}"))

	//remove trailing coma
	reComa := regexp.MustCompile(`,\n`)
	res = reComa.ReplaceAll(res, []byte{'\n'})

	//because json has a +1 indented nesting, deindent everything by 2 spaces
	reIndent := regexp.MustCompile(`(?m)^([[:blank:]]{2})([[:blank:]]*)`)
	res = reIndent.ReplaceAll(res, []byte("$2"))

	//remove closing }
	reClosing := regexp.MustCompile(`(?m)^([[:blank:]]*)(}\n)`)
	res = reClosing.ReplaceAll(res, []byte("$1"))

	res = bytes.Trim(res, " ")

	//append last \n
	if len(res) > 0 && res[len(res)-1] != '\n' {
		res = append(res, '\n')
	}
	return res, nil
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *json.Encoder {
	return &json.Encoder{w: w, escapeHTML: false}
}

// MarshalIndent is like [Marshal] but applies [Indent] to format the output.
// Each JSON element in the output will begin on a new line beginning with prefix
// followed by one or more copies of indent according to the indentation nesting.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	//can't use original Marshal, because it can't turn of escapeHtml
	//b, err := Marshal(v)
	b := bytes.Buffer{}
	err := json.NewEncoder(&b).Encode(v)
	if err != nil {
		return nil, err
	}
	b2 := make([]byte, 0, indentGrowthFactor*len(b))
	b2, err = appendIndent(b2, b, prefix, indent)
	if err != nil {
		return nil, err
	}
	return b2, nil
}
