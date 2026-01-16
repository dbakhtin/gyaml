package gyaml

import (
	"fmt"
)

type pathNode interface {
	fmt.Stringer
	chain(pathNode) pathNode
	//filter(ast.Node) (ast.Node, error)
	//replace(ast.Node, ast.Node) error
}

// Path represent YAMLPath ( like a JSONPath ).
type Path struct {
	node pathNode
}

// String path to text.
func (p *Path) String() string {
	return p.node.String()
}
