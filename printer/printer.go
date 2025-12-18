package printer

import (
	"fmt"
)

// Property additional property set for each the token
type Property struct {
	Prefix string
	Suffix string
}

// PrintFunc returns property instance
type PrintFunc func() *Property

// Printer create text from token collection or ast
type Printer struct {
	LineNumber       bool
	LineNumberFormat func(num int) string
	MapKey           PrintFunc
	Anchor           PrintFunc
	Alias            PrintFunc
	Bool             PrintFunc
	String           PrintFunc
	Number           PrintFunc
	Comment          PrintFunc
}

// PrintNode create text from ast.Node
func (p *Printer) Print(value any, breakLine bool) []byte {
	if breakLine {
		return fmt.Appendf(nil, "%+v\n", value)
	}
	return fmt.Appendf(nil, "%+v", value)
}
