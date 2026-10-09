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

// Ported from pdf.js src/core/jbig2.js: Huffman tables (Annex B) and the
// bit reader.

package jbig2

import "sync"

// reader reads bits, most significant first, from data[start:end].
type reader struct {
	data        []byte
	start, end  int
	position    int
	shift       int
	currentByte byte
}

func newReader(data []byte, start, end int) *reader {
	if end > len(data) {
		end = len(data)
	}
	return &reader{data: data, start: start, end: end, position: start, shift: -1}
}

func (r *reader) readBit() int {
	if r.shift < 0 {
		if r.position >= r.end {
			fail("end of data while reading bit")
		}
		r.currentByte = r.data[r.position]
		r.position++
		r.shift = 7
	}
	bit := int(r.currentByte>>uint(r.shift)) & 1
	r.shift--
	return bit
}

// readBits reads numBits bits; as in pdf.js, 32 bits give a signed int32.
func (r *reader) readBits(numBits int) int {
	var result int32
	for i := numBits - 1; i >= 0; i-- {
		result |= int32(r.readBit()) << uint(i&31)
	}
	return int(result)
}

func (r *reader) byteAlign() { r.shift = -1 }

// next returns the next byte, or -1 at the end (for the CCITT decoder).
func (r *reader) next() int {
	if r.position >= r.end {
		return -1
	}
	b := r.data[r.position]
	r.position++
	return int(b)
}

// huffmanLine is one line of a Huffman table (B.2).
type huffmanLine struct {
	isOOB        bool
	rangeLow     int
	prefixLength int
	rangeLength  int
	prefixCode   int
	isLowerRange bool
}

// line makes a normal (or upper range) table line.
func line(rangeLow, prefixLength, rangeLength, prefixCode int) huffmanLine {
	return huffmanLine{rangeLow: rangeLow, prefixLength: prefixLength, rangeLength: rangeLength, prefixCode: prefixCode}
}

// lowerLine makes a lower range table line.
func lowerLine(rangeLow, prefixLength, rangeLength, prefixCode int) huffmanLine {
	l := line(rangeLow, prefixLength, rangeLength, prefixCode)
	l.isLowerRange = true
	return l
}

// oobLine makes an out-of-band table line.
func oobLine(prefixLength, prefixCode int) huffmanLine {
	return huffmanLine{isOOB: true, prefixLength: prefixLength, prefixCode: prefixCode}
}

type huffmanNode struct {
	children     [2]*huffmanNode
	isLeaf       bool
	rangeLength  int
	rangeLow     int
	isLowerRange bool
	isOOB        bool
}

func (n *huffmanNode) buildTree(l *huffmanLine, shift int) {
	bit := (l.prefixCode >> uint(shift)) & 1
	if shift <= 0 {
		// Create a leaf node.
		n.children[bit] = &huffmanNode{
			isLeaf:       true,
			rangeLength:  l.rangeLength,
			rangeLow:     l.rangeLow,
			isLowerRange: l.isLowerRange,
			isOOB:        l.isOOB,
		}
		return
	}
	// Create an intermediate node and continue recursively.
	node := n.children[bit]
	if node == nil {
		node = &huffmanNode{}
		n.children[bit] = node
	}
	node.buildTree(l, shift-1)
}

// huffmanTable decodes values with a Huffman table.
type huffmanTable struct {
	root *huffmanNode
}

// decode returns the decoded value, or ok false for OOB.
func (t *huffmanTable) decode(r *reader) (value int, ok bool) {
	n := t.root
	for !n.isLeaf {
		n = n.children[r.readBit()]
		if n == nil {
			fail("invalid Huffman data")
		}
	}
	if n.isOOB {
		return 0, false
	}
	htOffset := r.readBits(n.rangeLength)
	if n.isLowerRange {
		return n.rangeLow - htOffset, true
	}
	return n.rangeLow + htOffset, true
}

// decodeOr0 is decode where OOB counts as 0, as pdf.js does where it does
// not check for null.
func (t *huffmanTable) decodeOr0(r *reader) int {
	v, _ := t.decode(r)
	return v
}

func newHuffmanTable(lines []huffmanLine, prefixCodesDone bool) *huffmanTable {
	if !prefixCodesDone {
		assignPrefixCodes(lines)
	}
	t := &huffmanTable{root: &huffmanNode{}}
	for i := range lines {
		if lines[i].prefixLength > 0 {
			if lines[i].prefixLength > 32 {
				fail("invalid Huffman prefix length")
			}
			t.root.buildTree(&lines[i], lines[i].prefixLength-1)
		}
	}
	return t
}

// assignPrefixCodes is Annex B.3, assigning the prefix codes.
func assignPrefixCodes(lines []huffmanLine) {
	prefixLengthMax := 0
	for _, l := range lines {
		prefixLengthMax = max(prefixLengthMax, l.prefixLength)
	}
	histogram := make([]int, prefixLengthMax+1)
	for _, l := range lines {
		histogram[l.prefixLength]++
	}
	currentLength, firstCode := 1, 0
	histogram[0] = 0
	for currentLength <= prefixLengthMax {
		firstCode = (firstCode + histogram[currentLength-1]) << 1
		currentCode := firstCode
		for i := range lines {
			if lines[i].prefixLength == currentLength {
				lines[i].prefixCode = currentCode
				currentCode++
			}
		}
		currentLength++
	}
}

// decodeTablesSegment decodes a Tables segment, i.e., a custom Huffman
// table (Annex B.2 Code table structure).
func decodeTablesSegment(data []byte, start, end int) *huffmanTable {
	flags := byteAt(data, start)
	lowestValue := int(int32(readUint32(data, start+1)))
	highestValue := int(int32(readUint32(data, start+5)))
	r := newReader(data, start+9, end)

	prefixSizeBits := int((flags>>1)&7) + 1
	rangeSizeBits := int((flags>>4)&7) + 1
	var lines []huffmanLine
	currentRangeLow := lowestValue

	// Normal table lines
	for {
		prefixLength := r.readBits(prefixSizeBits)
		rangeLength := r.readBits(rangeSizeBits)
		lines = append(lines, line(currentRangeLow, prefixLength, rangeLength, 0))
		// JavaScript shifts are modulo 32 and give an int32.
		currentRangeLow += int(int32(1) << uint(rangeLength&31))
		if currentRangeLow >= highestValue {
			break
		}
	}

	// Lower range table line
	prefixLength := r.readBits(prefixSizeBits)
	lines = append(lines, lowerLine(lowestValue-1, prefixLength, 32, 0))

	// Upper range table line
	prefixLength = r.readBits(prefixSizeBits)
	lines = append(lines, line(highestValue, prefixLength, 32, 0))

	if flags&1 != 0 {
		// Out-of-band table line
		prefixLength = r.readBits(prefixSizeBits)
		lines = append(lines, oobLine(prefixLength, 0))
	}
	return newHuffmanTable(lines, false)
}

var (
	standardTablesMu    sync.Mutex
	standardTablesCache = map[int]*huffmanTable{}
)

// getStandardTable returns one of the standard Huffman tables (B.5).
func getStandardTable(number int) *huffmanTable {
	standardTablesMu.Lock()
	defer standardTablesMu.Unlock()
	if t := standardTablesCache[number]; t != nil {
		return t
	}
	var lines []huffmanLine
	switch number {
	case 1:
		lines = []huffmanLine{
			line(0, 1, 4, 0x0),
			line(16, 2, 8, 0x2),
			line(272, 3, 16, 0x6),
			line(65808, 3, 32, 0x7), // upper
		}
	case 2:
		lines = []huffmanLine{
			line(0, 1, 0, 0x0),
			line(1, 2, 0, 0x2),
			line(2, 3, 0, 0x6),
			line(3, 4, 3, 0xe),
			line(11, 5, 6, 0x1e),
			line(75, 6, 32, 0x3e), // upper
			oobLine(6, 0x3f),
		}
	case 3:
		lines = []huffmanLine{
			line(-256, 8, 8, 0xfe),
			line(0, 1, 0, 0x0),
			line(1, 2, 0, 0x2),
			line(2, 3, 0, 0x6),
			line(3, 4, 3, 0xe),
			line(11, 5, 6, 0x1e),
			lowerLine(-257, 8, 32, 0xff),
			line(75, 7, 32, 0x7e), // upper
			oobLine(6, 0x3e),
		}
	case 4:
		lines = []huffmanLine{
			line(1, 1, 0, 0x0),
			line(2, 2, 0, 0x2),
			line(3, 3, 0, 0x6),
			line(4, 4, 3, 0xe),
			line(12, 5, 6, 0x1e),
			line(76, 5, 32, 0x1f), // upper
		}
	case 5:
		lines = []huffmanLine{
			line(-255, 7, 8, 0x7e),
			line(1, 1, 0, 0x0),
			line(2, 2, 0, 0x2),
			line(3, 3, 0, 0x6),
			line(4, 4, 3, 0xe),
			line(12, 5, 6, 0x1e),
			lowerLine(-256, 7, 32, 0x7f),
			line(76, 6, 32, 0x3e), // upper
		}
	case 6:
		lines = []huffmanLine{
			line(-2048, 5, 10, 0x1c),
			line(-1024, 4, 9, 0x8),
			line(-512, 4, 8, 0x9),
			line(-256, 4, 7, 0xa),
			line(-128, 5, 6, 0x1d),
			line(-64, 5, 5, 0x1e),
			line(-32, 4, 5, 0xb),
			line(0, 2, 7, 0x0),
			line(128, 3, 7, 0x2),
			line(256, 3, 8, 0x3),
			line(512, 4, 9, 0xc),
			line(1024, 4, 10, 0xd),
			lowerLine(-2049, 6, 32, 0x3e),
			line(2048, 6, 32, 0x3f), // upper
		}
	case 7:
		lines = []huffmanLine{
			line(-1024, 4, 9, 0x8),
			line(-512, 3, 8, 0x0),
			line(-256, 4, 7, 0x9),
			line(-128, 5, 6, 0x1a),
			line(-64, 5, 5, 0x1b),
			line(-32, 4, 5, 0xa),
			line(0, 4, 5, 0xb),
			line(32, 5, 5, 0x1c),
			line(64, 5, 6, 0x1d),
			line(128, 4, 7, 0xc),
			line(256, 3, 8, 0x1),
			line(512, 3, 9, 0x2),
			line(1024, 3, 10, 0x3),
			lowerLine(-1025, 5, 32, 0x1e),
			line(2048, 5, 32, 0x1f), // upper
		}
	case 8:
		lines = []huffmanLine{
			line(-15, 8, 3, 0xfc),
			line(-7, 9, 1, 0x1fc),
			line(-5, 8, 1, 0xfd),
			line(-3, 9, 0, 0x1fd),
			line(-2, 7, 0, 0x7c),
			line(-1, 4, 0, 0xa),
			line(0, 2, 1, 0x0),
			line(2, 5, 0, 0x1a),
			line(3, 6, 0, 0x3a),
			line(4, 3, 4, 0x4),
			line(20, 6, 1, 0x3b),
			line(22, 4, 4, 0xb),
			line(38, 4, 5, 0xc),
			line(70, 5, 6, 0x1b),
			line(134, 5, 7, 0x1c),
			line(262, 6, 7, 0x3c),
			line(390, 7, 8, 0x7d),
			line(646, 6, 10, 0x3d),
			lowerLine(-16, 9, 32, 0x1fe),
			line(1670, 9, 32, 0x1ff), // upper
			oobLine(2, 0x1),
		}
	case 9:
		lines = []huffmanLine{
			line(-31, 8, 4, 0xfc),
			line(-15, 9, 2, 0x1fc),
			line(-11, 8, 2, 0xfd),
			line(-7, 9, 1, 0x1fd),
			line(-5, 7, 1, 0x7c),
			line(-3, 4, 1, 0xa),
			line(-1, 3, 1, 0x2),
			line(1, 3, 1, 0x3),
			line(3, 5, 1, 0x1a),
			line(5, 6, 1, 0x3a),
			line(7, 3, 5, 0x4),
			line(39, 6, 2, 0x3b),
			line(43, 4, 5, 0xb),
			line(75, 4, 6, 0xc),
			line(139, 5, 7, 0x1b),
			line(267, 5, 8, 0x1c),
			line(523, 6, 8, 0x3c),
			line(779, 7, 9, 0x7d),
			line(1291, 6, 11, 0x3d),
			lowerLine(-32, 9, 32, 0x1fe),
			line(3339, 9, 32, 0x1ff), // upper
			oobLine(2, 0x0),
		}
	case 10:
		lines = []huffmanLine{
			line(-21, 7, 4, 0x7a),
			line(-5, 8, 0, 0xfc),
			line(-4, 7, 0, 0x7b),
			line(-3, 5, 0, 0x18),
			line(-2, 2, 2, 0x0),
			line(2, 5, 0, 0x19),
			line(3, 6, 0, 0x36),
			line(4, 7, 0, 0x7c),
			line(5, 8, 0, 0xfd),
			line(6, 2, 6, 0x1),
			line(70, 5, 5, 0x1a),
			line(102, 6, 5, 0x37),
			line(134, 6, 6, 0x38),
			line(198, 6, 7, 0x39),
			line(326, 6, 8, 0x3a),
			line(582, 6, 9, 0x3b),
			line(1094, 6, 10, 0x3c),
			line(2118, 7, 11, 0x7d),
			lowerLine(-22, 8, 32, 0xfe),
			line(4166, 8, 32, 0xff), // upper
			oobLine(2, 0x2),
		}
	case 11:
		lines = []huffmanLine{
			line(1, 1, 0, 0x0),
			line(2, 2, 1, 0x2),
			line(4, 4, 0, 0xc),
			line(5, 4, 1, 0xd),
			line(7, 5, 1, 0x1c),
			line(9, 5, 2, 0x1d),
			line(13, 6, 2, 0x3c),
			line(17, 7, 2, 0x7a),
			line(21, 7, 3, 0x7b),
			line(29, 7, 4, 0x7c),
			line(45, 7, 5, 0x7d),
			line(77, 7, 6, 0x7e),
			line(141, 7, 32, 0x7f), // upper
		}
	case 12:
		lines = []huffmanLine{
			line(1, 1, 0, 0x0),
			line(2, 2, 0, 0x2),
			line(3, 3, 1, 0x6),
			line(5, 5, 0, 0x1c),
			line(6, 5, 1, 0x1d),
			line(8, 6, 1, 0x3c),
			line(10, 7, 0, 0x7a),
			line(11, 7, 1, 0x7b),
			line(13, 7, 2, 0x7c),
			line(17, 7, 3, 0x7d),
			line(25, 7, 4, 0x7e),
			line(41, 8, 5, 0xfe),
			line(73, 8, 32, 0xff), // upper
		}
	case 13:
		lines = []huffmanLine{
			line(1, 1, 0, 0x0),
			line(2, 3, 0, 0x4),
			line(3, 4, 0, 0xc),
			line(4, 5, 0, 0x1c),
			line(5, 4, 1, 0xd),
			line(7, 3, 3, 0x5),
			line(15, 6, 1, 0x3a),
			line(17, 6, 2, 0x3b),
			line(21, 6, 3, 0x3c),
			line(29, 6, 4, 0x3d),
			line(45, 6, 5, 0x3e),
			line(77, 7, 6, 0x7e),
			line(141, 7, 32, 0x7f), // upper
		}
	case 14:
		lines = []huffmanLine{
			line(-2, 3, 0, 0x4),
			line(-1, 3, 0, 0x5),
			line(0, 1, 0, 0x0),
			line(1, 3, 0, 0x6),
			line(2, 3, 0, 0x7),
		}
	case 15:
		lines = []huffmanLine{
			line(-24, 7, 4, 0x7c),
			line(-8, 6, 2, 0x3c),
			line(-4, 5, 1, 0x1c),
			line(-2, 4, 0, 0xc),
			line(-1, 3, 0, 0x4),
			line(0, 1, 0, 0x0),
			line(1, 3, 0, 0x5),
			line(2, 4, 0, 0xd),
			line(3, 5, 1, 0x1d),
			line(5, 6, 2, 0x3d),
			line(9, 7, 4, 0x7d),
			lowerLine(-25, 7, 32, 0x7e),
			line(25, 7, 32, 0x7f), // upper
		}
	default:
		fail("standard table B.%d does not exist", number)
	}
	t := newHuffmanTable(lines, true)
	standardTablesCache[number] = t
	return t
}

// getCustomHuffmanTable returns the index-th Tables segment among the
// referred-to segments (7.4.2.1.6 / 7.4.3.1.6).
func getCustomHuffmanTable(index int, referredTo []uint32, customTables map[uint32]*huffmanTable) *huffmanTable {
	currentIndex := 0
	for _, ref := range referredTo {
		if t := customTables[ref]; t != nil {
			if index == currentIndex {
				return t
			}
			currentIndex++
		}
	}
	fail("can't find custom Huffman table")
	return nil
}

type textRegionHuffmanTables struct {
	symbolIDTable, tableFirstS, tableDeltaS, tableDeltaT *huffmanTable
	// symbolIDBits > 0 means fixed length symbol IDs instead of
	// symbolIDTable (text regions in symbol dictionaries, 6.5.8.2.3).
	symbolIDBits int
	// refinement tables (not in pdf.js)
	tableRefineDW, tableRefineDH, tableRefineDX, tableRefineDY, tableRefineSz *huffmanTable
}

// getTextRegionHuffmanTables reads the symbol ID Huffman table (7.4.3.1.7)
// and selects the other tables (7.4.3.1.6).
func getTextRegionHuffmanTables(tr *textRegion, referredTo []uint32,
	customTables map[uint32]*huffmanTable, numberOfSymbols int, r *reader) *textRegionHuffmanTables {
	// Read code lengths for RUNCODEs 0...34.
	codes := make([]huffmanLine, 0, 35)
	for i := 0; i <= 34; i++ {
		codes = append(codes, line(i, r.readBits(4), 0, 0))
	}
	// Assign Huffman codes for RUNCODEs.
	runCodesTable := newHuffmanTable(codes, false)

	// Read a Huffman code using the assignment above.
	// Interpret the RUNCODE codes and the additional bits (if any).
	codes = codes[:0:0]
	for i := 0; i < numberOfSymbols; {
		codeLength := runCodesTable.decodeOr0(r)
		if codeLength >= 32 {
			var repeatedLength, numberOfRepeats int
			switch codeLength {
			case 32:
				if i == 0 {
					fail("no previous value in symbol ID table")
				}
				numberOfRepeats = r.readBits(2) + 3
				repeatedLength = codes[i-1].prefixLength
			case 33:
				numberOfRepeats = r.readBits(3) + 3
				repeatedLength = 0
			case 34:
				numberOfRepeats = r.readBits(7) + 11
				repeatedLength = 0
			default:
				fail("invalid code length in symbol ID table")
			}
			for j := 0; j < numberOfRepeats; j++ {
				codes = append(codes, line(i, repeatedLength, 0, 0))
				i++
			}
		} else {
			codes = append(codes, line(i, codeLength, 0, 0))
			i++
		}
	}
	r.byteAlign()
	t := &textRegionHuffmanTables{symbolIDTable: newHuffmanTable(codes, false)}

	// 7.4.3.1.6 Text region segment Huffman table selection
	customIndex := 0
	switch tr.huffmanFS {
	case 0, 1:
		t.tableFirstS = getStandardTable(tr.huffmanFS + 6)
	case 3:
		t.tableFirstS = getCustomHuffmanTable(customIndex, referredTo, customTables)
		customIndex++
	default:
		fail("invalid Huffman FS selector")
	}

	switch tr.huffmanDS {
	case 0, 1, 2:
		t.tableDeltaS = getStandardTable(tr.huffmanDS + 8)
	case 3:
		t.tableDeltaS = getCustomHuffmanTable(customIndex, referredTo, customTables)
		customIndex++
	default:
		fail("invalid Huffman DS selector")
	}

	switch tr.huffmanDT {
	case 0, 1, 2:
		t.tableDeltaT = getStandardTable(tr.huffmanDT + 11)
	case 3:
		t.tableDeltaT = getCustomHuffmanTable(customIndex, referredTo, customTables)
		customIndex++
	default:
		fail("invalid Huffman DT selector")
	}

	if tr.refinement {
		// Load tables RDW, RDH, RDX and RDY (pdf.js does not support
		// refinement with Huffman coding): 0 is B.14, 1 is B.15, 3 custom.
		refineTable := func(selector int, name string) *huffmanTable {
			switch selector {
			case 0, 1:
				return getStandardTable(selector + 14)
			case 3:
				t := getCustomHuffmanTable(customIndex, referredTo, customTables)
				customIndex++
				return t
			}
			fail("invalid Huffman %s selector", name)
			return nil
		}
		t.tableRefineDW = refineTable(tr.huffmanRefinementDW, "RDW")
		t.tableRefineDH = refineTable(tr.huffmanRefinementDH, "RDH")
		t.tableRefineDX = refineTable(tr.huffmanRefinementDX, "RDX")
		t.tableRefineDY = refineTable(tr.huffmanRefinementDY, "RDY")
		if tr.huffmanRefinementSizeSelector {
			t.tableRefineSz = getCustomHuffmanTable(customIndex, referredTo, customTables)
		} else {
			t.tableRefineSz = getStandardTable(1)
		}
	}
	return t
}

type symbolDictionaryHuffmanTables struct {
	tableDeltaHeight, tableDeltaWidth        *huffmanTable
	tableBitmapSize, tableAggregateInstances *huffmanTable
}

// getSymbolDictionaryHuffmanTables is 7.4.2.1.6, symbol dictionary
// segment Huffman table selection.
func getSymbolDictionaryHuffmanTables(d *symbolDictionary, referredTo []uint32,
	customTables map[uint32]*huffmanTable) *symbolDictionaryHuffmanTables {
	t := &symbolDictionaryHuffmanTables{}
	customIndex := 0
	switch d.huffmanDHSelector {
	case 0, 1:
		t.tableDeltaHeight = getStandardTable(d.huffmanDHSelector + 4)
	case 3:
		t.tableDeltaHeight = getCustomHuffmanTable(customIndex, referredTo, customTables)
		customIndex++
	default:
		fail("invalid Huffman DH selector")
	}

	switch d.huffmanDWSelector {
	case 0, 1:
		t.tableDeltaWidth = getStandardTable(d.huffmanDWSelector + 2)
	case 3:
		t.tableDeltaWidth = getCustomHuffmanTable(customIndex, referredTo, customTables)
		customIndex++
	default:
		fail("invalid Huffman DW selector")
	}

	if d.bitmapSizeSelector != 0 {
		t.tableBitmapSize = getCustomHuffmanTable(customIndex, referredTo, customTables)
		customIndex++
	} else {
		t.tableBitmapSize = getStandardTable(1)
	}

	if d.aggregationInstancesSelector != 0 {
		t.tableAggregateInstances = getCustomHuffmanTable(customIndex, referredTo, customTables)
	} else {
		t.tableAggregateInstances = getStandardTable(1)
	}
	return t
}
