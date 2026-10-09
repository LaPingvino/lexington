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

// Ported from pdf.js src/core/jbig2.js: symbol dictionaries, text regions,
// pattern dictionaries and halftone regions.

package jbig2

// log2 is ceil(log2(x)), 0 for x <= 1 (pdf.js core_utils log2).
func log2(x int) int {
	n := 0
	for x > 1<<n {
		n++
	}
	return n
}

// symbolAt returns symbols[id] or fails.
func symbolAt(symbols []*bitmap, id int) *bitmap {
	if id < 0 || id >= len(symbols) || symbols[id] == nil {
		fail("invalid symbol ID %d (%d symbols)", id, len(symbols))
	}
	return symbols[id]
}

// decodeSymbolDictionary is 6.5.5, decoding the symbol dictionary. It
// returns the exported symbols.
func decodeSymbolDictionary(huffman, refinement bool, symbols []*bitmap,
	numberOfNewSymbols, numberOfExportedSymbols int, huffmanTables *symbolDictionaryHuffmanTables,
	templateIndex int, at []point, refinementTemplateIndex int, refinementAt []point,
	dc *decodingContext, huffmanInput *reader) []*bitmap {
	if numberOfNewSymbols < 0 || numberOfNewSymbols > maxPixels {
		fail("invalid number of new symbols")
	}

	newSymbols := make([]*bitmap, 0, min(numberOfNewSymbols, 1<<16))
	currentHeight := 0
	symbolCodeLength := log2(len(symbols) + numberOfNewSymbols)

	var tableB1 *huffmanTable
	var symbolWidths []int
	if huffman {
		tableB1 = getStandardTable(1)               // standard table B.1
		symbolCodeLength = max(symbolCodeLength, 1) // 6.5.8.2.3
	}
	cache := &dc.cache
	// The arithmetic decoder is only created when needed: a Huffman coded
	// dictionary does not use it.
	decoder := func() *arithDecoder { return dc.decoder() }

	for len(newSymbols) < numberOfNewSymbols {
		var deltaHeight int
		if huffman {
			deltaHeight = huffmanTables.tableDeltaHeight.decodeOr0(huffmanInput)
		} else {
			deltaHeight = decodeIntegerOr0(cache, "IADH", decoder()) // 6.5.6
		}
		currentHeight += deltaHeight
		currentWidth, totalWidth := 0, 0
		firstSymbol := 0
		if huffman {
			firstSymbol = len(symbolWidths)
		}
		for {
			var deltaWidth int
			var ok bool
			if huffman {
				deltaWidth, ok = huffmanTables.tableDeltaWidth.decode(huffmanInput)
			} else {
				deltaWidth, ok = decodeInteger(cache, "IADW", decoder()) // 6.5.7
			}
			if !ok {
				break // OOB
			}
			// More symbols than announced: corrupt data (pdf.js would loop
			// until OOB).
			if len(newSymbols)+len(symbolWidths)-firstSymbol >= numberOfNewSymbols {
				fail("too many symbols in symbol dictionary")
			}
			currentWidth += deltaWidth
			totalWidth += currentWidth
			if refinement {
				// 6.5.8.2 Refinement/aggregate-coded symbol bitmap
				var bm *bitmap
				var numberOfInstances int
				if huffman {
					numberOfInstances = huffmanTables.tableAggregateInstances.decodeOr0(huffmanInput)
				} else {
					numberOfInstances = decodeIntegerOr0(cache, "IAAI", decoder())
				}
				if numberOfInstances > 1 {
					// 6.5.8.2.1, with the parameters of Table 17.
					all := append(append([]*bitmap(nil), symbols...), newSymbols...)
					var tables *textRegionHuffmanTables
					if huffman {
						tables = &textRegionHuffmanTables{
							symbolIDBits:  symbolCodeLength,
							tableFirstS:   getStandardTable(6),
							tableDeltaS:   getStandardTable(8),
							tableDeltaT:   getStandardTable(11),
							tableRefineDW: getStandardTable(15),
							tableRefineDH: getStandardTable(15),
							tableRefineDX: getStandardTable(15),
							tableRefineDY: getStandardTable(15),
							tableRefineSz: getStandardTable(1),
						}
					}
					bm = decodeTextRegion(huffman, refinement, currentWidth, currentHeight, 0,
						numberOfInstances, 1, all, symbolCodeLength,
						false, // transposed
						0,     // ds offset
						1,     // top left 7.4.3.1.1
						0,     // OR operator
						tables, refinementTemplateIndex, refinementAt, dc, 0, huffmanInput)
				} else {
					// 6.5.8.2.2
					var symbolID, rdx, rdy int
					if huffman {
						symbolID = huffmanInput.readBits(symbolCodeLength)
						tableB15 := getStandardTable(15)
						rdx = tableB15.decodeOr0(huffmanInput)
						rdy = tableB15.decodeOr0(huffmanInput)
					} else {
						symbolID = decodeIAID(cache, decoder(), symbolCodeLength)
						rdx = decodeIntegerOr0(cache, "IARDX", decoder()) // 6.4.11.3
						rdy = decodeIntegerOr0(cache, "IARDY", decoder()) // 6.4.11.4
					}
					var symbol *bitmap
					if symbolID < len(symbols) {
						symbol = symbolAt(symbols, symbolID)
					} else {
						symbol = symbolAt(newSymbols, symbolID-len(symbols))
					}
					if huffman {
						bm = refineFromHuffmanInput(huffmanInput, currentWidth, currentHeight,
							refinementTemplateIndex, symbol, rdx, rdy, refinementAt, dc)
					} else {
						bm = decodeRefinement(currentWidth, currentHeight, refinementTemplateIndex,
							symbol, rdx, rdy, false, refinementAt, dc)
					}
				}
				newSymbols = append(newSymbols, bm)
			} else if huffman {
				// Store only symbol width and decode a collective bitmap
				// when the height class is done.
				symbolWidths = append(symbolWidths, currentWidth)
			} else {
				// 6.5.8.1 Direct-coded symbol bitmap
				bm := decodeBitmap(false, currentWidth, currentHeight, templateIndex,
					false, nil, at, dc)
				newSymbols = append(newSymbols, bm)
			}
		}
		if huffman && !refinement {
			// 6.5.9 Height class collective bitmap
			bitmapSize := huffmanTables.tableBitmapSize.decodeOr0(huffmanInput)
			huffmanInput.byteAlign()
			var collectiveBitmap *bitmap
			if bitmapSize == 0 {
				// Uncompressed collective bitmap
				collectiveBitmap = readUncompressedBitmap(huffmanInput, totalWidth, currentHeight)
			} else {
				// MMR collective bitmap
				originalEnd := huffmanInput.end
				bitmapEnd := huffmanInput.position + bitmapSize
				if bitmapSize < 0 || bitmapEnd > originalEnd {
					fail("invalid collective bitmap size")
				}
				huffmanInput.end = bitmapEnd
				collectiveBitmap = decodeMMRBitmap(huffmanInput, totalWidth, currentHeight, false)
				huffmanInput.end = originalEnd
				huffmanInput.position = bitmapEnd
			}
			numberOfSymbolsDecoded := len(symbolWidths)
			if firstSymbol == numberOfSymbolsDecoded-1 {
				// collectiveBitmap is a single symbol.
				newSymbols = append(newSymbols, collectiveBitmap)
			} else {
				// Divide collectiveBitmap into symbols.
				xMin := 0
				for i := firstSymbol; i < numberOfSymbolsDecoded; i++ {
					bitmapWidth := symbolWidths[i]
					xMax := xMin + bitmapWidth
					if bitmapWidth < 0 || xMax > collectiveBitmap.w {
						fail("invalid symbol width")
					}
					sym := &bitmap{w: bitmapWidth, h: currentHeight, rows: make([][]byte, currentHeight)}
					for y := 0; y < currentHeight; y++ {
						sym.rows[y] = collectiveBitmap.rows[y][xMin:xMax:xMax]
					}
					newSymbols = append(newSymbols, sym)
					xMin = xMax
				}
			}
		}
	}

	// 6.5.10 Exported symbols
	totalSymbolsLength := len(symbols) + numberOfNewSymbols
	flags := make([]bool, 0, totalSymbolsLength)
	currentFlag := false
	for runs := 0; len(flags) < totalSymbolsLength; runs++ {
		if runs > 2*totalSymbolsLength+2 {
			fail("invalid export flags")
		}
		var runLength int
		if huffman {
			runLength = tableB1.decodeOr0(huffmanInput)
		} else {
			runLength = decodeIntegerOr0(cache, "IAEX", decoder())
		}
		if runLength > totalSymbolsLength-len(flags) {
			// pdf.js would push the extra flags and ignore them.
			runLength = totalSymbolsLength - len(flags)
		}
		for ; runLength > 0; runLength-- {
			flags = append(flags, currentFlag)
		}
		currentFlag = !currentFlag
	}
	var exportedSymbols []*bitmap
	i := 0
	for ; i < len(symbols); i++ {
		if flags[i] {
			exportedSymbols = append(exportedSymbols, symbols[i])
		}
	}
	for j := 0; j < numberOfNewSymbols; i, j = i+1, j+1 {
		if flags[i] {
			exportedSymbols = append(exportedSymbols, symbolAt(newSymbols, j))
		}
	}
	return exportedSymbols
}

// combine combines a source pixel into a destination pixel.
func combine(dst *byte, src byte, op int) {
	switch op {
	case 0: // OR
		*dst |= src
	case 1: // AND
		*dst &= src
	case 2: // XOR
		*dst ^= src
	case 3: // XNOR
		*dst = ^(*dst ^ src) & 1
	case 4: // REPLACE
		*dst = src
	default:
		fail("operator %d is not supported", op)
	}
}

// decodeTextRegion is the text region decoding procedure (6.4).
func decodeTextRegion(huffman, refinement bool, width, height, defaultPixelValue,
	numberOfSymbolInstances, stripSize int, inputSymbols []*bitmap, symbolCodeLength int,
	transposed bool, dsOffset, referenceCorner, combinationOperator int,
	huffmanTables *textRegionHuffmanTables, refinementTemplateIndex int, refinementAt []point,
	dc *decodingContext, logStripSize int, huffmanInput *reader) *bitmap {
	// pdf.js only supports OR and XOR; the other operators of 6.4.10 are
	// implemented by combine.
	if combinationOperator > 4 {
		fail("operator %d is not supported", combinationOperator)
	}

	// Prepare bitmap
	bm := newBitmap(width, height)
	if defaultPixelValue != 0 {
		for _, row := range bm.rows {
			for j := range row {
				row[j] = 1
			}
		}
	}

	cache := &dc.cache
	decoder := func() *arithDecoder { return dc.decoder() }

	var stripT int
	if huffman {
		stripT = -huffmanTables.tableDeltaT.decodeOr0(huffmanInput)
	} else {
		stripT = -decodeIntegerOr0(cache, "IADT", decoder()) // 6.4.6
	}
	firstS := 0
	for i := 0; i < numberOfSymbolInstances; {
		var deltaT int
		if huffman {
			deltaT = huffmanTables.tableDeltaT.decodeOr0(huffmanInput)
		} else {
			deltaT = decodeIntegerOr0(cache, "IADT", decoder()) // 6.4.6
		}
		stripT += deltaT

		var deltaFirstS int
		if huffman {
			deltaFirstS = huffmanTables.tableFirstS.decodeOr0(huffmanInput)
		} else {
			deltaFirstS = decodeIntegerOr0(cache, "IAFS", decoder()) // 6.4.7
		}
		firstS += deltaFirstS
		currentS := firstS
		for {
			currentT := 0 // 6.4.9
			if stripSize > 1 {
				if huffman {
					currentT = huffmanInput.readBits(logStripSize)
				} else {
					currentT = decodeIntegerOr0(cache, "IAIT", decoder())
				}
			}
			t := stripSize*stripT + currentT
			var symbolID int
			if huffman && huffmanTables.symbolIDBits > 0 {
				// fixed length symbol codes, in symbol dictionaries (6.5.8.2.3)
				symbolID = huffmanInput.readBits(huffmanTables.symbolIDBits)
			} else if huffman {
				symbolID = huffmanTables.symbolIDTable.decodeOr0(huffmanInput)
			} else {
				symbolID = decodeIAID(cache, decoder(), symbolCodeLength)
			}
			applyRefinement := false
			if refinement {
				if huffman {
					applyRefinement = huffmanInput.readBit() != 0
				} else {
					applyRefinement = decodeIntegerOr0(cache, "IARI", decoder()) != 0
				}
			}
			symbolBitmap := symbolAt(inputSymbols, symbolID)
			symbolWidth, symbolHeight := symbolBitmap.w, symbolBitmap.h
			if applyRefinement && huffman {
				// 6.4.11 with Huffman coding (not in pdf.js)
				rdw := huffmanTables.tableRefineDW.decodeOr0(huffmanInput)
				rdh := huffmanTables.tableRefineDH.decodeOr0(huffmanInput)
				rdx := huffmanTables.tableRefineDX.decodeOr0(huffmanInput)
				rdy := huffmanTables.tableRefineDY.decodeOr0(huffmanInput)
				symbolWidth += rdw
				symbolHeight += rdh
				symbolBitmap = refineFromHuffmanInputSized(huffmanInput, huffmanTables.tableRefineSz,
					symbolWidth, symbolHeight, refinementTemplateIndex, symbolBitmap,
					(rdw>>1)+rdx, (rdh>>1)+rdy, refinementAt, dc)
			} else if applyRefinement {
				rdw := decodeIntegerOr0(cache, "IARDW", decoder()) // 6.4.11.1
				rdh := decodeIntegerOr0(cache, "IARDH", decoder()) // 6.4.11.2
				rdx := decodeIntegerOr0(cache, "IARDX", decoder()) // 6.4.11.3
				rdy := decodeIntegerOr0(cache, "IARDY", decoder()) // 6.4.11.4
				symbolWidth += rdw
				symbolHeight += rdh
				symbolBitmap = decodeRefinement(symbolWidth, symbolHeight, refinementTemplateIndex,
					symbolBitmap, (rdw>>1)+rdx, (rdh>>1)+rdy, false, refinementAt, dc)
			}

			increment := 0
			if !transposed {
				if referenceCorner > 1 {
					currentS += symbolWidth - 1
				} else {
					increment = symbolWidth - 1
				}
			} else if referenceCorner&1 == 0 {
				currentS += symbolHeight - 1
			} else {
				increment = symbolHeight - 1
			}

			// The reference corner is bottom unless bit 0 is set, and right
			// if bit 1 is set. pdf.js applies this to S and T as if never
			// transposed; when transposed, S is vertical and T horizontal.
			offsetT, offsetS := t, currentS
			if !transposed {
				if referenceCorner&1 == 0 {
					offsetT -= symbolHeight - 1
				}
				if referenceCorner&2 != 0 {
					offsetS -= symbolWidth - 1
				}
			} else {
				if referenceCorner&2 != 0 {
					offsetT -= symbolWidth - 1
				}
				if referenceCorner&1 == 0 {
					offsetS -= symbolHeight - 1
				}
			}
			if transposed {
				// Place Symbol Bitmap from T1,S1
				for s2 := 0; s2 < symbolHeight; s2++ {
					y := offsetS + s2
					if y < 0 || y >= height {
						continue
					}
					row := bm.rows[y]
					symbolRow := symbolBitmap.rows[s2]
					// To ignore Parts of Symbol bitmap which goes
					// outside bitmap region
					maxWidth := min(width-offsetT, symbolWidth)
					for t2 := 0; t2 < maxWidth; t2++ {
						if x := offsetT + t2; x >= 0 {
							combine(&row[x], symbolRow[t2], combinationOperator)
						}
					}
				}
			} else {
				for t2 := 0; t2 < symbolHeight; t2++ {
					y := offsetT + t2
					if y < 0 || y >= height {
						continue
					}
					row := bm.rows[y]
					symbolRow := symbolBitmap.rows[t2]
					for s2 := 0; s2 < symbolWidth; s2++ {
						if x := offsetS + s2; x >= 0 && x < width {
							combine(&row[x], symbolRow[s2], combinationOperator)
						}
					}
				}
			}
			i++
			if i > numberOfSymbolInstances {
				// pdf.js would loop until OOB.
				fail("too many symbol instances in text region")
			}
			var deltaS int
			var ok bool
			if huffman {
				deltaS, ok = huffmanTables.tableDeltaS.decode(huffmanInput)
			} else {
				deltaS, ok = decodeInteger(cache, "IADS", decoder()) // 6.4.8
			}
			if !ok {
				break // OOB
			}
			currentS += increment + deltaS + dsOffset
		}
	}
	return bm
}

// decodePatternDictionary is the pattern dictionary decoding procedure
// (6.7).
func decodePatternDictionary(mmr bool, patternWidth, patternHeight, maxPatternIndex,
	template int, dc *decodingContext) []*bitmap {
	var at []point
	if !mmr {
		at = append(at, point{-patternWidth, 0})
		if template == 0 {
			at = append(at, point{-3, -1}, point{2, -2}, point{-2, -2})
		}
	}
	if maxPatternIndex < 0 || maxPatternIndex >= maxPixels {
		fail("invalid number of patterns")
	}
	collectiveWidth := (maxPatternIndex + 1) * patternWidth
	collectiveBitmap := decodeBitmap(mmr, collectiveWidth, patternHeight, template,
		false, nil, at, dc)
	// Divide collective bitmap into patterns.
	patterns := make([]*bitmap, 0, maxPatternIndex+1)
	for i := 0; i <= maxPatternIndex; i++ {
		p := &bitmap{w: patternWidth, h: patternHeight, rows: make([][]byte, patternHeight)}
		xMin := patternWidth * i
		xMax := xMin + patternWidth
		for y := 0; y < patternHeight; y++ {
			p.rows[y] = collectiveBitmap.rows[y][xMin:xMax:xMax]
		}
		patterns = append(patterns, p)
	}
	return patterns
}

// decodeHalftoneRegion is the halftone region decoding procedure (6.6).
func decodeHalftoneRegion(mmr bool, patterns []*bitmap, template, regionWidth, regionHeight,
	defaultPixelValue int, enableSkip bool, combinationOperator, gridWidth, gridHeight,
	gridOffsetX, gridOffsetY, gridVectorX, gridVectorY int, dc *decodingContext) *bitmap {
	// pdf.js supports neither skipping nor operators other than OR.
	if combinationOperator > 4 {
		fail("operator %d is not supported in halftone region", combinationOperator)
	}
	if len(patterns) == 0 {
		fail("halftone region without patterns")
	}

	// Prepare bitmap.
	regionBitmap := newBitmap(regionWidth, regionHeight)
	if defaultPixelValue != 0 {
		for _, row := range regionBitmap.rows {
			for j := range row {
				row[j] = 1
			}
		}
	}

	numberOfPatterns := len(patterns)
	pattern0 := patterns[0]
	patternWidth, patternHeight := pattern0.w, pattern0.h
	bitsPerValue := log2(numberOfPatterns)
	var at []point
	if !mmr {
		x := 2
		if template <= 1 {
			x = 3
		}
		at = append(at, point{x, -1})
		if template == 0 {
			at = append(at, point{-3, -1}, point{2, -2}, point{-2, -2})
		}
	}
	checkSize(gridWidth, gridHeight)
	if bitsPerValue > 0 && gridWidth*gridHeight > maxPixels/bitsPerValue {
		fail("halftone grid too large")
	}
	// 6.6.5.1 Computing HSKIP
	var skip *bitmap
	if enableSkip {
		skip = newBitmap(gridWidth, gridHeight)
		for mg := 0; mg < gridHeight; mg++ {
			for ng := 0; ng < gridWidth; ng++ {
				x := int(int32(gridOffsetX+mg*gridVectorY+ng*gridVectorX) >> 8)
				y := int(int32(gridOffsetY+mg*gridVectorX-ng*gridVectorY) >> 8)
				if x+patternWidth <= 0 || x >= regionWidth || y+patternHeight <= 0 || y >= regionHeight {
					skip.rows[mg][ng] = 1
				}
			}
		}
	}
	// Annex C. Gray-scale Image Decoding Procedure.
	grayScaleBitPlanes := make([]*bitmap, bitsPerValue)
	var mmrInput *reader
	if mmr {
		// MMR bit planes are in one continuous stream. Only EOFB codes
		// indicate the end of each bitmap, so EOFBs must be decoded.
		mmrInput = newReader(dc.data, dc.start, dc.end)
	}
	for i := bitsPerValue - 1; i >= 0; i-- {
		if mmr {
			grayScaleBitPlanes[i] = decodeMMRBitmap(mmrInput, gridWidth, gridHeight, true)
		} else {
			grayScaleBitPlanes[i] = decodeBitmap(false, gridWidth, gridHeight, template,
				false, skip, at, dc)
		}
	}
	// 6.6.5.2 Rendering the patterns.
	for mg := 0; mg < gridHeight; mg++ {
		for ng := 0; ng < gridWidth; ng++ {
			bit, patternIndex := 0, 0
			for j := bitsPerValue - 1; j >= 0; j-- {
				bit ^= int(grayScaleBitPlanes[j].rows[mg][ng]) // Gray decoding
				patternIndex |= bit << j
			}
			if patternIndex >= numberOfPatterns {
				patternIndex = numberOfPatterns - 1
			}
			patternBitmap := patterns[patternIndex]
			// JavaScript's >> works on int32.
			x := int(int32(gridOffsetX+mg*gridVectorY+ng*gridVectorX) >> 8)
			y := int(int32(gridOffsetY+mg*gridVectorX-ng*gridVectorY) >> 8)
			// Draw patternBitmap at (x, y), clipped to the region.
			for i := max(0, -y); i < patternHeight && y+i < regionHeight; i++ {
				regionRow := regionBitmap.rows[y+i]
				patternRow := patternBitmap.rows[i]
				for j := max(0, -x); j < patternWidth && x+j < regionWidth; j++ {
					combine(&regionRow[x+j], patternRow[j], combinationOperator)
				}
			}
		}
	}
	return regionBitmap
}

// refineFromHuffmanInput decodes an arithmetic coded refinement bitmap
// embedded in Huffman coded data (6.4.11, 6.5.8.2.2): its size BMSIZE is
// read with table B.1 (or the text region's RSIZE table), then the bitmap
// data starts at the next byte boundary. The refinement contexts are those
// of the segment.
func refineFromHuffmanInput(r *reader, width, height, templateIndex int, reference *bitmap,
	dx, dy int, at []point, dc *decodingContext) *bitmap {
	return refineFromHuffmanInputSized(r, getStandardTable(1), width, height, templateIndex,
		reference, dx, dy, at, dc)
}

func refineFromHuffmanInputSized(r *reader, sizeTable *huffmanTable, width, height,
	templateIndex int, reference *bitmap, dx, dy int, at []point, dc *decodingContext) *bitmap {
	bitmapSize := sizeTable.decodeOr0(r)
	r.byteAlign()
	start := r.position
	end := start + bitmapSize
	if bitmapSize < 0 || end > r.end {
		fail("invalid refinement bitmap size")
	}
	sub := newDecodingContext(dc.data, start, end)
	sub.cache.gr = dc.cache.refinementContexts()
	bm := decodeRefinement(width, height, templateIndex, reference, dx, dy, false, at, sub)
	r.position = end
	return bm
}
