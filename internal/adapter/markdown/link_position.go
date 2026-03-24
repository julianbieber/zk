package markdown

import (
	"github.com/yuin/goldmark/ast"
)

// LinkPosition holds the source position of a link.
type LinkPosition struct {
	Start int // Byte offset of '['
	End   int // Byte offset after ')'
}

// GetLinkPosition calculates the source position of a link on-demand.
// Returns nil if position cannot be determined.
func GetLinkPosition(link *ast.Link, source []byte) *LinkPosition {
	// Use potentially cached position:
	if startVal, ok := link.AttributeString("linkStart"); ok {
		if endVal, ok := link.AttributeString("linkEnd"); ok {
			start, ok1 := startVal.(int)
			end, ok2 := endVal.(int)
			if ok1 && ok2 {
				return &LinkPosition{Start: start, End: end}
			}
		}
	}

	start, end := findLinkPositions(link, source)
	if start < 0 || end <= start {
		return nil
	}

	// Cache for future calls.
	link.SetAttributeString("linkStart", start)
	link.SetAttributeString("linkEnd", end)

	return &LinkPosition{Start: start, End: end}
}

// findLinkPositions finds the byte offsets for a Link node.
// Returns (start of '[', end after ')' or ']').
func findLinkPositions(link *ast.Link, source []byte) (int, int) {
	// Use the upstream position for the start of the link.
	linkStart := link.Pos()
	if linkStart < 0 {
		return -1, -1
	}

	// Find the position of the next sibling node (or EOF)…
	searchStart := len(source)
	if next := ast.Node(link).NextSibling(); next != nil {
		searchStart = next.Pos()
	}

	// … Then walk backwards to the first `]` or `)`.
	for i := searchStart - 1; i > linkStart; i-- {
		if source[i] == ')' || source[i] == ']' {
			return linkStart, i + 1
		}
	}

	return linkStart, -1
}
