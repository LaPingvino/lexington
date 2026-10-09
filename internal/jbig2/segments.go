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

// Ported from pdf.js src/core/jbig2.js: segment parsing and the page
// buffer (SimpleSegmentVisitor).

package jbig2

func byteAt(data []byte, i int) byte {
	if i < 0 || i >= len(data) {
		fail("unexpected end of data")
	}
	return data[i]
}

func readUint16(data []byte, i int) int {
	return int(byteAt(data, i))<<8 | int(byteAt(data, i+1))
}

func readUint32(data []byte, i int) uint32 {
	return uint32(byteAt(data, i))<<24 | uint32(byteAt(data, i+1))<<16 |
		uint32(byteAt(data, i+2))<<8 | uint32(byteAt(data, i+3))
}

func readInt8(data []byte, i int) int {
	return int(int8(byteAt(data, i)))
}

// 7.3 Segment types
var segmentTypes = map[int]string{
	0:  "SymbolDictionary",
	4:  "IntermediateTextRegion",
	6:  "ImmediateTextRegion",
	7:  "ImmediateLosslessTextRegion",
	16: "PatternDictionary",
	20: "IntermediateHalftoneRegion",
	22: "ImmediateHalftoneRegion",
	23: "ImmediateLosslessHalftoneRegion",
	36: "IntermediateGenericRegion",
	38: "ImmediateGenericRegion",
	39: "ImmediateLosslessGenericRegion",
	40: "IntermediateGenericRefinementRegion",
	42: "ImmediateGenericRefinementRegion",
	43: "ImmediateLosslessGenericRefinementRegion",
	48: "PageInformation",
	49: "EndOfPage",
	50: "EndOfStripe",
	51: "EndOfFile",
	52: "Profiles",
	53: "Tables",
	62: "Extension",
}

type segmentHeader struct {
	number            uint32
	typ               int
	deferredNonRetain bool
	retainBits        []byte
	referredTo        []uint32
	pageAssociation   uint32
	length            int
	unknownLength     bool // length was 0xffffffff, data ends with rowCount
	rowCount          int
	headerEnd         int
}

// 7.4.1 Region segment information field
type regionInfo struct {
	width, height       int
	x, y                int
	combinationOperator int
}

const regionSegmentInformationFieldLength = 17

func readRegionSegmentInformation(data []byte, start int) regionInfo {
	return regionInfo{
		width:               int(readUint32(data, start)),
		height:              int(readUint32(data, start+4)),
		x:                   int(readUint32(data, start+8)),
		y:                   int(readUint32(data, start+12)),
		combinationOperator: int(byteAt(data, start+16) & 7),
	}
}

func readSegmentHeader(data []byte, start int) *segmentHeader {
	h := &segmentHeader{number: readUint32(data, start)}
	flags := byteAt(data, start+4)
	segmentType := int(flags & 0x3f)
	if _, ok := segmentTypes[segmentType]; !ok {
		fail("invalid segment type: %d", segmentType)
	}
	h.typ = segmentType
	h.deferredNonRetain = flags&0x80 != 0

	pageAssociationFieldSize := flags&0x40 != 0
	referredFlags := byteAt(data, start+5)
	referredToCount := int(referredFlags>>5) & 7
	h.retainBits = []byte{referredFlags & 31}
	position := start + 6
	switch referredToCount {
	case 7:
		// Long form (7.2.4). pdf.js tests the whole byte against 7 here,
		// which never matches a long form count; this follows the spec.
		referredToCount = int(readUint32(data, position-1) & 0x1fffffff)
		position += 3
		bytes := (referredToCount + 8) >> 3 // count+1 retention bits
		h.retainBits = h.retainBits[:0]
		for ; bytes > 0; bytes-- {
			h.retainBits = append(h.retainBits, byteAt(data, position))
			position++
		}
	case 5, 6:
		fail("invalid referred-to flags")
	}

	referredToSegmentNumberSize := 4
	if h.number <= 256 {
		referredToSegmentNumberSize = 1
	} else if h.number <= 65536 {
		referredToSegmentNumberSize = 2
	}
	if referredToCount*referredToSegmentNumberSize > len(data)-position {
		fail("invalid referred-to segment count")
	}
	for i := 0; i < referredToCount; i++ {
		var number uint32
		switch referredToSegmentNumberSize {
		case 1:
			number = uint32(byteAt(data, position))
		case 2:
			number = uint32(readUint16(data, position))
		default:
			number = readUint32(data, position)
		}
		h.referredTo = append(h.referredTo, number)
		position += referredToSegmentNumberSize
	}
	if !pageAssociationFieldSize {
		h.pageAssociation = uint32(byteAt(data, position))
		position++
	} else {
		h.pageAssociation = readUint32(data, position)
		position += 4
	}
	length := readUint32(data, position)
	position += 4

	if length == 0xffffffff {
		// 7.2.7 Segment data length, unknown segment length
		if segmentType != 38 { // ImmediateGenericRegion
			fail("invalid unknown segment length")
		}
		genericRegionInfo := readRegionSegmentInformation(data, position)
		genericRegionSegmentFlags := byteAt(data, position+regionSegmentInformationFieldLength)
		genericRegionMmr := genericRegionSegmentFlags&1 != 0
		// Searching for the segment end: 0xffac (arithmetic) or 0x0000
		// (MMR), then the row count (7.4.6.4). pdf.js searches for the
		// region's height as row count, which fails when the height is
		// unknown too; the row count is the real height of the region.
		marker := [2]byte{0xff, 0xac}
		if genericRegionMmr {
			marker = [2]byte{0, 0}
		}
		found := false
		for i := position + regionSegmentInformationFieldLength + 1; i+6 <= len(data); i++ {
			if data[i] != marker[0] || data[i+1] != marker[1] {
				continue
			}
			rowCount := readUint32(data, i+2)
			if genericRegionMmr && genericRegionInfo.height != 0xffffffff &&
				int(rowCount) > genericRegionInfo.height {
				continue
			}
			h.length = i + 6 - position
			h.unknownLength = true
			h.rowCount = int(rowCount)
			found = true
			break
		}
		if !found {
			fail("segment end was not found")
		}
	} else {
		h.length = int(length)
	}
	h.headerEnd = position
	return h
}

type segment struct {
	header     *segmentHeader
	data       []byte
	start, end int
}

// readSegments reads the segments of data[start:end]. Data embedded in PDF
// is organised sequentially; standalone files may use random access, where
// all segment headers come first.
func readSegments(randomAccess bool, data []byte, start, end int) []*segment {
	var segments []*segment
	position := start
	for position < end {
		h := readSegmentHeader(data, position)
		position = h.headerEnd
		s := &segment{header: h, data: data}
		if !randomAccess {
			s.start = position
			position += h.length
			s.end = position
		}
		segments = append(segments, s)
		if h.typ == 51 {
			break // end of file is found
		}
	}
	if randomAccess {
		for _, s := range segments {
			s.start = position
			position += s.header.length
			s.end = position
		}
	}
	for _, s := range segments {
		if s.end > len(data) {
			s.end = len(data) // truncated data: decode what there is
		}
		if s.start > s.end {
			s.start = s.end
		}
	}
	return segments
}

type symbolDictionary struct {
	huffman                      bool
	refinement                   bool
	huffmanDHSelector            int
	huffmanDWSelector            int
	bitmapSizeSelector           int
	aggregationInstancesSelector int
	bitmapCodingContextUsed      bool
	bitmapCodingContextRetained  bool
	template                     int
	refinementTemplate           int
	at                           []point
	refinementAt                 []point
	numberOfExportedSymbols      int
	numberOfNewSymbols           int
}

type textRegion struct {
	info                          regionInfo
	huffman                       bool
	refinement                    bool
	logStripSize                  int
	stripSize                     int
	referenceCorner               int
	transposed                    bool
	combinationOperator           int
	defaultPixelValue             int
	dsOffset                      int
	refinementTemplate            int
	huffmanFS                     int
	huffmanDS                     int
	huffmanDT                     int
	huffmanRefinementDW           int
	huffmanRefinementDH           int
	huffmanRefinementDX           int
	huffmanRefinementDY           int
	huffmanRefinementSizeSelector bool
	refinementAt                  []point
	numberOfSymbolInstances       int
}

type patternDictionary struct {
	mmr             bool
	template        int
	patternWidth    int
	patternHeight   int
	maxPatternIndex int
}

type halftoneRegion struct {
	info                regionInfo
	mmr                 bool
	template            int
	enableSkip          bool
	combinationOperator int
	defaultPixelValue   int
	gridWidth           int
	gridHeight          int
	gridOffsetX         int
	gridOffsetY         int
	gridVectorX         int
	gridVectorY         int
}

type genericRegion struct {
	info       regionInfo
	mmr        bool
	template   int
	prediction bool
	at         []point
}

type refinementRegion struct {
	info       regionInfo
	template   int
	prediction bool
	at         []point
}

// intermediateRegion is the result of an intermediate region segment,
// waiting to be refined.
type intermediateRegion struct {
	info regionInfo
	bm   *bitmap
}

type pageInformation struct {
	width, height               int
	heightUnknown               bool
	resolutionX, resolutionY    uint32
	lossless                    bool
	refinement                  bool
	defaultPixelValue           int
	combinationOperator         int
	requiresBuffer              bool
	combinationOperatorOverride bool
}

func readAt(data []byte, position, n int) []point {
	at := make([]point, n)
	for i := range at {
		at[i] = point{readInt8(data, position), readInt8(data, position+1)}
		position += 2
	}
	return at
}

// processSegment parses a segment's data header and hands it to the page.
func (p *page) processSegment(s *segment) {
	h := s.header
	data, end := s.data, s.end
	position := s.start
	switch h.typ {
	case 0: // SymbolDictionary
		// 7.4.2 Symbol dictionary segment syntax
		d := &symbolDictionary{}
		dictionaryFlags := readUint16(data, position) // 7.4.2.1.1
		d.huffman = dictionaryFlags&1 != 0
		d.refinement = dictionaryFlags&2 != 0
		d.huffmanDHSelector = (dictionaryFlags >> 2) & 3
		d.huffmanDWSelector = (dictionaryFlags >> 4) & 3
		d.bitmapSizeSelector = (dictionaryFlags >> 6) & 1
		d.aggregationInstancesSelector = (dictionaryFlags >> 7) & 1
		d.bitmapCodingContextUsed = dictionaryFlags&256 != 0
		d.bitmapCodingContextRetained = dictionaryFlags&512 != 0
		d.template = (dictionaryFlags >> 10) & 3
		d.refinementTemplate = (dictionaryFlags >> 12) & 1
		position += 2
		if !d.huffman {
			atLength := 1
			if d.template == 0 {
				atLength = 4
			}
			d.at = readAt(data, position, atLength)
			position += 2 * atLength
		}
		if d.refinement && d.refinementTemplate == 0 {
			d.refinementAt = readAt(data, position, 2)
			position += 4
		}
		d.numberOfExportedSymbols = int(readUint32(data, position))
		position += 4
		d.numberOfNewSymbols = int(readUint32(data, position))
		position += 4
		p.onSymbolDictionary(d, h, data, position, end)

	case 4, 6, 7: // IntermediateTextRegion (not in pdf.js), ImmediateTextRegion, ImmediateLosslessTextRegion
		tr := &textRegion{}
		tr.info = readRegionSegmentInformation(data, position)
		position += regionSegmentInformationFieldLength
		flags := readUint16(data, position)
		position += 2
		tr.huffman = flags&1 != 0
		tr.refinement = flags&2 != 0
		tr.logStripSize = (flags >> 2) & 3
		tr.stripSize = 1 << tr.logStripSize
		tr.referenceCorner = (flags >> 4) & 3
		tr.transposed = flags&64 != 0
		tr.combinationOperator = (flags >> 7) & 3
		tr.defaultPixelValue = (flags >> 9) & 1
		tr.dsOffset = int(int32(uint32(flags)<<17) >> 27)
		tr.refinementTemplate = (flags >> 15) & 1
		if tr.huffman {
			hf := readUint16(data, position)
			position += 2
			tr.huffmanFS = hf & 3
			tr.huffmanDS = (hf >> 2) & 3
			tr.huffmanDT = (hf >> 4) & 3
			tr.huffmanRefinementDW = (hf >> 6) & 3
			tr.huffmanRefinementDH = (hf >> 8) & 3
			tr.huffmanRefinementDX = (hf >> 10) & 3
			tr.huffmanRefinementDY = (hf >> 12) & 3
			tr.huffmanRefinementSizeSelector = hf&0x4000 != 0
		}
		if tr.refinement && tr.refinementTemplate == 0 {
			tr.refinementAt = readAt(data, position, 2)
			position += 4
		}
		tr.numberOfSymbolInstances = int(readUint32(data, position))
		position += 4
		p.onTextRegion(tr, h, data, position, end)

	case 16: // PatternDictionary
		// 7.4.4. Pattern dictionary segment syntax
		pd := &patternDictionary{}
		flags := byteAt(data, position)
		position++
		pd.mmr = flags&1 != 0
		pd.template = int(flags>>1) & 3
		pd.patternWidth = int(byteAt(data, position))
		position++
		pd.patternHeight = int(byteAt(data, position))
		position++
		pd.maxPatternIndex = int(readUint32(data, position))
		position += 4
		p.onPatternDictionary(pd, h.number, data, position, end)

	case 20, 22, 23: // IntermediateHalftoneRegion (not in pdf.js), ImmediateHalftoneRegion, ImmediateLosslessHalftoneRegion
		// 7.4.5 Halftone region segment syntax
		hr := &halftoneRegion{}
		hr.info = readRegionSegmentInformation(data, position)
		position += regionSegmentInformationFieldLength
		flags := byteAt(data, position)
		position++
		hr.mmr = flags&1 != 0
		hr.template = int(flags>>1) & 3
		hr.enableSkip = flags&8 != 0
		hr.combinationOperator = int(flags>>4) & 7
		hr.defaultPixelValue = int(flags>>7) & 1
		hr.gridWidth = int(readUint32(data, position))
		position += 4
		hr.gridHeight = int(readUint32(data, position))
		position += 4
		hr.gridOffsetX = int(int32(readUint32(data, position)))
		position += 4
		hr.gridOffsetY = int(int32(readUint32(data, position)))
		position += 4
		hr.gridVectorX = readUint16(data, position)
		position += 2
		hr.gridVectorY = readUint16(data, position)
		position += 2
		p.onHalftoneRegion(hr, h, data, position, end)

	case 36, 38, 39: // IntermediateGenericRegion (not in pdf.js), ImmediateGenericRegion, ImmediateLosslessGenericRegion
		gr := &genericRegion{}
		gr.info = readRegionSegmentInformation(data, position)
		position += regionSegmentInformationFieldLength
		flags := byteAt(data, position)
		position++
		gr.mmr = flags&1 != 0
		gr.template = int(flags>>1) & 3
		gr.prediction = flags&8 != 0
		if flags&0x30 != 0 {
			fail("extended templates and colour are not supported")
		}
		if !gr.mmr {
			atLength := 1
			if gr.template == 0 {
				atLength = 4
			}
			gr.at = readAt(data, position, atLength)
			position += 2 * atLength
		}
		if h.unknownLength && h.rowCount < gr.info.height {
			gr.info.height = h.rowCount
		}
		p.onGenericRegion(gr, h, data, position, end)

	case 40, 42, 43: // Intermediate, Immediate and ImmediateLossless
		// GenericRefinementRegion (7.4.7; not in pdf.js)
		rr := &refinementRegion{}
		rr.info = readRegionSegmentInformation(data, position)
		position += regionSegmentInformationFieldLength
		flags := byteAt(data, position)
		position++
		rr.template = int(flags & 1)
		rr.prediction = flags&2 != 0
		if rr.template == 0 {
			rr.at = readAt(data, position, 2)
			position += 4
		}
		p.onRefinementRegion(rr, h, data, position, end)

	case 48: // PageInformation
		info := &pageInformation{
			width:       int(readUint32(data, position)),
			resolutionX: readUint32(data, position+8),
			resolutionY: readUint32(data, position+12),
		}
		if height := readUint32(data, position+4); height == 0xffffffff {
			info.heightUnknown = true
		} else {
			info.height = int(height)
		}
		flags := byteAt(data, position+16)
		readUint16(data, position+17) // pageStripingInformation
		info.lossless = flags&1 != 0
		info.refinement = flags&2 != 0
		info.defaultPixelValue = int(flags>>2) & 1
		info.combinationOperator = int(flags>>3) & 3
		info.requiresBuffer = flags&32 != 0
		info.combinationOperatorOverride = flags&64 != 0
		p.onPageInformation(info)

	case 49: // EndOfPage
	case 50: // EndOfStripe
		// 7.4.10: the row number of the last row of the stripe.
		if end-position >= 4 {
			p.onEndOfStripe(int(readUint32(data, position)))
		}
	case 51: // EndOfFile
	case 52: // Profiles: informative only (pdf.js rejects them).
	case 53: // Tables
		p.onTables(h.number, data, position, end)
	case 62: // 7.4.15 defines 2 extension types which are comments and can
		// be ignored.
	default:
		fail("segment type %s(%d) is not implemented", segmentTypes[h.typ], h.typ)
	}
}

// page is pdf.js's SimpleSegmentVisitor: it holds the page buffer and the
// dictionaries and tables defined so far.
type page struct {
	info         *pageInformation
	width        int
	height       int // rows allocated
	growable     bool
	buf          []byte // one byte per pixel, 1 is black
	defaultPixel byte

	// fallback size, from the PDF image dictionary
	pdfWidth, pdfHeight int

	symbols      map[uint32][]*bitmap
	patterns     map[uint32][]*bitmap
	customTables map[uint32]*huffmanTable
	// retained arithmetic contexts of symbol dictionaries (7.4.2.2)
	retained map[uint32]*contextCache
	// intermediate region results (not in pdf.js)
	regions map[uint32]*intermediateRegion
}

func (p *page) onPageInformation(info *pageInformation) {
	p.info = info
	width, height := info.width, info.height
	p.growable = false
	if info.heightUnknown {
		// The height comes from end of stripe segments; start with the
		// height of the PDF image (pdf.js gives up on such pages).
		height = p.pdfHeight
		p.growable = true
	}
	// Only the part the PDF image shows is kept: Decode crops to it, and
	// corrupt page sizes then cost no memory.
	if p.pdfWidth > 0 {
		width = min(width, p.pdfWidth)
	}
	if p.pdfHeight > 0 && !p.growable {
		height = min(height, p.pdfHeight)
	}
	checkSize(width, height)
	p.width, p.height = width, height
	p.buf = make([]byte, width*height)
	p.defaultPixel = byte(info.defaultPixelValue)
	if p.defaultPixel != 0 {
		for i := range p.buf {
			p.buf[i] = 1
		}
	}
}

// ensurePage makes a page buffer when a region comes before any page
// information, with the size of the PDF image.
func (p *page) ensurePage() {
	if p.info == nil {
		p.onPageInformation(&pageInformation{width: p.pdfWidth, height: p.pdfHeight})
	}
}

// growTo makes a page of unknown height at least rows high.
func (p *page) growTo(rows int) {
	if p.pdfHeight > 0 {
		rows = min(rows, p.pdfHeight) // the rest would be cropped
	}
	if !p.growable || rows <= p.height {
		return
	}
	checkSize(p.width, rows)
	extra := make([]byte, p.width*(rows-p.height))
	if p.defaultPixel != 0 {
		for i := range extra {
			extra[i] = 1
		}
	}
	p.buf = append(p.buf, extra...)
	p.height = rows
}

func (p *page) onEndOfStripe(lastRow int) {
	if p.info != nil && p.info.heightUnknown && lastRow >= 0 && lastRow < maxPixels {
		p.growTo(lastRow + 1)
	}
}

// drawBitmap combines a region bitmap into the page, clipped to the page.
func (p *page) drawBitmap(ri regionInfo, bm *bitmap) {
	p.ensurePage()
	if ri.y+ri.height > p.height && ri.height < maxPixels && ri.y < maxPixels {
		p.growTo(ri.y + ri.height)
	}
	combinationOperator := p.info.combinationOperator
	if p.info.combinationOperatorOverride {
		combinationOperator = ri.combinationOperator
	}
	if combinationOperator > 4 {
		fail("operator %d is not supported", combinationOperator)
	}
	for i := 0; i < ri.height && i < bm.h; i++ {
		y := ri.y + i
		if y < 0 || y >= p.height {
			continue
		}
		dst := p.buf[y*p.width : (y+1)*p.width]
		src := bm.rows[i]
		for j := 0; j < ri.width && j < len(src); j++ {
			x := ri.x + j
			if x < 0 || x >= p.width {
				continue
			}
			switch combinationOperator {
			case 0: // OR
				dst[x] |= src[j]
			case 2: // XOR
				dst[x] ^= src[j]
			default:
				combine(&dst[x], src[j], combinationOperator)
			}
		}
	}
}

// referredSymbols combines the exported symbols of the referred segments.
func (p *page) referredSymbols(referredSegments []uint32) []*bitmap {
	var inputSymbols []*bitmap
	for _, ref := range referredSegments {
		// nil when we have a reference to a Tables segment instead of a
		// SymbolDictionary.
		inputSymbols = append(inputSymbols, p.symbols[ref]...)
	}
	return inputSymbols
}

// storeOrDraw draws the result of an immediate region, and stores that of
// an intermediate region (segment types 4, 20, 36 and 40) for refinement.
func (p *page) storeOrDraw(h *segmentHeader, ri regionInfo, bm *bitmap) {
	switch h.typ {
	case 4, 20, 36, 40:
		if p.regions == nil {
			p.regions = map[uint32]*intermediateRegion{}
		}
		p.regions[h.number] = &intermediateRegion{ri, bm}
	default:
		p.drawBitmap(ri, bm)
	}
}

func (p *page) onGenericRegion(region *genericRegion, h *segmentHeader, data []byte, start, end int) {
	ri := region.info
	dc := newDecodingContext(data, start, end)
	bm := decodeBitmap(region.mmr, ri.width, ri.height, region.template,
		region.prediction, nil, region.at, dc)
	p.storeOrDraw(h, ri, bm)
}

// onRefinementRegion decodes a generic refinement region (7.4.7.5): it
// refines the intermediate region it refers to, or else the page.
func (p *page) onRefinementRegion(region *refinementRegion, h *segmentHeader, data []byte, start, end int) {
	ri := region.info
	var reference *bitmap
	if len(h.referredTo) > 0 {
		r := p.regions[h.referredTo[0]]
		if r == nil {
			fail("refinement of a missing region")
		}
		reference = r.bm
		delete(p.regions, h.referredTo[0])
	} else {
		p.ensurePage()
		checkSize(ri.width, ri.height)
		reference = newBitmap(ri.width, ri.height)
		for y := 0; y < ri.height; y++ {
			py := ri.y + y
			if py < 0 || py >= p.height {
				continue
			}
			for x := 0; x < ri.width; x++ {
				if px := ri.x + x; px >= 0 && px < p.width {
					reference.rows[y][x] = p.buf[py*p.width+px]
				}
			}
		}
	}
	dc := newDecodingContext(data, start, end)
	bm := decodeRefinement(ri.width, ri.height, region.template, reference, 0, 0,
		region.prediction, region.at, dc)
	p.storeOrDraw(h, ri, bm)
}

func (p *page) onSymbolDictionary(d *symbolDictionary, h *segmentHeader, data []byte, start, end int) {
	var huffmanTables *symbolDictionaryHuffmanTables
	var huffmanInput *reader
	if d.huffman {
		huffmanTables = getSymbolDictionaryHuffmanTables(d, h.referredTo, p.customTables)
		huffmanInput = newReader(data, start, end)
	}
	if p.symbols == nil {
		p.symbols = map[uint32][]*bitmap{}
	}
	inputSymbols := p.referredSymbols(h.referredTo)
	dc := newDecodingContext(data, start, end)
	if d.bitmapCodingContextUsed {
		// 7.4.2.2 (not in pdf.js): start with the generic and refinement
		// contexts retained by the last referred symbol dictionary.
		for i := len(h.referredTo) - 1; i >= 0; i-- {
			if c := p.retained[h.referredTo[i]]; c != nil {
				dc.cache.gb = append([]uint8(nil), c.gb...)
				dc.cache.gr = append([]uint8(nil), c.gr...)
				break
			}
		}
	}
	p.symbols[h.number] = decodeSymbolDictionary(d.huffman, d.refinement,
		inputSymbols, d.numberOfNewSymbols, d.numberOfExportedSymbols, huffmanTables,
		d.template, d.at, d.refinementTemplate, d.refinementAt, dc, huffmanInput)
	if d.bitmapCodingContextRetained {
		if p.retained == nil {
			p.retained = map[uint32]*contextCache{}
		}
		p.retained[h.number] = &contextCache{gb: dc.cache.gb, gr: dc.cache.gr}
	}
}

func (p *page) onTextRegion(region *textRegion, h *segmentHeader, data []byte, start, end int) {
	ri := region.info
	var huffmanTables *textRegionHuffmanTables
	var huffmanInput *reader

	inputSymbols := p.referredSymbols(h.referredTo)
	symbolCodeLength := log2(len(inputSymbols))
	if region.huffman {
		huffmanInput = newReader(data, start, end)
		huffmanTables = getTextRegionHuffmanTables(region, h.referredTo,
			p.customTables, len(inputSymbols), huffmanInput)
	}
	dc := newDecodingContext(data, start, end)
	bm := decodeTextRegion(region.huffman, region.refinement, ri.width, ri.height,
		region.defaultPixelValue, region.numberOfSymbolInstances, region.stripSize,
		inputSymbols, symbolCodeLength, region.transposed, region.dsOffset,
		region.referenceCorner, region.combinationOperator, huffmanTables,
		region.refinementTemplate, region.refinementAt, dc, region.logStripSize,
		huffmanInput)
	p.storeOrDraw(h, ri, bm)
}

func (p *page) onPatternDictionary(d *patternDictionary, currentSegment uint32,
	data []byte, start, end int) {
	if p.patterns == nil {
		p.patterns = map[uint32][]*bitmap{}
	}
	dc := newDecodingContext(data, start, end)
	p.patterns[currentSegment] = decodePatternDictionary(d.mmr, d.patternWidth,
		d.patternHeight, d.maxPatternIndex, d.template, dc)
}

func (p *page) onHalftoneRegion(region *halftoneRegion, h *segmentHeader, data []byte, start, end int) {
	// HalftoneRegion refers to exactly one PatternDictionary.
	if len(h.referredTo) == 0 {
		fail("halftone region without pattern dictionary")
	}
	patterns := p.patterns[h.referredTo[0]]
	ri := region.info
	dc := newDecodingContext(data, start, end)
	bm := decodeHalftoneRegion(region.mmr, patterns, region.template, ri.width, ri.height,
		region.defaultPixelValue, region.enableSkip, region.combinationOperator,
		region.gridWidth, region.gridHeight, region.gridOffsetX, region.gridOffsetY,
		region.gridVectorX, region.gridVectorY, dc)
	p.storeOrDraw(h, ri, bm)
}

func (p *page) onTables(currentSegment uint32, data []byte, start, end int) {
	if p.customTables == nil {
		p.customTables = map[uint32]*huffmanTable{}
	}
	p.customTables[currentSegment] = decodeTablesSegment(data, start, end)
}
