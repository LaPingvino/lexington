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

// Ported from pdf.js src/core/arithmetic_decoder.js and the arithmetic
// integer decoding parts of src/core/jbig2.js.

package jbig2

import "math"

// qe is one row of Table C-2 (JPEG 2000 Part I, Annex C).
type qe struct {
	qe         uint32
	nmps, nlps uint8
	switchFlag uint8
}

var qeTable = [...]qe{
	{0x5601, 1, 1, 1},
	{0x3401, 2, 6, 0},
	{0x1801, 3, 9, 0},
	{0x0ac1, 4, 12, 0},
	{0x0521, 5, 29, 0},
	{0x0221, 38, 33, 0},
	{0x5601, 7, 6, 1},
	{0x5401, 8, 14, 0},
	{0x4801, 9, 14, 0},
	{0x3801, 10, 14, 0},
	{0x3001, 11, 17, 0},
	{0x2401, 12, 18, 0},
	{0x1c01, 13, 20, 0},
	{0x1601, 29, 21, 0},
	{0x5601, 15, 14, 1},
	{0x5401, 16, 14, 0},
	{0x5101, 17, 15, 0},
	{0x4801, 18, 16, 0},
	{0x3801, 19, 17, 0},
	{0x3401, 20, 18, 0},
	{0x3001, 21, 19, 0},
	{0x2801, 22, 19, 0},
	{0x2401, 23, 20, 0},
	{0x2201, 24, 21, 0},
	{0x1c01, 25, 22, 0},
	{0x1801, 26, 23, 0},
	{0x1601, 27, 24, 0},
	{0x1401, 28, 25, 0},
	{0x1201, 29, 26, 0},
	{0x1101, 30, 27, 0},
	{0x0ac1, 31, 28, 0},
	{0x09c1, 32, 29, 0},
	{0x08a1, 33, 30, 0},
	{0x0521, 34, 31, 0},
	{0x0441, 35, 32, 0},
	{0x02a1, 36, 33, 0},
	{0x0221, 37, 34, 0},
	{0x0141, 38, 35, 0},
	{0x0111, 39, 36, 0},
	{0x0085, 40, 37, 0},
	{0x0049, 41, 38, 0},
	{0x0025, 42, 39, 0},
	{0x0015, 43, 40, 0},
	{0x0009, 44, 41, 0},
	{0x0005, 45, 42, 0},
	{0x0001, 45, 43, 0},
	{0x5601, 46, 46, 0},
}

// arithDecoder implements the MQ decoder of JPEG 2000 Part I, Annex C.3,
// as used by JBIG2 (Annex E).
type arithDecoder struct {
	data        []byte
	bp, dataEnd int
	chigh, clow uint32
	ct          int
	a           uint32
	exhausted   bool // the data is used up: the decoder reads 1 bits
	past        int  // decisions decoded since
}

// maxPast bounds the decisions decoded once the data is used up. Valid data
// needs only a few, though encoders may strip trailing bytes the decoder
// then fills in (some 2M decisions per 8M pixels in the worst conformance
// file); corrupt data (a bad count of symbols, say) could otherwise keep
// the decoder, like pdf.js, busy for minutes.
const maxPast = 1 << 25

// at returns data[i], or 0 beyond the data (JavaScript's undefined used in
// arithmetic).
func (d *arithDecoder) at(i int) uint32 {
	if i >= 0 && i < len(d.data) {
		return uint32(d.data[i])
	}
	return 0
}

// newArithDecoder is INITDEC (C.3.5).
func newArithDecoder(data []byte, start, end int) *arithDecoder {
	d := &arithDecoder{data: data, bp: start, dataEnd: end}
	d.chigh = d.at(start)
	d.clow = 0
	d.byteIn()
	d.chigh = ((d.chigh << 7) & 0xffff) | ((d.clow >> 9) & 0x7f)
	d.clow = (d.clow << 7) & 0xffff
	d.ct -= 7
	d.a = 0x8000
	return d
}

// byteIn is BYTEIN (C.3.4).
func (d *arithDecoder) byteIn() {
	bp := d.bp
	if bp >= 0 && bp < len(d.data) && d.data[bp] == 0xff {
		if d.at(bp+1) > 0x8f {
			d.clow += 0xff00
			d.ct = 8
			d.exhausted = true
		} else {
			bp++
			d.clow += d.at(bp) << 9
			d.ct = 7
			d.bp = bp
		}
	} else {
		bp++
		if bp < d.dataEnd {
			d.clow += d.at(bp) << 8
		} else {
			d.clow += 0xff00
			d.exhausted = true
		}
		d.ct = 8
		d.bp = bp
	}
	if d.clow > 0xffff {
		d.chigh += d.clow >> 16
		d.clow &= 0xffff
	}
}

// readBit is DECODE (C.3.2). A context is packed in one byte: the highest
// 7 bits are the index into qeTable, the lowest bit is the MPS.
func (d *arithDecoder) readBit(contexts []uint8, pos int) int {
	if d.exhausted {
		d.past++
		if d.past > maxPast {
			fail("arithmetic coded data exhausted")
		}
	}
	cxIndex := contexts[pos] >> 1
	cxMps := contexts[pos] & 1
	row := &qeTable[cxIndex]
	qeIcx := row.qe
	var bit uint8
	a := d.a - qeIcx

	if d.chigh < qeIcx {
		// exchangeLps
		if a < qeIcx {
			a = qeIcx
			bit = cxMps
			cxIndex = row.nmps
		} else {
			a = qeIcx
			bit = 1 ^ cxMps
			if row.switchFlag == 1 {
				cxMps = bit
			}
			cxIndex = row.nlps
		}
	} else {
		d.chigh -= qeIcx
		if a&0x8000 != 0 {
			d.a = a
			return int(cxMps)
		}
		// exchangeMps
		if a < qeIcx {
			bit = 1 ^ cxMps
			if row.switchFlag == 1 {
				cxMps = bit
			}
			cxIndex = row.nlps
		} else {
			bit = cxMps
			cxIndex = row.nmps
		}
	}
	// C.3.3 renormD
	for {
		if d.ct == 0 {
			d.byteIn()
		}
		a <<= 1
		d.chigh = ((d.chigh << 1) & 0xffff) | ((d.clow >> 15) & 1)
		d.clow = (d.clow << 1) & 0xffff
		d.ct--
		if a&0x8000 != 0 {
			break
		}
	}
	d.a = a
	contexts[pos] = cxIndex<<1 | cxMps
	return int(bit)
}

// contextCache holds the arithmetic coding contexts of one segment.
type contextCache struct {
	gb, gr []uint8
	named  map[string][]uint8
}

func (c *contextCache) get(id string) []uint8 {
	if c.named == nil {
		c.named = map[string][]uint8{}
	}
	cx, ok := c.named[id]
	if !ok {
		cx = make([]uint8, 1<<16)
		c.named[id] = cx
	}
	return cx
}

func (c *contextCache) genericContexts() []uint8 {
	if c.gb == nil {
		c.gb = make([]uint8, 1<<16)
	}
	return c.gb
}

func (c *contextCache) refinementContexts() []uint8 {
	if c.gr == nil {
		c.gr = make([]uint8, 1<<16)
	}
	return c.gr
}

// iaidContexts returns the IAID contexts, large enough for codeLength.
func (c *contextCache) iaidContexts(codeLength int) []uint8 {
	if codeLength > 24 {
		fail("symbol code length %d is too large", codeLength)
	}
	cx := c.get("IAID")
	if need := 1 << (codeLength + 1); need > len(cx) {
		cx = append(cx, make([]uint8, need-len(cx))...)
		c.named["IAID"] = cx
	}
	return cx
}

// decodingContext is the arithmetic decoding state of one segment; the
// decoder is created on first use, as in pdf.js.
type decodingContext struct {
	data       []byte
	start, end int
	dec        *arithDecoder
	cache      contextCache
}

func newDecodingContext(data []byte, start, end int) *decodingContext {
	return &decodingContext{data: data, start: start, end: end}
}

func (dc *decodingContext) decoder() *arithDecoder {
	if dc.dec == nil {
		dc.dec = newArithDecoder(dc.data, dc.start, dc.end)
	}
	return dc.dec
}

// decodeInteger is the arithmetic integer decoding procedure (Annex A.2).
// ok is false for OOB, or when the value does not fit in 32 bits (pdf.js
// returns null in both cases).
func decodeInteger(cache *contextCache, procedure string, decoder *arithDecoder) (value int, ok bool) {
	contexts := cache.get(procedure)
	prev := 1

	readBits := func(length int) int64 {
		var v uint32
		for i := 0; i < length; i++ {
			bit := decoder.readBit(contexts, prev)
			if prev < 256 {
				prev = prev<<1 | bit
			} else {
				prev = ((prev<<1|bit)&511 | 256)
			}
			v = v<<1 | uint32(bit)
		}
		return int64(v)
	}

	sign := readBits(1)
	var v int64
	if readBits(1) == 0 {
		v = readBits(2)
	} else if readBits(1) == 0 {
		v = readBits(4) + 4
	} else if readBits(1) == 0 {
		v = readBits(6) + 20
	} else if readBits(1) == 0 {
		v = readBits(8) + 84
	} else if readBits(1) == 0 {
		v = readBits(12) + 340
	} else {
		v = readBits(32) + 4436
	}
	if sign != 0 {
		if v == 0 {
			return 0, false // OOB
		}
		v = -v
	}
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, false
	}
	return int(v), true
}

// decodeIntegerOr0 is decodeInteger where OOB counts as 0, which is what
// pdf.js does when it does not check for null.
func decodeIntegerOr0(cache *contextCache, procedure string, decoder *arithDecoder) int {
	v, _ := decodeInteger(cache, procedure, decoder)
	return v
}

// decodeIAID is the IAID decoding procedure (Annex A.3).
func decodeIAID(cache *contextCache, decoder *arithDecoder, codeLength int) int {
	contexts := cache.iaidContexts(codeLength)
	prev := 1
	for i := 0; i < codeLength; i++ {
		bit := decoder.readBit(contexts, prev)
		prev = prev<<1 | bit
	}
	if codeLength < 31 {
		return prev & (1<<codeLength - 1)
	}
	return prev & 0x7fffffff
}
