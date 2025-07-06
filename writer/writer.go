package writer

import (
	"io"

	"github.com/lapingvino/lexington/lex"
)

// Writer defines the interface for different screenplay output formats (e.g., HTML, PDF, FDX).
// Implementations of this interface will be responsible for converting a lex.Screenplay
// into the desired output format and writing it to the provided io.Writer.
type Writer interface {
	Write(w io.Writer, screenplay lex.Screenplay) error
}
