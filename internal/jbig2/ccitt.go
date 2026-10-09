// Copyright 1996-2003 Glyph & Cog, LLC
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

// Ported from pdf.js src/core/ccitt.js, a JavaScript port of XPDF's CCITT
// decoder. Only what JBIG2 MMR decoding needs is used, but the decoder is
// ported whole.

package jbig2

const (
	ccittEOL = -2
	ccittEOF = -1

	twoDimPass   = 0
	twoDimHoriz  = 1
	twoDimVert0  = 2
	twoDimVertR1 = 3
	twoDimVertL1 = 4
	twoDimVertR2 = 5
	twoDimVertL2 = 6
	twoDimVertR3 = 7
	twoDimVertL3 = 8
)

// byteSource yields one byte at a time, or -1 at the end of the data.
type byteSource interface {
	next() int
}

type ccittOptions struct {
	K                int
	EndOfLine        bool
	EncodedByteAlign bool
	Columns          int
	Rows             int
	EndOfBlock       bool
	BlackIs1         bool
}

type ccittDecoder struct {
	source    byteSource
	eof       bool
	encoding  int
	eoline    bool
	byteAlign bool
	columns   int
	rows      int
	eoblock   bool
	black     bool

	codingLine []int
	refLine    []int
	codingPos  int

	row        int
	nextLine2D bool
	inputBits  int
	inputBuf   uint32
	outputBits int
	rowsDone   bool
	err        bool
}

func newCCITTDecoder(source byteSource, opt ccittOptions) *ccittDecoder {
	d := &ccittDecoder{
		source:    source,
		encoding:  opt.K,
		eoline:    opt.EndOfLine,
		byteAlign: opt.EncodedByteAlign,
		columns:   opt.Columns,
		rows:      opt.Rows,
		eoblock:   opt.EndOfBlock,
		black:     opt.BlackIs1,
	}
	if d.columns <= 0 {
		d.columns = 1728
	}
	// pdf.js uses typed arrays of columns+1 and columns+2 entries, where
	// out of range writes are ignored; a little slack keeps Go in range.
	d.codingLine = make([]int, d.columns+4)
	d.refLine = make([]int, d.columns+4)
	d.codingLine[0] = d.columns
	d.nextLine2D = d.encoding < 0

	var code1 int
	for {
		code1 = d.lookBits(12)
		if code1 != 0 {
			break
		}
		d.eatBits(1)
	}
	if code1 == 1 {
		d.eatBits(12)
	}
	if d.encoding > 0 {
		d.nextLine2D = d.lookBits(1) == 0
		d.eatBits(1)
	}
	return d
}

// ref returns refLine[i]; beyond the line it acts as the end of the line.
func (d *ccittDecoder) ref(i int) int {
	if i < 0 || i >= len(d.refLine) {
		return d.columns
	}
	return d.refLine[i]
}

// coding returns codingLine[i]; beyond the line it acts as its end.
func (d *ccittDecoder) coding(i int) int {
	if i < 0 || i >= len(d.codingLine) {
		return d.columns
	}
	return d.codingLine[i]
}

func (d *ccittDecoder) setCoding(i, v int) {
	if i >= 0 && i < len(d.codingLine) {
		d.codingLine[i] = v
	}
}

// readNextChar returns the next 8 pixels (1 bits are white unless
// BlackIs1), or -1 at the end.
func (d *ccittDecoder) readNextChar() int {
	if d.eof {
		return -1
	}
	columns := d.columns
	var refPos, blackPixels, i int

	if d.outputBits == 0 {
		if d.rowsDone {
			d.eof = true
		}
		if d.eof {
			return -1
		}
		d.err = false

		var code1, code2, code3 int
		if d.nextLine2D {
			for i = 0; d.coding(i) < columns; i++ {
				d.refLine[i] = d.codingLine[i]
			}
			if i < len(d.refLine) {
				d.refLine[i] = columns
			}
			i++
			if i < len(d.refLine) {
				d.refLine[i] = columns
			}
			d.codingLine[0] = 0
			d.codingPos = 0
			refPos = 0
			blackPixels = 0

			for d.coding(d.codingPos) < columns {
				code1 = d.getTwoDimCode()
				switch code1 {
				case twoDimPass:
					d.addPixels(d.ref(refPos+1), blackPixels)
					if d.ref(refPos+1) < columns {
						refPos += 2
					}
				case twoDimHoriz:
					code1, code2 = 0, 0
					if blackPixels != 0 {
						for {
							code3 = d.getBlackCode()
							code1 += code3
							if code3 < 64 {
								break
							}
						}
						for {
							code3 = d.getWhiteCode()
							code2 += code3
							if code3 < 64 {
								break
							}
						}
					} else {
						for {
							code3 = d.getWhiteCode()
							code1 += code3
							if code3 < 64 {
								break
							}
						}
						for {
							code3 = d.getBlackCode()
							code2 += code3
							if code3 < 64 {
								break
							}
						}
					}
					d.addPixels(d.coding(d.codingPos)+code1, blackPixels)
					if d.coding(d.codingPos) < columns {
						d.addPixels(d.coding(d.codingPos)+code2, blackPixels^1)
					}
					for d.ref(refPos) <= d.coding(d.codingPos) && d.ref(refPos) < columns {
						refPos += 2
					}
				case twoDimVertR3, twoDimVertR2, twoDimVertR1, twoDimVert0:
					delta := 0
					switch code1 {
					case twoDimVertR3:
						delta = 3
					case twoDimVertR2:
						delta = 2
					case twoDimVertR1:
						delta = 1
					}
					d.addPixels(d.ref(refPos)+delta, blackPixels)
					blackPixels ^= 1
					if d.coding(d.codingPos) < columns {
						refPos++
						for d.ref(refPos) <= d.coding(d.codingPos) && d.ref(refPos) < columns {
							refPos += 2
						}
					}
				case twoDimVertL3, twoDimVertL2, twoDimVertL1:
					delta := 1
					switch code1 {
					case twoDimVertL3:
						delta = 3
					case twoDimVertL2:
						delta = 2
					}
					d.addPixelsNeg(d.ref(refPos)-delta, blackPixels)
					blackPixels ^= 1
					if d.coding(d.codingPos) < columns {
						if refPos > 0 {
							refPos--
						} else {
							refPos++
						}
						for d.ref(refPos) <= d.coding(d.codingPos) && d.ref(refPos) < columns {
							refPos += 2
						}
					}
				case ccittEOF:
					d.addPixels(columns, 0)
					d.eof = true
				default:
					// bad 2d code
					d.addPixels(columns, 0)
					d.err = true
				}
			}
		} else {
			d.codingLine[0] = 0
			d.codingPos = 0
			blackPixels = 0
			for d.coding(d.codingPos) < columns {
				code1 = 0
				if blackPixels != 0 {
					for {
						code3 = d.getBlackCode()
						code1 += code3
						if code3 < 64 {
							break
						}
					}
				} else {
					for {
						code3 = d.getWhiteCode()
						code1 += code3
						if code3 < 64 {
							break
						}
					}
				}
				d.addPixels(d.coding(d.codingPos)+code1, blackPixels)
				blackPixels ^= 1
			}
		}

		gotEOL := false

		if d.byteAlign {
			d.inputBits &^= 7
		}

		if !d.eoblock && d.row == d.rows-1 {
			d.rowsDone = true
		} else {
			code1 = d.lookBits(12)
			if d.eoline {
				for code1 != ccittEOF && code1 != 1 {
					d.eatBits(1)
					code1 = d.lookBits(12)
				}
			} else {
				for code1 == 0 {
					d.eatBits(1)
					code1 = d.lookBits(12)
				}
			}
			if code1 == 1 {
				d.eatBits(12)
				gotEOL = true
			} else if code1 == ccittEOF {
				d.eof = true
			}
		}

		if !d.eof && d.encoding > 0 && !d.rowsDone {
			d.nextLine2D = d.lookBits(1) == 0
			d.eatBits(1)
		}

		if d.eoblock && gotEOL && d.byteAlign {
			code1 = d.lookBits(12)
			if code1 == 1 {
				d.eatBits(12)
				if d.encoding > 0 {
					d.lookBits(1)
					d.eatBits(1)
				}
				if d.encoding >= 0 {
					for i = 0; i < 4; i++ {
						d.lookBits(12) // "bad rtc code" is only a warning
						d.eatBits(12)
						if d.encoding > 0 {
							d.lookBits(1)
							d.eatBits(1)
						}
					}
				}
				d.eof = true
			}
		} else if d.err && d.eoline {
			for {
				code1 = d.lookBits(13)
				if code1 == ccittEOF {
					d.eof = true
					return -1
				}
				if code1>>1 == 1 {
					break
				}
				d.eatBits(1)
			}
			d.eatBits(12)
			if d.encoding > 0 {
				d.eatBits(1)
				d.nextLine2D = code1&1 == 0
			}
		}

		if d.codingLine[0] > 0 {
			d.codingPos = 0
		} else {
			d.codingPos = 1
		}
		d.outputBits = d.coding(d.codingPos)
		d.row++
	}

	var c int
	if d.outputBits >= 8 {
		if d.codingPos&1 != 0 {
			c = 0
		} else {
			c = 0xff
		}
		d.outputBits -= 8
		if d.outputBits == 0 && d.coding(d.codingPos) < columns {
			d.codingPos++
			d.outputBits = d.coding(d.codingPos) - d.coding(d.codingPos-1)
		}
	} else {
		bits := 8
		c = 0
		for bits != 0 {
			if d.outputBits > bits {
				c <<= bits
				if d.codingPos&1 == 0 {
					c |= 0xff >> (8 - bits)
				}
				d.outputBits -= bits
				bits = 0
			} else {
				c <<= d.outputBits
				if d.codingPos&1 == 0 {
					c |= 0xff >> (8 - d.outputBits)
				}
				bits -= d.outputBits
				d.outputBits = 0
				if d.coding(d.codingPos) < columns {
					d.codingPos++
					d.outputBits = d.coding(d.codingPos) - d.coding(d.codingPos-1)
				} else if bits > 0 {
					c <<= bits
					bits = 0
				}
			}
			if d.outputBits < 0 {
				fail("invalid MMR data")
			}
		}
	}
	if d.black {
		c ^= 0xff
	}
	return c & 0xff
}

func (d *ccittDecoder) addPixels(a1, blackPixels int) {
	codingPos := d.codingPos
	if a1 > d.coding(codingPos) {
		if a1 > d.columns {
			d.err = true // row is wrong length
			a1 = d.columns
		}
		if (codingPos&1)^blackPixels != 0 {
			codingPos++
		}
		d.setCoding(codingPos, a1)
	}
	d.codingPos = codingPos
}

func (d *ccittDecoder) addPixelsNeg(a1, blackPixels int) {
	codingPos := d.codingPos
	if a1 > d.coding(codingPos) {
		if a1 > d.columns {
			d.err = true // row is wrong length
			a1 = d.columns
		}
		if (codingPos&1)^blackPixels != 0 {
			codingPos++
		}
		d.setCoding(codingPos, a1)
	} else if a1 < d.coding(codingPos) {
		if a1 < 0 {
			d.err = true // invalid code
			a1 = 0
		}
		for codingPos > 0 && a1 < d.coding(codingPos-1) {
			codingPos--
		}
		d.setCoding(codingPos, a1)
	}
	d.codingPos = codingPos
}

// findTableCode looks for a code of start to end bits in table. It
// returns whether a code was found, the code's value, and whether bits
// were consumed (false at the end of the data).
func (d *ccittDecoder) findTableCode(start, end int, table [][2]int16, limit int) (bool, int, bool) {
	for i := start; i <= end; i++ {
		code := d.lookBits(i)
		if code == ccittEOF {
			return true, 1, false
		}
		if i < end {
			code <<= end - i
		}
		if limit == 0 || code >= limit {
			if idx := code - limit; idx >= 0 && idx < len(table) {
				p := table[idx]
				if int(p[0]) == i {
					d.eatBits(i)
					return true, int(p[1]), true
				}
			}
		}
	}
	return false, 0, false
}

func (d *ccittDecoder) getTwoDimCode() int {
	if d.eoblock {
		code := d.lookBits(7)
		if code >= 0 && code < len(twoDimTable) {
			if p := twoDimTable[code]; p[0] > 0 {
				d.eatBits(int(p[0]))
				return int(p[1])
			}
		}
	} else {
		found, v, ate := d.findTableCode(1, 7, twoDimTable[:], 0)
		if found && ate {
			return v
		}
	}
	// bad two dim code
	return ccittEOF
}

func (d *ccittDecoder) getWhiteCode() int {
	if d.eoblock {
		code := d.lookBits(12)
		if code == ccittEOF {
			return 1
		}
		var p [2]int16
		if code>>5 == 0 {
			p = whiteTable1[code]
		} else {
			p = whiteTable2[code>>3]
		}
		if p[0] > 0 {
			d.eatBits(int(p[0]))
			return int(p[1])
		}
	} else {
		if found, v, _ := d.findTableCode(1, 9, whiteTable2[:], 0); found {
			return v
		}
		if found, v, _ := d.findTableCode(11, 12, whiteTable1[:], 0); found {
			return v
		}
	}
	// bad white code
	d.eatBits(1)
	return 1
}

func (d *ccittDecoder) getBlackCode() int {
	if d.eoblock {
		code := d.lookBits(13)
		if code == ccittEOF {
			return 1
		}
		var p [2]int16
		if code>>7 == 0 {
			p = blackTable1[code]
		} else if code>>9 == 0 && code>>7 != 0 {
			p = blackTable2[(code>>1)-64]
		} else {
			p = blackTable3[code>>7]
		}
		if p[0] > 0 {
			d.eatBits(int(p[0]))
			return int(p[1])
		}
	} else {
		if found, v, _ := d.findTableCode(2, 6, blackTable3[:], 0); found {
			return v
		}
		if found, v, _ := d.findTableCode(7, 12, blackTable2[:], 64); found {
			return v
		}
		if found, v, _ := d.findTableCode(10, 13, blackTable1[:], 0); found {
			return v
		}
	}
	// bad black code
	d.eatBits(1)
	return 1
}

func (d *ccittDecoder) lookBits(n int) int {
	for d.inputBits < n {
		c := d.source.next()
		if c == -1 {
			if d.inputBits == 0 {
				return ccittEOF
			}
			return int((d.inputBuf << uint(n-d.inputBits)) & (0xffff >> uint(16-n)))
		}
		d.inputBuf = d.inputBuf<<8 | uint32(c)
		d.inputBits += 8
	}
	return int((d.inputBuf >> uint(d.inputBits-n)) & (0xffff >> uint(16-n)))
}

func (d *ccittDecoder) eatBits(n int) {
	d.inputBits -= n
	if d.inputBits < 0 {
		d.inputBits = 0
	}
}
