package pdfin

import (
	"sort"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/internal/pdf"
)

// TextQuality is how a page's own text layer reads: of its words, how
// many look like words of a language. A scan's text layer made by a poor
// OCR has odd characters (stray marks of the scan read as | _ ~, alone
// or at a line's ends),
// words with case or script changing inside them, letters and digits
// mixed (l0ve), punctuation glued inside a word (OLD“MILL”), words broken
// apart by stray spaces (T H R E E), and a line's words at two heights.
// Printed text has hardly any. (Word lists and letter statistics were
// tried as well, against public-domain English: a poor OCR's layer has
// as many words outside them as a clean script has names and slang, so
// they tell nothing; the stray marks and broken lines do.)
type TextQuality struct {
	Words int // the tokens between spaces, punctuation alone not counted
	Odd   int // of them, the ones that look like no word
	Lines int
	// Split are the lines printed just above or below the line before
	// them, too close to be the next line: the words of one line at
	// different heights, as a skewed scan's text layer has them, so the
	// line is read as two
	Split int
}

// qualityOf is how a page's lines read.
func qualityOf(ls []line) TextQuality {
	var q TextQuality
	var gaps []float64
	for i, l := range ls {
		q.add(l.text())
		if i > 0 {
			gaps = append(gaps, l.Y-ls[i-1].Y)
		}
	}
	q.Lines = len(ls)
	if len(gaps) < 4 {
		return q
	}
	sorted := append([]float64(nil), gaps...)
	sort.Float64s(sorted)
	step := sorted[len(sorted)/2] // the usual gap: a line's
	for _, g := range gaps {
		if g < 0.35*step {
			q.Split++
		}
	}
	return q
}

// Score is the share of a page's words that look like words, a line
// split in two counted as an odd word too; 1 if it has none.
func (q TextQuality) Score() float64 {
	if q.Words == 0 {
		return 1
	}
	return max(0, 1-float64(q.Odd+q.Split)/float64(q.Words))
}

// add counts the words of a line of text.
func (q *TextQuality) add(text string) {
	tokens := strings.Fields(text)
	single := 0 // single letters in a row: a word spread by stray spaces
	for i, tok := range tokens {
		core := strings.TrimFunc(tok, plainPunct)
		if core == "" { // punctuation alone: a dash between words, an ellipsis, or a stray mark
			edge := i == 0 || i == len(tokens)-1
			if strings.IndexFunc(tok, oddMark) >= 0 || !ellipsis(tok) && !spaced(tok) && (edge || !dash(tok)) {
				q.Words++
				q.Odd++
			}
			single = 0
			continue
		}
		q.Words++
		if len([]rune(core)) == 1 && unicode.IsLetter([]rune(core)[0]) {
			single++
			if single >= 3 { // T H R E E: every letter of it is odd
				q.Odd++
			}
			if single == 3 { // the two before it too
				q.Odd += 2
			}
			continue
		}
		single = 0
		if !wordLike(core) {
			q.Odd++
		}
	}
}

// dash and ellipsis are punctuation that stands alone between words.
func dash(s string) bool     { return strings.Trim(s, "-–—") == "" }
func spaced(s string) bool   { return strings.Trim(s, "!?;:«»‹›") == "" } // French: « Oui ! »
func ellipsis(s string) bool { return s == "…" || strings.Trim(s, ".") == "" && len(s) >= 3 }

// plainPunct is punctuation a word can start or end with.
func plainPunct(r rune) bool {
	return strings.ContainsRune(`"'“”‘’„«»‹›()[]{}.,;:!?¡¿…*-–—/`, r) || r == ' '
}

// oddMark is a character a text layer has for a stray mark of the scan:
// none of a word's, none of plain punctuation's.
func oddMark(r rune) bool {
	return !plainPunct(r) && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) &&
		!sign(r)
}

// sign is a sign a word or a line can have: & % € # @ °, an emoji, ♪.
func sign(r rune) bool {
	return strings.ContainsRune("&%$€£#@+=°§", r) || unicode.Is(unicode.So, r)
}

// wordLike says if a token (its outer punctuation off) looks like a word:
// letters of one script, in one case or capitalised (McDONALD, O'Brien
// too), joined by an apostrophe, hyphen or full stop at most; or a number
// (12, 12A, 3:15, 1st, 1,000.50); or a sign (&, 20%).
func wordLike(s string) bool {
	rs := []rune(s)
	letters, digits := 0, 0
	var script *unicode.RangeTable
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r):
			letters++
			sc := scriptOf(r)
			if script == nil {
				script = sc
			} else if sc != nil && sc != script {
				return false // mixed scripts: Latin and Cyrillic look-alikes
			}
			// a capital after a small letter: Mc and Mac names only
			if i > 0 && unicode.IsUpper(r) && unicode.IsLower(rs[i-1]) {
				if p := string(rs[:i]); p != "Mc" && p != "Mac" && p != "De" && p != "La" && p != "Le" && p != "Di" && p != "Van" {
					return false
				}
			}
		case unicode.IsMark(r):
		case unicode.IsDigit(r):
			digits++
		case 0x3000 <= r && r <= 0x303f || 0xff01 <= r && r <= 0xff65: // CJK's punctuation, inside its runs
		case strings.ContainsRune("'’-.‐,:/־׳״", r) && i > 0 && i < len(rs)-1:
			// inside: don't, well-known, U.S.A, 1,000, 3:15, and/or
			if !unicode.IsLetter(rs[i-1]) && !unicode.IsDigit(rs[i-1]) && !unicode.IsMark(rs[i-1]) {
				return false // two in a row
			}
		case sign(r):
		default:
			return false // anything else inside a word: a stray mark
		}
	}
	if letters > 0 && digits > 0 {
		// 12A, 4-A, 1st, 21st, 3rd, 80s: digits first, few letters after
		first := strings.IndexFunc(s, unicode.IsLetter)
		lastDigit := strings.LastIndexFunc(s, unicode.IsDigit)
		return first > lastDigit && letters <= 3
	}
	return true
}

// scripts are the writing systems words are told apart by.
var scripts = []*unicode.RangeTable{unicode.Latin, unicode.Cyrillic, unicode.Greek, unicode.Arabic,
	unicode.Hebrew, unicode.Devanagari, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul,
	unicode.Thai, unicode.Armenian, unicode.Georgian, unicode.Syriac, unicode.Ethiopic, unicode.Bengali, unicode.Tamil}

func scriptOf(r rune) *unicode.RangeTable {
	for _, t := range scripts {
		if unicode.Is(t, r) {
			if t == unicode.Hiragana || t == unicode.Katakana { // Japanese mixes them with Han
				return unicode.Han
			}
			return t
		}
	}
	return nil
}

// PageText is a page's own text layer and how it reads.
type PageText struct {
	Page    int
	Quality TextQuality
	Scan    bool // the page is a scan OCR can read: its text layer can be read again
}

// GarbledBar is the score below which a scan's text layer looks garbled.
// Printed text scores 1 (a stray sign or two in a long script: 0.99), a
// layer a poor OCR made with the scan less: a scanned book this was
// measured on (stray marks of the scan read as | _ — : at the lines'
// ends, words of one line at two heights) scores 0.94.
const GarbledBar = 0.97

// minWords is how many words a page needs to be judged at all.
const minWords = 20

// Garbled says if a page's text layer looks garbled: a scan whose text
// scores below GarbledBar.
func (pt PageText) Garbled() bool {
	return pt.Scan && pt.Quality.Words >= minWords && pt.Quality.Score() < GarbledBar
}

// TextLayers are, for every page of the PDF file at path, how its text
// layer reads and whether it is a scan.
func TextLayers(path string) (pages []PageText, err error) {
	err = eachPage(path, func(i int, p pdf.Page) {
		pages = append(pages, PageText{Page: i, Scan: isScan(p), Quality: qualityOf(lines(i, p))})
	})
	return pages, err
}

// GarbledPages counts the scanned pages of the PDF file at path whose
// own text layer looks garbled (a poor OCR's, made with the scan), and
// all its pages, without reading them, so a program can offer to read
// them with OCR instead (Options.OCRText).
func GarbledPages(path string) (garbled, pages int, err error) {
	layers, err := TextLayers(path)
	if err != nil {
		return 0, 0, err
	}
	return CountGarbled(layers), len(layers), nil
}

// CountGarbled counts the pages whose text layer looks garbled, but only
// if the scans' text layers, all together, score below GarbledBar: a
// page or two of a good layer that score low (a list of names) are not
// worth reading again.
func CountGarbled(layers []PageText) (garbled int) {
	var all TextQuality
	for _, pt := range layers {
		if pt.Scan {
			all.Words += pt.Quality.Words
			all.Odd += pt.Quality.Odd
			all.Lines += pt.Quality.Lines
			all.Split += pt.Quality.Split
		}
		if pt.Garbled() {
			garbled++
		}
	}
	if all.Words < minWords || all.Score() >= GarbledBar {
		return 0
	}
	return garbled
}

// isScan says if a page is a scan OCR can read: an image about the
// page's shape, at 100 dots an inch or more (not a logo on a title
// page), that can be decoded.
func isScan(p pdf.Page) bool {
	x := scanOf(p)
	if !readable(x) {
		return false
	}
	box := p.V.Key("MediaBox")
	if box.Len() != 4 {
		return false
	}
	pw, ph := box.Index(2).Float64()-box.Index(0).Float64(), box.Index(3).Float64()-box.Index(1).Float64()
	iw, ih := float64(x.Key("Width").Int64()), float64(x.Key("Height").Int64())
	if pw <= 0 || ph <= 0 || iw <= 0 || ih <= 0 {
		return false
	}
	shape := (ih / iw) / (ph / pw)
	return shape > 0.9 && shape < 1.1 && iw/(pw/72) >= 100
}
