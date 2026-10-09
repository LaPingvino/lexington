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

// Ported from pdf.js src/core/jbig2.js (parseJbig2Chunks) and
// src/core/jbig2_stream.js.

package jbig2

import (
	"bytes"
	"errors"
	"fmt"
	"image"
)

// Decode decodes a PDF JBIG2Decode image: data is the image stream,
// globals the JBIG2Globals stream (nil if none); width and height are the
// PDF image's. It returns the natural rendering, ink black: a JBIG2 1 bit
// (black) is Gray 0 and a 0 bit is Gray 255.
//
// In PDF terms Gray 0 is image sample 0 and Gray 255 sample 1, which is
// right as it is for DeviceGray with the default Decode array. The caller
// applies a Decode array of [1 0] (swapping the two), and an Indexed
// colour space: then the sample is an index into the palette. Some
// encoders store paper as JBIG2 1 bits with a palette of white, black
// (e.g. [/Indexed /DeviceGray 1 <FF00>]); without the palette such a
// page comes out as white text on black.
//
// The page is decoded at the size given by its page information segment
// and then cropped or padded (with white) to width x height.
func Decode(data, globals []byte, width, height int) (img *image.Gray, err error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("jbig2: invalid image size %dx%d", width, height)
	}
	if width > maxPixels/height {
		return nil, fmt.Errorf("jbig2: image too large: %dx%d", width, height)
	}
	p, err := decodePage(data, globals, width, height)
	if err != nil {
		return nil, err
	}
	img = image.NewGray(image.Rect(0, 0, width, height))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := 0; y < height && y < p.height; y++ {
		src := p.buf[y*p.width : (y+1)*p.width]
		dst := img.Pix[y*img.Stride : y*img.Stride+width]
		for x := 0; x < width && x < p.width; x++ {
			if src[x] != 0 {
				dst[x] = 0
			}
		}
	}
	return img, nil
}

// decodePage decodes the segments of globals and then data onto one page.
func decodePage(data, globals []byte, width, height int) (p *page, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*jbig2Error); ok {
				err = e
			} else {
				err = fmt.Errorf("jbig2: invalid data: %v", r)
			}
			p = nil
		}
	}()
	p = &page{pdfWidth: width, pdfHeight: height}
	for _, chunk := range [][]byte{globals, data} {
		for _, s := range readSegments(false, chunk, 0, len(chunk)) {
			p.processSegment(s)
		}
	}
	if p.info == nil {
		return nil, errors.New("jbig2: no page information segment")
	}
	return p, nil
}

// fileHeader is the JBIG2 file header id string (D.4.1).
var fileHeader = []byte{0x97, 0x4a, 0x42, 0x32, 0x0d, 0x0a, 0x1a, 0x0a}

// decodeFile decodes page pageNumber (from 1) of a standalone JBIG2 file,
// as pdf.js's parseJbig2 does (which however draws all pages onto one
// buffer). It is used for testing with standalone test files.
func decodeFile(data []byte, pageNumber uint32) (p *page, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*jbig2Error); ok {
				err = e
			} else {
				err = fmt.Errorf("jbig2: invalid data: %v", r)
			}
			p = nil
		}
	}()
	if len(data) < 9 || !bytes.Equal(data[:8], fileHeader) {
		return nil, errors.New("jbig2: invalid header")
	}
	position := 8
	flags := data[position]
	position++
	randomAccess := flags&1 == 0
	if flags&2 == 0 {
		position += 4 // number of pages
	}
	p = &page{}
	for _, s := range readSegments(randomAccess, data, position, len(data)) {
		if pa := s.header.pageAssociation; pa != 0 && pa != pageNumber {
			continue
		}
		p.processSegment(s)
	}
	if p.info == nil {
		return nil, errors.New("jbig2: no page information segment")
	}
	return p, nil
}
