package parse

import "fmt"

// Pos captures positional information during parsing.
type Pos struct {
	line int // Line number, starting at 1
	col  int // Column number (character count), starting at 1
}

// IsValid reports whether this Pos has been initialized.  The zero
// Pos is invalid.
func (p Pos) IsValid() bool {
	return p.line > 0 && p.col > 0
}

func (p Pos) String() string {
	if !p.IsValid() {
		return "[no pos]"
	}
	// We try to keep the output the same as go/parser, so that errors look
	// consistent to the user.  That's also why we retain go/parser semantics
	// where lines and columns start at 1 (and not 0).
	return fmt.Sprintf("%d:%d", p.line, p.col)
}
