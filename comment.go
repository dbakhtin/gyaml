package gyaml

// CommentPosition type of the position for comment.
type CommentPosition int

const (
	CommentHeadPosition CommentPosition = CommentPosition(iota)
	CommentLinePosition
	CommentFootPosition
)

func (p CommentPosition) String() string {
	switch p {
	case CommentHeadPosition:
		return "Head"
	case CommentLinePosition:
		return "Line"
	case CommentFootPosition:
		return "Foot"
	default:
		return ""
	}
}

// LineComment create a one-line comment for CommentMap.
func LineComment(text string) *Comment {
	return &Comment{
		Texts:    []string{text},
		Position: CommentLinePosition,
	}
}

// HeadComment create a multiline comment for CommentMap.
func HeadComment(texts ...string) *Comment {
	return &Comment{
		Texts:    texts,
		Position: CommentHeadPosition,
	}
}

// FootComment create a multiline comment for CommentMap.
func FootComment(texts ...string) *Comment {
	return &Comment{
		Texts:    texts,
		Position: CommentFootPosition,
	}
}

// Comment raw data for comment.
type Comment struct {
	Texts    []string
	Position CommentPosition
}

// CommentMap map of the position of the comment and the comment information.
type CommentMap map[string][]*Comment
