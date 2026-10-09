// Copyright 2012 Mozilla Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Ported from pdf.js src/core/jbig2.js: generic region and generic
// refinement decoding, MMR bitmaps.

package jbig2

import (
	"fmt"
	"sort"
)

// jbig2Error is raised (panicked) inside the decoder and turned into a
// returned error by Decode.
type jbig2Error struct{ msg string }

func (e *jbig2Error) Error() string { return "jbig2: " + e.msg }

func fail(format string, args ...any) {
	panic(&jbig2Error{fmt.Sprintf(format, args...)})
}

// bitmap is a bi-level image, one byte (0 or 1) per pixel; 1 is black.
// Rows may share memory (typical prediction duplicates rows, symbols are
// cut out of collective bitmaps), so bitmaps are treated as read-only once
// decoded.
type bitmap struct {
	w, h int
	rows [][]byte
}

func newBitmap(w, h int) *bitmap {
	checkSize(w, h)
	b := &bitmap{w: w, h: h, rows: make([][]byte, h)}
	buf := make([]byte, w*h)
	for i := range b.rows {
		b.rows[i] = buf[i*w : (i+1)*w : (i+1)*w]
	}
	return b
}

// maxPixels bounds the size of any bitmap, against corrupt data; checkSize
// also bounds the width and the height.
const maxPixels = 1 << 28

func checkSize(w, h int) {
	if w < 0 || h < 0 || w > 1<<24 || h > 1<<20 || (w > 0 && h > maxPixels/w) {
		fail("invalid bitmap size %dx%d", w, h)
	}
}

// pixel returns the pixel at (x, y), 0 outside the bitmap.
func (b *bitmap) pixel(x, y int) byte {
	if y < 0 || y >= len(b.rows) || x < 0 || x >= len(b.rows[y]) {
		return 0
	}
	return b.rows[y][x]
}

type point struct{ x, y int }

var codingTemplates = [4][]point{
	{{-1, -2}, {0, -2}, {1, -2}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {2, -1}, {-4, 0}, {-3, 0}, {-2, 0}, {-1, 0}},
	{{-1, -2}, {0, -2}, {1, -2}, {2, -2}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {2, -1}, {-3, 0}, {-2, 0}, {-1, 0}},
	{{-1, -2}, {0, -2}, {1, -2}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {-2, 0}, {-1, 0}},
	{{-3, -1}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {-4, 0}, {-3, 0}, {-2, 0}, {-1, 0}},
}

var refinementTemplates = [2]struct{ coding, reference []point }{
	{
		coding:    []point{{0, -1}, {1, -1}, {-1, 0}},
		reference: []point{{0, -1}, {1, -1}, {-1, 0}, {0, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}},
	},
	{
		coding:    []point{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}},
		reference: []point{{0, -1}, {-1, 0}, {0, 0}, {1, 0}, {0, 1}, {1, 1}},
	},
}

// See 6.2.5.7 Decoding the bitmap.
var reusedContexts = [4]int{
	0x9b25, // 10011 0110010 0101
	0x0795, // 0011 110010 101
	0x00e5, // 001 11001 01
	0x0195, // 011001 0101
}

var refinementReusedContexts = [2]int{
	0x0020, // '000' + '0' (coding) + '00010000' + '0' (reference)
	0x0008, // '0000' + '001000'
}

// decodeBitmapTemplate0 is the generic region decoding for template 0 with
// the nominal adaptive pixels, without TPGDON or skip.
func decodeBitmapTemplate0(width, height int, dc *decodingContext) *bitmap {
	decoder := dc.decoder()
	contexts := dc.cache.genericContexts()
	bm := newBitmap(width, height)

	// ...ooooo....
	// ..ooooooo... Context template for current pixel (X)
	// .ooooX...... (concatenate values of 'o'-pixels to get contextLabel)
	const oldPixelMask = 0x7bf7 // 01111 0111111 0111

	for i := 0; i < height; i++ {
		row := bm.rows[i]
		row1, row2 := row, row
		if i >= 1 {
			row1 = bm.rows[i-1]
		}
		if i >= 2 {
			row2 = bm.rows[i-2]
		}
		// pdf.js reads the (still empty) current row for missing rows.
		get := func(r []byte, j int) int {
			if j < len(r) {
				return int(r[j])
			}
			return 0
		}
		contextLabel := get(row2, 0)<<13 | get(row2, 1)<<12 | get(row2, 2)<<11 |
			get(row1, 0)<<7 | get(row1, 1)<<6 | get(row1, 2)<<5 | get(row1, 3)<<4

		for j := 0; j < width; j++ {
			pixel := decoder.readBit(contexts, contextLabel)
			row[j] = byte(pixel)
			contextLabel = (contextLabel&oldPixelMask)<<1 | pixel
			if j+3 < width {
				contextLabel |= int(row2[j+3]) << 11
			}
			if j+4 < width {
				contextLabel |= int(row1[j+4]) << 4
			}
		}
	}
	return bm
}

// decodeBitmap is the generic region decoding procedure (6.2).
func decodeBitmap(mmr bool, width, height, templateIndex int, prediction bool,
	skip *bitmap, at []point, dc *decodingContext) *bitmap {
	checkSize(width, height)
	if mmr {
		input := newReader(dc.data, dc.start, dc.end)
		return decodeMMRBitmap(input, width, height, false)
	}
	if templateIndex < 0 || templateIndex > 3 {
		fail("invalid generic region template %d", templateIndex)
	}

	// Use optimized version for the most common case
	if templateIndex == 0 && skip == nil && !prediction && len(at) == 4 &&
		at[0] == (point{3, -1}) && at[1] == (point{-3, -1}) &&
		at[2] == (point{2, -2}) && at[3] == (point{-2, -2}) {
		return decodeBitmapTemplate0(width, height, dc)
	}
	return decodeBitmapGeneric(width, height, templateIndex, prediction, skip, at, dc)
}

// decodeBitmapGeneric is decodeBitmap for any template.
func decodeBitmapGeneric(width, height, templateIndex int, prediction bool,
	skip *bitmap, at []point, dc *decodingContext) *bitmap {
	template := append(append([]point(nil), codingTemplates[templateIndex]...), at...)

	// Sorting is non-standard, and it is not required. But sorting increases
	// the number of template bits that can be reused from the previous
	// contextLabel in the main loop.
	sort.SliceStable(template, func(a, b int) bool {
		if template[a].y != template[b].y {
			return template[a].y < template[b].y
		}
		return template[a].x < template[b].x
	})

	templateLength := len(template)
	if templateLength > 16 {
		fail("too many template pixels")
	}
	templateX := make([]int, templateLength)
	templateY := make([]int, templateLength)
	var changingTemplateEntries []int
	reuseMask, minX, maxX, minY := 0, 0, 0, 0

	for k := 0; k < templateLength; k++ {
		templateX[k] = template[k].x
		templateY[k] = template[k].y
		minX = min(minX, template[k].x)
		maxX = max(maxX, template[k].x)
		minY = min(minY, template[k].y)
		// Check if the template pixel appears in two consecutive context
		// labels, so it can be reused. Otherwise, we add it to the list of
		// changing template entries.
		if k < templateLength-1 && template[k].y == template[k+1].y &&
			template[k].x == template[k+1].x-1 {
			reuseMask |= 1 << (templateLength - 1 - k)
		} else {
			changingTemplateEntries = append(changingTemplateEntries, k)
		}
	}
	changingEntriesLength := len(changingTemplateEntries)
	changingTemplateX := make([]int, changingEntriesLength)
	changingTemplateY := make([]int, changingEntriesLength)
	changingTemplateBit := make([]int, changingEntriesLength)
	for c, k := range changingTemplateEntries {
		changingTemplateX[c] = template[k].x
		changingTemplateY[c] = template[k].y
		changingTemplateBit[c] = 1 << (templateLength - 1 - k)
	}

	// Get the safe bounding box edges from the width, height, minX, maxX, minY
	sbbLeft := -minX
	sbbTop := -minY
	sbbRight := width - maxX

	pseudoPixelContext := reusedContexts[templateIndex]
	row := make([]byte, width)
	bm := &bitmap{w: width, h: height, rows: make([][]byte, 0, height)}

	decoder := dc.decoder()
	contexts := dc.cache.genericContexts()

	// get reads a decoded pixel; rows below the current one (only possible
	// with invalid adaptive pixels) read as 0.
	get := func(i0, j0 int) int {
		if i0 < len(bm.rows) {
			return int(bm.rows[i0][j0])
		}
		return 0
	}

	ltp := 0
	contextLabel := 0
	for i := 0; i < height; i++ {
		if prediction {
			sltp := decoder.readBit(contexts, pseudoPixelContext)
			ltp ^= sltp
			if ltp != 0 {
				bm.rows = append(bm.rows, row) // duplicate previous row
				continue
			}
		}
		row = append([]byte(nil), row...)
		bm.rows = append(bm.rows, row)
		for j := 0; j < width; j++ {
			if skip != nil && skip.pixel(j, i) != 0 {
				row[j] = 0
				continue
			}
			// Are we in the middle of a scanline, so we can reuse
			// contextLabel bits?
			if j >= sbbLeft && j < sbbRight && i >= sbbTop {
				// If yes, we can just shift the bits that are reusable and
				// only fetch the remaining ones.
				contextLabel = (contextLabel << 1) & reuseMask
				for k := 0; k < changingEntriesLength; k++ {
					if get(i+changingTemplateY[k], j+changingTemplateX[k]) != 0 {
						contextLabel |= changingTemplateBit[k]
					}
				}
			} else {
				// compute the contextLabel from scratch
				contextLabel = 0
				shift := templateLength - 1
				for k := 0; k < templateLength; k, shift = k+1, shift-1 {
					j0 := j + templateX[k]
					if j0 >= 0 && j0 < width {
						i0 := i + templateY[k]
						if i0 >= 0 && get(i0, j0) != 0 {
							contextLabel |= 1 << shift
						}
					}
				}
			}
			row[j] = byte(decoder.readBit(contexts, contextLabel))
		}
	}
	return bm
}

// decodeRefinement is the generic refinement region decoding procedure
// (6.3.2).
func decodeRefinement(width, height, templateIndex int, reference *bitmap,
	offsetX, offsetY int, prediction bool, at []point, dc *decodingContext) *bitmap {
	checkSize(width, height)
	if templateIndex < 0 || templateIndex > 1 {
		fail("invalid refinement template %d", templateIndex)
	}
	if reference == nil {
		fail("missing reference bitmap for refinement")
	}
	codingTemplate := refinementTemplates[templateIndex].coding
	referenceTemplate := refinementTemplates[templateIndex].reference
	if templateIndex == 0 {
		if len(at) < 2 {
			fail("missing refinement adaptive pixels")
		}
		codingTemplate = append(append([]point(nil), codingTemplate...), at[0])
		referenceTemplate = append(append([]point(nil), referenceTemplate...), at[1])
	}
	referenceWidth, referenceHeight := reference.w, reference.h

	pseudoPixelContext := refinementReusedContexts[templateIndex]
	bm := newBitmap(width, height)

	decoder := dc.decoder()
	contexts := dc.cache.refinementContexts()

	// typical returns whether the 3x3 reference pixels around the pixel
	// matching (j, i) all have the same value, and that value (6.3.5.6,
	// TPGRON; pdf.js does not support it).
	typical := func(i, j int) (byte, bool) {
		v := reference.pixel(j-offsetX-1, i-offsetY-1)
		for y := -1; y <= 1; y++ {
			for x := -1; x <= 1; x++ {
				if reference.pixel(j-offsetX+x, i-offsetY+y) != v {
					return 0, false
				}
			}
		}
		return v, true
	}

	ltp := 0
	for i := 0; i < height; i++ {
		if prediction {
			sltp := decoder.readBit(contexts, pseudoPixelContext)
			ltp ^= sltp
		}
		row := bm.rows[i]
		for j := 0; j < width; j++ {
			if ltp != 0 {
				if v, ok := typical(i, j); ok {
					row[j] = v
					continue
				}
			}
			contextLabel := 0
			for _, p := range codingTemplate {
				i0, j0 := i+p.y, j+p.x
				if i0 < 0 || j0 < 0 || j0 >= width {
					contextLabel <<= 1 // out of bound pixel
				} else {
					contextLabel = contextLabel<<1 | int(bm.pixel(j0, i0))
				}
			}
			for _, p := range referenceTemplate {
				i0, j0 := i+p.y-offsetY, j+p.x-offsetX
				if i0 < 0 || i0 >= referenceHeight || j0 < 0 || j0 >= referenceWidth {
					contextLabel <<= 1 // out of bound pixel
				} else {
					contextLabel = contextLabel<<1 | int(reference.rows[i0][j0])
				}
			}
			row[j] = byte(decoder.readBit(contexts, contextLabel))
		}
	}
	return bm
}

// readUncompressedBitmap reads a bitmap stored as plain bits, each row
// byte aligned.
func readUncompressedBitmap(r *reader, width, height int) *bitmap {
	bm := newBitmap(width, height)
	for y := 0; y < height; y++ {
		row := bm.rows[y]
		for x := 0; x < width; x++ {
			row[x] = byte(r.readBit())
		}
		r.byteAlign()
	}
	return bm
}

// decodeMMRBitmap decodes an MMR (CCITT group 4) coded bitmap.
func decodeMMRBitmap(input *reader, width, height int, endOfBlock bool) *bitmap {
	checkSize(width, height)
	// MMR is the same compression algorithm as the PDF filter
	// CCITTFaxDecode with /K -1.
	decoder := newCCITTDecoder(input, ccittOptions{
		K:          -1,
		Columns:    width,
		Rows:       height,
		BlackIs1:   true,
		EndOfBlock: endOfBlock,
	})
	bm := newBitmap(width, height)
	eof := false
	currentByte := 0
	for y := 0; y < height; y++ {
		row := bm.rows[y]
		shift := -1
		for x := 0; x < width; x++ {
			if shift < 0 {
				currentByte = decoder.readNextChar()
				if currentByte == -1 {
					// Set the rest of the bits to zero.
					currentByte = 0
					eof = true
				}
				shift = 7
			}
			row[x] = byte(currentByte>>shift) & 1
			shift--
		}
	}

	if endOfBlock && !eof {
		// Read until EOFB has been consumed.
		const lookForEOFLimit = 5
		for i := 0; i < lookForEOFLimit; i++ {
			if decoder.readNextChar() == -1 {
				break
			}
		}
	}
	return bm
}
