package pdfin

import (
	"strings"
	"sync"

	"github.com/phpdave11/gofpdf"
)

// The PDF's standard fonts (Times, Helvetica, Courier) need not say how
// wide their glyphs are: every reader knows. Without the widths a line's
// end was guessed at Courier's, too far for Times; their metrics come
// from gofpdf, which has them.

var (
	stdMu   sync.Mutex
	stdPDF  *gofpdf.Fpdf
	stdSeen = map[string]float64{} // family+style+text: width at 1 pt
)

// standardWidth is how wide s is in the standard font named font at size
// points, in inches; 0 if font is not one of them.
func standardWidth(font, s string, size float64) float64 {
	name := strings.ToLower(font)
	if i := strings.IndexByte(name, '+'); i >= 0 { // a subset: ABCDEF+Times-Bold
		name = name[i+1:]
	}
	var family string
	switch {
	case strings.HasPrefix(name, "times"):
		family = "Times"
	case strings.HasPrefix(name, "helvetica"), strings.HasPrefix(name, "arial"):
		family = "Helvetica"
	case strings.HasPrefix(name, "courier"):
		family = "Courier"
	default:
		return 0
	}
	style := ""
	if strings.Contains(name, "bold") {
		style += "B"
	}
	if strings.Contains(name, "italic") || strings.Contains(name, "oblique") {
		style += "I"
	}
	if size <= 0 {
		size = 12
	}
	key := family + style + "\x00" + s
	stdMu.Lock()
	defer stdMu.Unlock()
	w, ok := stdSeen[key]
	if !ok {
		if stdPDF == nil {
			stdPDF = gofpdf.New("P", "pt", "Letter", "")
		}
		stdPDF.SetFont(family, style, 1)
		w = stdPDF.GetStringWidth(stdPDF.UnicodeTranslatorFromDescriptor("")(s))
		if len(stdSeen) < 4096 {
			stdSeen[key] = w
		}
	}
	return w * size / 72
}
